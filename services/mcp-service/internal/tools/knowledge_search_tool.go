// Package tools provides tool implementations
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// KnowledgeSearchTool implements knowledge base search via Knowledge Service
type KnowledgeSearchTool struct {
	knowledgeServiceAddr string
	httpClient           *http.Client
}

// NewKnowledgeSearchTool creates a new knowledge search tool
func NewKnowledgeSearchTool(knowledgeServiceAddr string) *KnowledgeSearchTool {
	return &KnowledgeSearchTool{
		knowledgeServiceAddr: knowledgeServiceAddr,
		httpClient:           &http.Client{Timeout: 30 * time.Second},
	}
}

// GetInfo returns tool information for LLM
func (t *KnowledgeSearchTool) GetInfo() ToolInfo {
	return ToolInfo{
		Name:        "knowledge_search",
		Description: "Search the knowledge base for information. Use this to find relevant documents and information from internal sources.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The search query",
				},
				"top_k": map[string]interface{}{
					"type":        "integer",
					"description": "Number of results to return (default: 5)",
				},
				"search_type": map[string]interface{}{
					"type":        "string",
					"description": "Search type: 'vector', 'bm25', or 'hybrid' (default: hybrid)",
				},
			},
			"required": []string{"query"},
		},
	}
}

// Execute executes the knowledge search tool
func (t *KnowledgeSearchTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	return t.ExecuteWithConfig(ctx, args, nil)
}

// ExecuteWithConfig executes the knowledge search tool with config
func (t *KnowledgeSearchTool) ExecuteWithConfig(ctx context.Context, args map[string]interface{}, config map[string]interface{}) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	topK := 5
	if k, ok := args["top_k"].(float64); ok {
		topK = int(k)
	}

	searchType := "hybrid"
	if st, ok := args["search_type"].(string); ok {
		searchType = st
	}

	// Build search request
	searchReq := map[string]interface{}{
		"query":       query,
		"top_k":       topK,
		"search_type": searchType,
	}

	reqBody, err := json.Marshal(searchReq)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	// Use HTTP client to call Knowledge Service via Gateway
	url := fmt.Sprintf("%s/api/v2/knowledge/search", t.knowledgeServiceAddr)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("search failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var searchResp struct {
		Results []struct {
			ChunkID    string  `json:"chunk_id"`
			DocumentID string  `json:"document_id"`
			Content    string  `json:"content"`
			Score      float64 `json:"score"`
		} `json:"results"`
		Total int64 `json:"total"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	// Format results
	if len(searchResp.Results) == 0 {
		return fmt.Sprintf("No results found for query: %q", query), nil
	}

	result := fmt.Sprintf("Found %d results for %q:\n\n", len(searchResp.Results), query)
	for i, r := range searchResp.Results {
		result += fmt.Sprintf("%d. [Score: %.3f] %s\n", i+1, r.Score, truncate(r.Content, 200))
		if i < len(searchResp.Results)-1 {
			result += "\n"
		}
	}

	return result, nil
}

// Close closes the HTTP client
func (t *KnowledgeSearchTool) Close() error {
	t.httpClient.CloseIdleConnections()
	return nil
}