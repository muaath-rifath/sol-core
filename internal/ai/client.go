package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type AnalyzeRequest struct {
	DeviceStates map[string]any `json:"device_states"`
	UserContext   map[string]any `json:"user_context,omitempty"`
	Query        string         `json:"query,omitempty"`
}

type AnalyzeResponse struct {
	Suggestions []Suggestion `json:"suggestions"`
	Insights    []string     `json:"insights,omitempty"`
}

type Suggestion struct {
	Type        string         `json:"type"`
	Description string         `json:"description"`
	Actions     []SuggestedAction `json:"actions"`
	Confidence  float64        `json:"confidence"`
}

type SuggestedAction struct {
	DeviceID string         `json:"device_id"`
	Action   string         `json:"action"`
	Params   map[string]any `json:"params,omitempty"`
}

func (c *Client) Analyze(ctx context.Context, req AnalyzeRequest) (*AnalyzeResponse, error) {
	return doPost[AnalyzeResponse](c, ctx, "/api/v1/analyze", req)
}

func doPost[T any](c *Client, ctx context.Context, path string, body any) (*T, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ai request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ai service returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}
