package aiclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCapabilityConstants(t *testing.T) {
	assert.Equal(t, Capability("chat"), CapabilityChat)
	assert.Equal(t, Capability("embedding"), CapabilityEmbedding)
	assert.Equal(t, Capability("rerank"), CapabilityRerank)
}

func TestCapability_DisplayName(t *testing.T) {
	assert.Equal(t, "Chat", CapabilityChat.DisplayName())
	assert.Equal(t, "Embedding", CapabilityEmbedding.DisplayName())
	assert.Equal(t, "Rerank", CapabilityRerank.DisplayName())
}
