package ingestion

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/YuHangN/ragent-go/internal/intent"
	"github.com/YuHangN/ragent-go/pkg/aiclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type routerStubRepo struct{ nodes []intent.Node }

func (r *routerStubRepo) Create(_ *intent.Node) error                            { return nil }
func (r *routerStubRepo) Update(_ *intent.Node) error                            { return nil }
func (r *routerStubRepo) Delete(_ int64) error                                   { return nil }
func (r *routerStubRepo) FindByID(_ int64) (*intent.Node, error)                 { return nil, nil }
func (r *routerStubRepo) FindByKbID(_ int64) ([]intent.Node, error)              { return nil, nil }
func (r *routerStubRepo) FindClassifiableByKbID(_ int64) ([]intent.Node, error)  { return r.nodes, nil }

// routerStubLLM 按调用顺序返回固定的响应序列；err 不为 nil 时所有调用直接返回该 err。
type routerStubLLM struct {
	responses []string
	err       error
	calls     atomic.Int32
}

func (s *routerStubLLM) Chat(_ context.Context, _ aiclient.ChatRequest) (string, error) {
	idx := int(s.calls.Add(1) - 1)
	if s.err != nil {
		return "", s.err
	}
	if idx >= len(s.responses) {
		return "[]", nil
	}
	return s.responses[idx], nil
}
func (s *routerStubLLM) StreamChat(_ context.Context, _ aiclient.ChatRequest, _ aiclient.StreamCallback) error {
	return nil
}

func newRouterClassifier(nodes []intent.Node, responses []string, err error) (*intent.Classifier, *routerStubLLM) {
	llm := &routerStubLLM{responses: responses, err: err}
	return intent.NewClassifier(llm, &routerStubRepo{nodes: nodes}), llm
}

func TestChunkRouter_BatchAssignsPartitions(t *testing.T) {
	nodes := []intent.Node{
		{ID: 1, KbID: 100, Kind: intent.KindKB, Name: "退款政策", PartitionName: "refund"},
		{ID: 2, KbID: 100, Kind: intent.KindKB, Name: "产品安装", PartitionName: "install"},
	}
	// BatchSize=2 → 2 个 chunk 一次 LLM 调用
	cls, llm := newRouterClassifier(nodes, []string{
		`[[{"node_id":1,"score":0.9}],[{"node_id":2,"score":0.85}]]`,
	}, nil)
	node := NewChunkRouterNode(cls, 100, "_fallback", ChunkRouterParams{
		MinScore: 0.5, Concurrency: 1, BatchSize: 2, MaxRetries: 0,
	})

	ic := &IngestionContext{
		Chunks: []VectorChunk{
			{Index: 0, Content: "退款流程"},
			{Index: 1, Content: "安装步骤"},
		},
		PartitionName: "_fallback",
	}
	res := node.Execute(context.Background(), ic)
	require.True(t, res.Success, "msg=%s err=%v", res.Message, res.Err)
	assert.Equal(t, "refund", ic.Chunks[0].TargetPartition)
	assert.Equal(t, "install", ic.Chunks[1].TargetPartition)
	assert.Equal(t, int32(1), llm.calls.Load(), "BatchSize=2 应只调一次 LLM")
}

func TestChunkRouter_WritesMetadata(t *testing.T) {
	nodes := []intent.Node{
		{ID: 1, KbID: 100, Kind: intent.KindKB, Name: "退款政策", PartitionName: "refund"},
	}
	cls, _ := newRouterClassifier(nodes, []string{
		`[[{"node_id":1,"score":0.92}]]`,
	}, nil)
	node := NewChunkRouterNode(cls, 100, "_fallback", ChunkRouterParams{
		MinScore: 0.5, Concurrency: 1, BatchSize: 1, MaxRetries: 0,
	})

	ic := &IngestionContext{
		Chunks:        []VectorChunk{{Index: 0, Content: "退款流程"}},
		PartitionName: "_fallback",
	}
	res := node.Execute(context.Background(), ic)
	require.True(t, res.Success)

	meta := ic.Chunks[0].Metadata
	require.NotNil(t, meta, "metadata 应被初始化")
	routing, ok := meta["routing"].(map[string]any)
	require.True(t, ok, "metadata.routing 应是 map")
	assert.Equal(t, int64(1), routing["node_id"])
	assert.Equal(t, "退款政策", routing["node_name"])
	assert.Equal(t, "refund", routing["partition"])
	assert.InDelta(t, 0.92, routing["score"], 0.0001)
}

