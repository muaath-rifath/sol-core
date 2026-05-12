package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// CohereClient calls the Cohere embed model via Azure AI Services.
type CohereClient struct {
	endpoint   string
	apiKey     string
	deployment string
	apiVersion string
	http       *http.Client
}

func NewCohereClient(endpoint, apiKey, deployment, apiVersion string) *CohereClient {
	return &CohereClient{
		endpoint:   endpoint,
		apiKey:     apiKey,
		deployment: deployment,
		apiVersion: apiVersion,
		http:       &http.Client{},
	}
}

// Embed returns a 1024-dim vector for text.
// inputType: "search_document" when indexing, "search_query" when querying.
func (c *CohereClient) Embed(ctx context.Context, text, inputType string) ([]float32, error) {
	body, _ := json.Marshal(map[string]any{
		"model":           c.deployment,
		"input":           []string{text},
		"input_type":      inputType,
		"embedding_types": []string{"float"},
	})

	url := fmt.Sprintf("%s/models/embed?api-version=%s", c.endpoint, c.apiVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("chat/cohere: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chat/cohere: request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("chat/cohere: status %d: %s", resp.StatusCode, raw)
	}

	var result struct {
		Data []struct {
			Embedding struct {
				Float []float32 `json:"float"`
			} `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("chat/cohere: decode: %w", err)
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("chat/cohere: empty response")
	}
	return result.Data[0].Embedding.Float, nil
}
