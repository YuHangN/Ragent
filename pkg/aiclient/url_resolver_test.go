package aiclient

import (
	"testing"

	"github.com/YuHangN/ragent-go/config"
	"github.com/stretchr/testify/assert"
)

func TestResolveURL_CandidateOverride(t *testing.T) {
	provider := config.ProviderConfig{URL: "https://api.openai.com"}
	candidate := config.ModelCandidate{URL: "https://custom.example.com/v1/chat"}
	url, err := ResolveURL(provider, candidate, CapabilityChat)
	assert.NoError(t, err)
	assert.Equal(t, "https://custom.example.com/v1/chat", url)
}

func TestResolveURL_FromEndpointMap(t *testing.T) {
	provider := config.ProviderConfig{
		URL: "https://api.openai.com",
		Endpoints: map[string]string{
			"chat":      "/v1/chat/completions",
			"embedding": "/v1/embeddings",
		},
	}

	url, err := ResolveURL(provider, config.ModelCandidate{}, CapabilityChat)
	assert.NoError(t, err)
	assert.Equal(t, "https://api.openai.com/v1/chat/completions", url)
}

func TestResolveURL_TrailingAndLeadingSlashes(t *testing.T) {
	provider := config.ProviderConfig{
		URL:       "https://api.example.com/",
		Endpoints: map[string]string{"chat": "/v1/chat"},
	}

	url, _ := ResolveURL(provider, config.ModelCandidate{}, CapabilityChat)
	assert.Equal(t, "https://api.example.com/v1/chat", url)
}

func TestResolveURL_MissingBaseURL(t *testing.T) {
	provider := config.ProviderConfig{}
	_, err := ResolveURL(provider, config.ModelCandidate{}, CapabilityChat)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "baseUrl")
}

func TestResolveURL_MissingEndpoint(t *testing.T) {
	provider := config.ProviderConfig{URL: "https://x.com", Endpoints: map[string]string{}}
	_, err := ResolveURL(provider, config.ModelCandidate{}, CapabilityChat)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint")
}
