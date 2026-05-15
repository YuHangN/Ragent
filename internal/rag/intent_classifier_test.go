package rag

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubIntentRepo struct{ classifiable []IntentNode }

func (r *stubIntentRepo) Create(_ *IntentNode) error               { return nil }
func (r *stubIntentRepo) Update(_ *IntentNode) error               { return nil }
func (r *stubIntentRepo) Delete(_ int64) error                     { return nil }
func (r *stubIntentRepo) FindByID(_ int64) (*IntentNode, error)    { return nil, nil }
func (r *stubIntentRepo) FindByKbID(_ int64) ([]IntentNode, error) { return nil, nil }
func (r *stubIntentRepo) FindClassifiableByKbID(_ int64) ([]IntentNode, error) {
	return r.classifiable, nil
}

func TestIntentNode_TableName(t *testing.T) {
	assert.Equal(t, "t_intent_node", IntentNode{}.TableName())
}

func TestIntentKind_Constants(t *testing.T) {
	assert.Equal(t, IntentKind("KB"), IntentKindKB)
	assert.Equal(t, IntentKind("SYSTEM"), IntentKindSystem)
	assert.Equal(t, IntentKind("MCP"), IntentKindMCP)
}

func TestClassify_FiltersByMinScore(t *testing.T) {
	repo := &stubIntentRepo{classifiable: []IntentNode{
		{ID: 1, KbID: 100, Name: "节点A", Kind: IntentKindKB, PartitionName: "install"},
		{ID: 2, KbID: 100, Name: "节点B", Kind: IntentKindKB, PartitionName: "refund"},
	}}
	llm := &stubLLM{resp: `[{"node_id":1,"score":0.9},{"node_id":2,"score":0.3}]`}
	cls := NewIntentClassifier(llm, repo)

	got, err := cls.Classify(context.Background(), 100, "Q", 5, 0.5)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, int64(1), got[0].NodeID)
	assert.InDelta(t, 0.9, got[0].Score, 0.001)
	assert.Equal(t, "install", got[0].PartitionName)
}

func TestClassify_HandlesMarkdownCodeFence(t *testing.T) {
	repo := &stubIntentRepo{classifiable: []IntentNode{
		{ID: 1, Kind: IntentKindKB, PartitionName: "p1"},
	}}
	llm := &stubLLM{resp: "```json\n[{\"node_id\":1,\"score\":0.8}]\n```"}
	cls := NewIntentClassifier(llm, repo)

	got, err := cls.Classify(context.Background(), 1, "Q", 5, 0.5)
	require.NoError(t, err)
	require.Len(t, got, 1)
}

func TestClassify_NoClassifiableNodes(t *testing.T) {
	repo := &stubIntentRepo{classifiable: nil}
	cls := NewIntentClassifier(&stubLLM{}, repo)

	got, err := cls.Classify(context.Background(), 1, "Q", 5, 0.5)
	require.NoError(t, err)
	assert.Nil(t, got)
}
