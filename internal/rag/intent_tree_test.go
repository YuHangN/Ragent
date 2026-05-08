package rag

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildTree_TwoLevels(t *testing.T) {
	id1, id2 := int64(2), int64(2)
	nodes := []IntentNode{
		{ID: 1, KbID: 100, Name: "Root A", Level: 1, Kind: IntentKindKB, Enabled: 1},
		{ID: 2, KbID: 100, Name: "Branch A1", Level: 2, Kind: IntentKindKB, ParentID: ptrInt64(1), Enabled: 1},
		{ID: 3, KbID: 100, Name: "Leaf A1.1", Level: 3, Kind: IntentKindKB, ParentID: &id1, Enabled: 1},
		{ID: 4, KbID: 100, Name: "Leaf A1.2", Level: 3, Kind: IntentKindKB, ParentID: &id2, Enabled: 1},
	}
	tree := BuildTree(nodes)
	require.Len(t, tree, 1)
	assert.Equal(t, "Root A", tree[0].Name)
	require.Len(t, tree[0].Children, 1)
	assert.Equal(t, "Branch A1", tree[0].Children[0].Name)
	assert.Len(t, tree[0].Children[0].Children, 2)
}

func TestBuildTree_OrphanBecomesRoot(t *testing.T) {
	missing := int64(999)
	nodes := []IntentNode{
		{ID: 5, KbID: 100, Name: "Orphan", Level: 2, Kind: IntentKindKB, ParentID: &missing, Enabled: 1},
	}
	tree := BuildTree(nodes)
	require.Len(t, tree, 1)
	assert.Equal(t, "Orphan", tree[0].Name)
}

func TestBuildTree_Empty(t *testing.T) {
	assert.Nil(t, BuildTree(nil))
}

func ptrInt64(v int64) *int64 { return &v }
