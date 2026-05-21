package intent

import (
	"context"
	"testing"

	"github.com/YuHangN/ragent-go/pkg/aiclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubLLM 测试用 LLMService 桩
type stubLLM struct {
	resp string
	err  error
}

func (s *stubLLM) Chat(_ context.Context, _ aiclient.ChatRequest) (string, error) {
	return s.resp, s.err
}
func (s *stubLLM) StreamChat(_ context.Context, _ aiclient.ChatRequest, _ aiclient.StreamCallback) error {
	return nil
}

type stubRepo struct{ classifiable []Node }

func (r *stubRepo) Create(_ *Node) error               { return nil }
func (r *stubRepo) Update(_ *Node) error               { return nil }
func (r *stubRepo) Delete(_ int64) error                     { return nil }
func (r *stubRepo) FindByID(_ int64) (*Node, error)    { return nil, nil }
func (r *stubRepo) FindByKbID(_ int64) ([]Node, error) { return nil, nil }
func (r *stubRepo) FindClassifiableByKbID(_ int64) ([]Node, error) {
	return r.classifiable, nil
}

func TestNode_TableName(t *testing.T) {
	assert.Equal(t, "t_intent_node", Node{}.TableName())
}

func TestKind_Constants(t *testing.T) {
	assert.Equal(t, Kind("KB"), KindKB)
	assert.Equal(t, Kind("SYSTEM"), KindSystem)
	assert.Equal(t, Kind("MCP"), KindMCP)
}

func TestClassify_FiltersByMinScore(t *testing.T) {
	repo := &stubRepo{classifiable: []Node{
		{ID: 1, KbID: 100, Name: "节点A", Kind: KindKB, PartitionName: "install"},
		{ID: 2, KbID: 100, Name: "节点B", Kind: KindKB, PartitionName: "refund"},
	}}
	llm := &stubLLM{resp: `[{"node_id":1,"score":0.9},{"node_id":2,"score":0.3}]`}
	cls := NewClassifier(llm, repo)

	got, err := cls.ClassifyQuery(context.Background(), 100, "Q", 5, 0.5)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, int64(1), got[0].NodeID)
	assert.InDelta(t, 0.9, got[0].Score, 0.001)
	assert.Equal(t, "install", got[0].PartitionName)
}

func TestClassify_HandlesMarkdownCodeFence(t *testing.T) {
	repo := &stubRepo{classifiable: []Node{
		{ID: 1, Kind: KindKB, PartitionName: "p1"},
	}}
	llm := &stubLLM{resp: "```json\n[{\"node_id\":1,\"score\":0.8}]\n```"}
	cls := NewClassifier(llm, repo)

	got, err := cls.ClassifyQuery(context.Background(), 1, "Q", 5, 0.5)
	require.NoError(t, err)
	require.Len(t, got, 1)
}

func TestClassify_NoClassifiableNodes(t *testing.T) {
	repo := &stubRepo{classifiable: nil}
	cls := NewClassifier(&stubLLM{}, repo)

	got, err := cls.ClassifyQuery(context.Background(), 1, "Q", 5, 0.5)
	require.NoError(t, err)
	assert.Nil(t, got)
}
