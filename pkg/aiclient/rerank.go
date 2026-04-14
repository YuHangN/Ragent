package aiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/YuHangN/ragent-go/config"
)

type RerankService interface {
	Rerank(ctx context.Context, query string, candidates []RetrievedChunk, topN int) ([]RetrievedChunk, error)
}

type noopRerankService struct{}

func (noopRerankService) Rerank(_ context.Context, _ string, candidates []RetrievedChunk, topN int) ([]RetrievedChunk, error) {
	if topN > 0 && topN < len(candidates) {
		return candidates[:topN], nil
	}

	return candidates, nil
}

// httpRerankService calls any Cohere-compatible rerank endpoint.
type httpRerankService struct {
	apiURL string
	apiKey string
	model  string
	client *http.Client
}

// NewRerankService returns a noopRerankService if cfg.Rerank.Candidates is empty,
// otherwise returns an httpRerankService backed by the resolved provider.
func NewRerankService(cfg *config.AIConfig) (RerankService, error) {
	if len(cfg.Rerank.Candidates) == 0 {
		return noopRerankService{}, nil
	}
	candidate, provider, err := resolveDefault(
		cfg.Rerank.DefaultModel,
		cfg.Rerank.Candidates,
		cfg.Providers,
	)
	if err != nil {
		return nil, fmt.Errorf("rerank: %w", err)
	}
	apiURL := resolveEndpoint(provider, "rerank", "/v1/rerank")
	return &httpRerankService{
		apiURL: apiURL,
		apiKey: provider.APIKey,
		model:  candidate.Model,
		client: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// ── Cohere-compatible request / response ──────────────────────────
type cohereRerankReq struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n"`
}

type cohereRerankResp struct {
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float32 `json:"relevance_score"`
	} `json:"results"`
	// Standard error envelope (same as embedding/chat).
	Error *apiError `json:"error,omitempty"`
}

func (s *httpRerankService) Rerank(ctx context.Context, query string, candidates []RetrievedChunk, topN int) ([]RetrievedChunk, error) {
	if len(candidates) == 0 {
		return nil, nil
	}

	// Deduplicate by ID
	seen := make(map[string]bool, len(candidates))
	dedup := make([]RetrievedChunk, 0, len(candidates))
	for _, c := range candidates {
		if !seen[c.ID] {
			seen[c.ID] = true
			dedup = append(dedup, c)
		}
	}

	// Skip HTTP call if topN is already satisfied by dedup size.
	if topN <= 0 || len(dedup) <= topN {
		return dedup, nil
	}

	docs := make([]string, len(dedup))
	for i, c := range dedup {
		docs[i] = c.Text
	}

	data, _ := json.Marshal(cohereRerankReq{
		Model:     s.model,
		Query:     query,
		Documents: docs,
		TopN:      topN,
	})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("rerank build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("rerank HTTP: %w", err)
	}
	defer resp.Body.Close()

	respData, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rerank HTTP %d: %s", resp.StatusCode, respData)
	}

	var result cohereRerankResp
	if err := json.Unmarshal(respData, &result); err != nil {
		return nil, fmt.Errorf("rerank parse: %w", err)
	}
	if result.Error != nil {
		return nil, fmt.Errorf("rerank API error %s: %s", result.Error.Code, result.Error.Message)
	}

	reranked := make([]RetrievedChunk, 0, topN)
	for _, r := range result.Results {
		if r.Index >= 0 && r.Index < len(dedup) {
			c := dedup[r.Index]
			c.Score = r.RelevanceScore
			reranked = append(reranked, c)
		}
		if len(reranked) >= topN {
			break
		}
	}

	return reranked, nil
}