func TestChunkRouter_LowScoreFallback(t *testing.T) {
	nodes := []intent.Node{
		{ID: 1, KbID: 100, Kind: intent.KindKB, Name: "退款政策", PartitionName: "refund"},
	}
	cls, _ := newRouterClassifier(nodes, []string{
		`[[{"node_id":1,"score":0.3}]]`,
	}, nil)
	node := NewChunkRouterNode(cls, 100, "_fallback", ChunkRouterParams{
		MinScore: 0.5, Concurrency: 1, BatchSize: 1, MaxRetries: 0,
	})

	ic := &IngestionContext{
		Chunks:        []VectorChunk{{Index: 0, Content: "模糊内容"}},
		PartitionName: "_fallback",
	}
	res := node.Execute(context.Background(), ic)
	require.True(t, res.Success)
	assert.Equal(t, "_fallback", ic.Chunks[0].TargetPartition)
}

func TestChunkRouter_LLMErrorRetriesThenFallback(t *testing.T) {
	nodes := []intent.Node{
		{ID: 1, KbID: 100, Kind: intent.KindKB, PartitionName: "refund"},
	}
	cls, llm := newRouterClassifier(nodes, nil, errors.New("llm down"))
	node := NewChunkRouterNode(cls, 100, "_fallback", ChunkRouterParams{
		MinScore: 0.5, Concurrency: 1, BatchSize: 1, MaxRetries: 2,
	})

	ic := &IngestionContext{
		Chunks:        []VectorChunk{{Index: 0, Content: "x"}},
		PartitionName: "_fallback",
	}
	res := node.Execute(context.Background(), ic)
	require.True(t, res.Success, "LLM 失败不应 fail 整 node")
	assert.Equal(t, "_fallback", ic.Chunks[0].TargetPartition)
	assert.Equal(t, int32(3), llm.calls.Load(), "MaxRetries=2 表示首次 + 2 次重试 = 3 次")
}

func TestChunkRouter_PromptIncludesAllChunksInBatch(t *testing.T) {
	nodes := []intent.Node{
		{ID: 1, KbID: 100, Kind: intent.KindKB, PartitionName: "refund"},
	}
	// 通过 LLM 收到的 prompt 间接验证 batch 拼装：用一个能截获 prompt 的 stub。
	captured := ""
	captureLLM := &captureLLMStub{
		onChat: func(req aiclient.ChatRequest) {
			if len(req.Messages) > 0 {
				captured = req.Messages[0].Content
			}
		},
		resp: `[[{"node_id":1,"score":0.9}],[{"node_id":1,"score":0.9}]]`,
	}
	cls := intent.NewClassifier(captureLLM, &routerStubRepo{nodes: nodes})
	node := NewChunkRouterNode(cls, 100, "_fallback", ChunkRouterParams{
		MinScore: 0.5, Concurrency: 1, BatchSize: 2, MaxRetries: 0,
	})

	ic := &IngestionContext{
		Chunks: []VectorChunk{
			{Index: 0, Content: "AAA-unique-marker"},
			{Index: 1, Content: "BBB-unique-marker"},
		},
		PartitionName: "_fallback",
	}
	_ = node.Execute(context.Background(), ic)
	assert.True(t, strings.Contains(captured, "AAA-unique-marker"), "prompt 应包含第一个 chunk 内容")
	assert.True(t, strings.Contains(captured, "BBB-unique-marker"), "prompt 应包含第二个 chunk 内容")
}

type captureLLMStub struct {
	onChat func(aiclient.ChatRequest)
	resp   string
}

func (s *captureLLMStub) Chat(_ context.Context, req aiclient.ChatRequest) (string, error) {
	if s.onChat != nil {
		s.onChat(req)
	}
	return s.resp, nil
}
func (s *captureLLMStub) StreamChat(_ context.Context, _ aiclient.ChatRequest, _ aiclient.StreamCallback) error {
	return nil
}
