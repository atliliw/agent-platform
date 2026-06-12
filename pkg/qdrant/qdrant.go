// Package qdrant provides Qdrant vector database client
package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Config holds Qdrant client configuration
type Config struct {
	URL        string
	Collection string
	APIKey     string
}

// Client is the Qdrant client
type Client struct {
	config     Config
	httpClient *http.Client
}

// Point represents a Qdrant point
type Point struct {
	ID      string                 `json:"id"`
	Vector  []float64              `json:"vector"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// NamedVectorPoint represents a Qdrant point with named vector
type NamedVectorPoint struct {
	ID      string                 `json:"id"`
	Vector  NamedVector            `json:"vector"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// NamedVector represents a named vector for Qdrant 1.18+
type NamedVector struct {
	Name   string    `json:"name"`
	Vector []float64 `json:"vector"`
}

// SearchResult represents a search result
type SearchResult struct {
	ID      string                 `json:"id"`
	Score   float64                `json:"score"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// SearchRequest represents a search request
type SearchRequest struct {
	Vector      []float64              `json:"vector"`
	Limit       int                    `json:"limit"`
	ScoreThreshold float64             `json:"score_threshold,omitempty"`
	Filter      map[string]interface{} `json:"filter,omitempty"`
	WithPayload bool                   `json:"with_payload"`
}

// NewClient creates a new Qdrant client
func NewClient(cfg Config) *Client {
	return &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Upsert inserts or updates points
func (c *Client) Upsert(ctx context.Context, points []Point) error {
	// For anonymous vectors, use simple vector format
	req := map[string]interface{}{
		"points": points,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points", c.config.URL, c.config.Collection)
	httpReq, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		httpReq.Header.Set("api-key", c.config.APIKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Qdrant error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// Search performs vector search
func (c *Client) Search(ctx context.Context, req *SearchRequest) ([]SearchResult, error) {
	// Qdrant 1.18+ requires named vector format even for anonymous vectors
	body := map[string]interface{}{
		"vector": map[string]interface{}{
			"name":   "", // empty string for anonymous vector
			"vector": req.Vector,
		},
		"limit":        req.Limit,
		"with_payload": req.WithPayload,
	}

	if req.ScoreThreshold > 0 {
		body["score_threshold"] = req.ScoreThreshold
	}

	if req.Filter != nil {
		body["filter"] = req.Filter
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/search", c.config.URL, c.config.Collection)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		httpReq.Header.Set("api-key", c.config.APIKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Qdrant error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var searchResp struct {
		Result []struct {
			ID      interface{}            `json:"id"`
			Score   float64                `json:"score"`
			Payload map[string]interface{} `json:"payload,omitempty"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	results := make([]SearchResult, len(searchResp.Result))
	for i, hit := range searchResp.Result {
		id := ""
		switch v := hit.ID.(type) {
		case string:
			id = v
		case float64:
			id = fmt.Sprintf("%.0f", v)
		default:
			id = fmt.Sprintf("%v", v)
		}
		results[i] = SearchResult{
			ID:      id,
			Score:   hit.Score,
			Payload: hit.Payload,
		}
	}

	return results, nil
}

// Delete deletes points by ID
func (c *Client) Delete(ctx context.Context, ids []string) error {
	req := map[string]interface{}{
		"ids": ids,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/delete", c.config.URL, c.config.Collection)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		httpReq.Header.Set("api-key", c.config.APIKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Qdrant error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// DeleteByFilter deletes points by filter
func (c *Client) DeleteByFilter(ctx context.Context, filter map[string]interface{}) error {
	req := map[string]interface{}{
		"filter": filter,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/delete", c.config.URL, c.config.Collection)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		httpReq.Header.Set("api-key", c.config.APIKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Qdrant error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// CreateCollection creates a collection with anonymous vector (for simpler API)
func (c *Client) CreateCollection(ctx context.Context, vectorSize int) error {
	// Use anonymous vector configuration (no name)
	// This allows simpler API for upsert and search
	req := map[string]interface{}{
		"vectors": map[string]interface{}{
			"size":     vectorSize,
			"distance": "Cosine",
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s", c.config.URL, c.config.Collection)
	httpReq, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		httpReq.Header.Set("api-key", c.config.APIKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 409 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Qdrant error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// CollectionExists checks if collection exists
func (c *Client) CollectionExists(ctx context.Context) (bool, error) {
	url := fmt.Sprintf("%s/collections/%s", c.config.URL, c.config.Collection)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("create request: %w", err)
	}

	if c.config.APIKey != "" {
		httpReq.Header.Set("api-key", c.config.APIKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return false, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200, nil
}