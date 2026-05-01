package aiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type CohereRerankClient struct {
	provider Provider
	client   *http.Client
}

func NewCohereRerankClient() *CohereRerankClient {
	return &CohereRerankClient{
		provider: ProviderCohere,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *CohereRerankClient) WithProvider(p Provider) *CohereRerankClient {
	c.provider = p
	return c
}

func (c *CohereRerankClient) Provider() Provider { return c.provider }

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
	Error *apiError `json:"error,omitempty"`
}

func (c *CohereRerankClient) Rerank(ctx context.Context, query string, candidates []RetrievedChunk, topN int, target *ModelTarget) ([]RetrievedChunk, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	// Dedup by ID（对齐 Java 实现）
	seen := make(map[string]bool, len(candidates))
	dedup := make([]RetrievedChunk, 0, len(candidates))
	for _, x := range candidates {
		if !seen[x.ID] {
			seen[x.ID] = true
			dedup = append(dedup, x)
		}
	}
	if topN <= 0 || len(dedup) <= topN {
		return dedup, nil
	}

	url, err := ResolveURL(target.Provider, target.Candidate, CapabilityRerank)
	if err != nil {
		return nil, &ClientError{Type: ErrProviderError, Message: err.Error()}
	}
	docs := make([]string, len(dedup))
	for i, x := range dedup {
		docs[i] = x.Text
	}
	body, _ := json.Marshal(cohereRerankReq{
		Model: target.Candidate.Model, Query: query, Documents: docs, TopN: topN,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, &ClientError{Type: ErrNetworkError, Message: err.Error(), Cause: err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+target.Provider.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, &ClientError{Type: ErrNetworkError, Message: err.Error(), Cause: err}
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, NewHTTPError(resp.StatusCode, string(data))
	}

	var result cohereRerankResp
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, &ClientError{Type: ErrInvalidResponse, Message: err.Error(), Cause: err}
	}
	if result.Error != nil {
		return nil, &ClientError{Type: ErrProviderError, Message: fmt.Sprintf("%s: %s", result.Error.Code, result.Error.Message)}
	}

	reranked := make([]RetrievedChunk, 0, topN)
	for _, r := range result.Results {
		if r.Index >= 0 && r.Index < len(dedup) {
			x := dedup[r.Index]
			x.Score = r.RelevanceScore
			reranked = append(reranked, x)
		}
		if len(reranked) >= topN {
			break
		}
	}

	return reranked, nil
}
