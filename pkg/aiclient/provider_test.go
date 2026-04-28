package aiclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProviderConstants(t *testing.T) {
	assert.Equal(t, Provider("openai"), ProviderOpenAI)
	assert.Equal(t, Provider("ollama"), ProviderOllama)
	assert.Equal(t, Provider("cohere"), ProviderCohere)
	assert.Equal(t, Provider("noop"), ProviderNoop)
}

func TestProvider_Matches_CaseInsensitive(t *testing.T) {
	assert.True(t, ProviderOpenAI.Matches("openai"))
	assert.True(t, ProviderOpenAI.Matches("OpenAI"))
	assert.True(t, ProviderOpenAI.Matches("OPENAI"))
	assert.False(t, ProviderOpenAI.Matches("ollama"))
	assert.False(t, ProviderOpenAI.Matches(""))
}
