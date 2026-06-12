// Package llm provides LLM client implementations
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DashScopeClient implements Client for Alibaba DashScope API (通义千问)
type DashScopeClient struct {
	config     Config
	httpClient *http.Client
}

// NewDashScopeClient creates a new DashScope client
func NewDashScopeClient(cfg Config) *DashScopeClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 4096
	}
	return &DashScopeClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 180 * time.Second, // 3 分钟，浏览器 Agent 需要更长超时
		},
	}
}

// Chat sends a chat request to DashScope
func (c *DashScopeClient) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	// Build messages
	messages := make([]openAIMessage, 0, len(req.Messages)+1)

	// Add system prompt if provided
	if req.SystemPrompt != "" {
		messages = append(messages, openAIMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	// Add messages
	for _, m := range req.Messages {
		messages = append(messages, openAIMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	// Build request (DashScope uses OpenAI-compatible format)
	dashReq := openAIChatRequest{
		Model:    c.config.Model,
		Messages: messages,
		Stream:   false,
	}

	if req.MaxTokens > 0 {
		dashReq.MaxTokens = req.MaxTokens
	} else {
		dashReq.MaxTokens = c.config.MaxTokens
	}

	if req.Temperature > 0 {
		dashReq.Temperature = req.Temperature
	}

	// Add tools
	if len(req.Tools) > 0 {
		for _, t := range req.Tools {
			dashReq.Tools = append(dashReq.Tools, openAITool{
				Type: t.Type,
				Function: openAIToolFunc{
					Name:        t.Function.Name,
					Description: t.Function.Description,
					Parameters:  t.Function.Parameters,
				},
			})
		}
	}

	// Send request
	body, err := json.Marshal(dashReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		c.config.BaseURL+"/chat/completions",
		bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var dashResp openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&dashResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Build response
	result := &ChatResponse{
		TotalTokens:  dashResp.Usage.TotalTokens,
		PromptTokens: dashResp.Usage.PromptTokens,
	}

	if len(dashResp.Choices) > 0 {
		choice := dashResp.Choices[0]
		result.Content = choice.Message.Content
		result.FinishReason = choice.FinishReason

		// Parse tool calls
		for _, tc := range choice.Message.ToolCalls {
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: ToolCallFunction{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
	}

	// Calculate cost (DashScope pricing)
	result.Cost = c.calculateCost(result.TotalTokens)

	return result, nil
}

// ChatStream sends a streaming chat request to DashScope
func (c *DashScopeClient) ChatStream(ctx context.Context, req *ChatRequest) (<-chan ChatStreamChunk, error) {
	ch := make(chan ChatStreamChunk, 100)

	go func() {
		defer close(ch)

		// Build messages
		messages := make([]openAIMessage, 0, len(req.Messages)+1)

		if req.SystemPrompt != "" {
			messages = append(messages, openAIMessage{
				Role:    "system",
				Content: req.SystemPrompt,
			})
		}

		for _, m := range req.Messages {
			messages = append(messages, openAIMessage{
				Role:    m.Role,
				Content: m.Content,
			})
		}

		// Build streaming request
		dashReq := openAIChatRequest{
			Model:    c.config.Model,
			Messages: messages,
			Stream:   true,
		}

		if req.MaxTokens > 0 {
			dashReq.MaxTokens = req.MaxTokens
		} else {
			dashReq.MaxTokens = c.config.MaxTokens
		}

		if req.Temperature > 0 {
			dashReq.Temperature = req.Temperature
		}

		// Add tools
		if len(req.Tools) > 0 {
			for _, t := range req.Tools {
				dashReq.Tools = append(dashReq.Tools, openAITool{
					Type: t.Type,
					Function: openAIToolFunc{
						Name:        t.Function.Name,
						Description: t.Function.Description,
						Parameters:  t.Function.Parameters,
					},
				})
			}
		}

		body, err := json.Marshal(dashReq)
		if err != nil {
			ch <- ChatStreamChunk{Error: fmt.Errorf("marshal request: %w", err)}
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST",
			c.config.BaseURL+"/chat/completions",
			bytes.NewReader(body))
		if err != nil {
			ch <- ChatStreamChunk{Error: fmt.Errorf("create request: %w", err)}
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			ch <- ChatStreamChunk{Error: fmt.Errorf("send request: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			respBody, _ := io.ReadAll(resp.Body)
			ch <- ChatStreamChunk{Error: fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))}
			return
		}

		// Parse SSE stream
		reader := bufio.NewReader(resp.Body)

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					ch <- ChatStreamChunk{Done: true}
					return
				}
				ch <- ChatStreamChunk{Error: fmt.Errorf("read stream: %w", err)}
				return
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")

			if data == "[DONE]" {
				ch <- ChatStreamChunk{Done: true}
				return
			}

			var streamResp openAIChatResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				continue
			}

			if len(streamResp.Choices) > 0 {
				choice := streamResp.Choices[0]
				if choice.Delta.Content != "" {
					ch <- ChatStreamChunk{
						Content: choice.Delta.Content,
					}
				}

				if len(choice.Delta.ToolCalls) > 0 {
					for _, tc := range choice.Delta.ToolCalls {
						ch <- ChatStreamChunk{
							ToolCall: &ToolCall{
								ID:   tc.ID,
								Type: tc.Type,
								Function: ToolCallFunction{
									Name:      tc.Function.Name,
									Arguments: tc.Function.Arguments,
								},
							},
						}
					}
				}
			}
		}
	}()

	return ch, nil
}

// Embed generates embeddings using DashScope embedding model
func (c *DashScopeClient) Embed(ctx context.Context, text string) ([]float64, error) {
	results, err := c.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return results[0], nil
}

// EmbedBatch generates embeddings for multiple texts
func (c *DashScopeClient) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	// Use the configured embedding model
	model := c.config.Model
	if c.config.EmbeddingModel != "" {
		model = c.config.EmbeddingModel
	}

	fmt.Printf("DEBUG EmbedBatch: model=%s, texts=%v\n", model, texts)

	// DashScope embedding API uses different endpoint
	// https://dashscope.aliyuncs.com/api/v1/services/embeddings/text-embedding/text-embedding
	embedURL := "https://dashscope.aliyuncs.com/api/v1/services/embeddings/text-embedding/text-embedding"

	req := map[string]interface{}{
		"model": model,
		"input": map[string]interface{}{
			"texts": texts,
		},
		"parameters": map[string]interface{}{
			"text_type": "query",
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		embedURL,
		bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("DEBUG EmbedBatch: status=%d, response=%s\n", resp.StatusCode, string(respBody))

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// DashScope embedding response format
	// API returns: {"output":{"embeddings":[{"embedding":[...],"text_index":0}]}}
	var embedResp struct {
		Output struct {
			Embeddings []struct {
				Embedding []float64 `json:"embedding"`
				TextIndex int       `json:"text_index"`
			} `json:"embeddings"`
		} `json:"output"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(respBody, &embedResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if embedResp.Code != "" && embedResp.Code != "Success" {
		return nil, fmt.Errorf("API error: %s", embedResp.Message)
	}

	results := make([][]float64, len(texts))
	for _, r := range embedResp.Output.Embeddings {
		if r.TextIndex < len(results) {
			results[r.TextIndex] = r.Embedding
		}
	}

	return results, nil
}

// calculateCost calculates cost based on DashScope pricing
func (c *DashScopeClient) calculateCost(tokens int) float64 {
	// Qwen pricing: approximately $0.002/1K tokens for input + output
	// This is an approximation
	return float64(tokens) * 0.000002
}
