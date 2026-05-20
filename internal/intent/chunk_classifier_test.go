package intent

import (
	"context"
	"errors"
	"testing"

	"github.com/YuHangN/ragent-go/pkg/aiclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chunkStubRepo 只用 FindClassifiableByKbID，其它方法返回零值。
type chunkStubRepo struct{ nodes []Node }

func (r *chunkStubRepo) Create(_ *Node) error                            { return nil }
func (r *chunkStubRepo) Update(_ *Node) error                            { return nil }
func (r *chunkStubRepo) Delete(_ int64) error                            { return nil }
func (r *chunkStubRepo) FindByID(_ int64) (*Node, error)                 { return nil, nil }
func (r *chunkStubRepo) FindByKbID(_ int64) ([]Node, error)              { return nil, nil }
func (r *chunkStubRepo) FindClassifiableByKbID(_ int64) ([]Node, error)  { return r.nodes, nil }

type chunkStubLLM struct {
	resp string
	err  error
}

func (s *chunkStubLLM) Chat(_ context.Context, _ aiclient.ChatRequest) (string, error) {
	return s.resp, s.err
}
func (s *chunkStubLLM) StreamChat(_ context.Context, _ aiclient.ChatRequest, _ aiclient.StreamCallback) error {
	return nil
}

func TestClassifyChunks_ReturnsResultsInInputOrder(t *testing.T) {
	nodes := []Node{
		{ID: 1, KbID: 100, Kind: KindKB, Name: "退款政策", PartitionName: "refund"},
		{ID: 2, KbID: 100, Kind: KindKB, Name: "产品安装", PartitionName: "install"},
	}
	// LLM 返回严格按输入顺序的二维数组
	llmResp := `[
		[{"node_id":1,"score":0.9}],
		[{"node_id":2,"score":0.85}],
		[]
	]`
	c := NewClassifier(&chunkStubLLM{resp: llmResp}, &chunkStubRepo{nodes: nodes})

	out, err := c.ClassifyChunks(context.Background(), 100,
		[]string{"退款流程", "安装手册", "无关内容"}, 1, 0.5)
	require.NoError(t, err)
	require.Len(t, out, 3)

	require.Len(t, out[0], 1)
	assert.Equal(t, "refund", out[0][0].PartitionName)
	assert.InDelta(t, 0.9, out[0][0].Score, 0.0001)

	require.Len(t, out[1], 1)
	assert.Equal(t, "install", out[1][0].PartitionName)

	assert.Empty(t, out[2], "第三个 chunk 无候选")
}

func TestClassifyChunks_FiltersBelowMinScore(t *testing.T) {
	nodes := []Node{
		{ID: 1, KbID: 100, Kind: KindKB, PartitionName: "refund"},
	}
	llmResp := `[[{"node_id":1,"score":0.3}]]`
	c := NewClassifier(&chunkStubLLM{resp: llmResp}, &chunkStubRepo{nodes: nodes})

	out, err := c.ClassifyChunks(context.Background(), 100, []string{"x"}, 1, 0.5)
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Empty(t, out[0], "0.3 低于 minScore=0.5 应被过滤")
}

func TestClassifyChunks_LLMError_Propagates(t *testing.T) {
	nodes := []Node{{ID: 1, KbID: 100, Kind: KindKB, PartitionName: "p"}}
	c := NewClassifier(&chunkStubLLM{err: errors.New("llm down")}, &chunkStubRepo{nodes: nodes})

	_, err := c.ClassifyChunks(context.Background(), 100, []string{"x"}, 1, 0.5)
	require.Error(t, err)
}

func TestClassifyChunks_NoClassifiableNodes_AllEmpty(t *testing.T) {
	c := NewClassifier(&chunkStubLLM{}, &chunkStubRepo{nodes: nil})
	out, err := c.ClassifyChunks(context.Background(), 100, []string{"a", "b"}, 1, 0.5)
	require.NoError(t, err)
	require.Len(t, out, 2)
	assert.Empty(t, out[0])
	assert.Empty(t, out[1])
}
