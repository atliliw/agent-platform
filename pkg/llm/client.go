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

// Config holds LLM client configuration
type Config struct {
	Provider       string
	APIKey         string
	BaseURL        string
	Model          string
	EmbeddingModel string
	MaxTokens      int
}

// Client is the LLM client interface
type Client interface {
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	ChatStream(ctx context.Context, req *ChatRequest) (<-chan ChatStreamChunk, error)
	Embed(ctx context.Context, text string) ([]float64, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float64, error)
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ToolDefinition represents a tool definition
type ToolDefinition struct {
	Type     string                 `json:"type"`
	Function ToolFunction           `json:"function"`
}

// ToolFunction represents a tool function
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolCall represents a tool call
type ToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function ToolCallFunction       `json:"function"`
}

// ToolCallFunction represents a tool call function
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatRequest represents a chat request
type ChatRequest struct {
	Messages     []Message        `json:"messages"`
	Model        string           `json:"model"`
	MaxTokens    int              `json:"max_tokens,omitempty"`
	Temperature  float64          `json:"temperature,omitempty"`
	Tools        []ToolDefinition `json:"tools,omitempty"`
	Stream       bool             `json:"stream,omitempty"`
	SystemPrompt string           `json:"-"`
}

// ChatResponse represents a chat response
type ChatResponse struct {
	Content      string     `json:"content"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	TotalTokens  int        `json:"total_tokens"`
	PromptTokens int        `json:"prompt_tokens"`
	FinishReason string     `json:"finish_reason"`
	Cost         float64    `json:"cost"`
}

// ChatStreamChunk represents a streaming chat chunk
type ChatStreamChunk struct {
	Content   string    `json:"content"`
	ToolCall  *ToolCall `json:"tool_call,omitempty"`
	Done      bool      `json:"done"`
	Error     error     `json:"error,omitempty"`
}

// OpenAIClient implements Client for OpenAI API
type OpenAIClient struct {
	config     Config
	httpClient *http.Client
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(cfg Config) *OpenAIClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 4096
	}
	return &OpenAIClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 300 * time.Second, // 5 分钟
		},
	}
}

type openAIChatRequest struct {
	Model       string           `json:"model"`
	Messages    []openAIMessage  `json:"messages"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
	Tools       []openAITool     `json:"tools,omitempty"`
	Stream      bool             `json:"stream,omitempty"`
}

type openAIMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []openAIToolCall `json:"tool_calls,omitempty"`
}

type openAITool struct {
	Type     string            `json:"type"`
	Function openAIToolFunc    `json:"function"`
}

type openAIToolFunc struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type openAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openAIToolCallFunc `json:"function"`
}

type openAIToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int            `json:"index"`
		Message      openAIMessage  `json:"message"`
		Delta        openAIMessage  `json:"delta,omitempty"`
		FinishReason string         `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// Chat sends a chat request
func (c *OpenAIClient) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
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

	// Build request
	openAIReq := openAIChatRequest{
		Model:       c.config.Model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}

	if req.MaxTokens == 0 {
		openAIReq.MaxTokens = c.config.MaxTokens
	}

	// Add tools
	if len(req.Tools) > 0 {
		for _, t := range req.Tools {
			openAIReq.Tools = append(openAIReq.Tools, openAITool{
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
	body, err := json.Marshal(openAIReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		c.config.BaseURL+"/chat/completions",
		strings.NewReader(string(body)))
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

	var openAIResp openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Build response
	result := &ChatResponse{
		TotalTokens:  openAIResp.Usage.TotalTokens,
		PromptTokens: openAIResp.Usage.PromptTokens,
	}

	if len(openAIResp.Choices) > 0 {
		choice := openAIResp.Choices[0]
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

	// Calculate cost
	result.Cost = c.calculateCost(result.TotalTokens)

	return result, nil
}

// ChatStream sends a streaming chat request - REAL implementation
func (c *OpenAIClient) ChatStream(ctx context.Context, req *ChatRequest) (<-chan ChatStreamChunk, error) {
	ch := make(chan ChatStreamChunk, 100)

	go func() {
		defer close(ch)

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

		// Build streaming request
		openAIReq := openAIChatRequest{
			Model:    c.config.Model,
			Messages: messages,
			Stream:   true, // Enable streaming
		}

		if req.MaxTokens > 0 {
			openAIReq.MaxTokens = req.MaxTokens
		} else {
			openAIReq.MaxTokens = c.config.MaxTokens
		}

		if req.Temperature > 0 {
			openAIReq.Temperature = req.Temperature
		}

		// Add tools
		if len(req.Tools) > 0 {
			for _, t := range req.Tools {
				openAIReq.Tools = append(openAIReq.Tools, openAITool{
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
		body, err := json.Marshal(openAIReq)
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

			// Skip empty lines
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// Check for data prefix
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			// Extract data
			data := strings.TrimPrefix(line, "data: ")

			// Check for stream end
			if data == "[DONE]" {
				ch <- ChatStreamChunk{Done: true}
				return
			}

			// Parse JSON
			var streamResp openAIChatResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				continue // Skip malformed chunks
			}

			// Extract content from delta
			if len(streamResp.Choices) > 0 {
				choice := streamResp.Choices[0]
				if choice.Delta.Content != "" {
					ch <- ChatStreamChunk{
						Content: choice.Delta.Content,
					}
				}

				// Check for tool calls in stream
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

			// Track tokens - unused but kept for reference
			_ = streamResp.Usage.TotalTokens
		}
	}()

	return ch, nil
}

// Embed generates embeddings for a single text
func (c *OpenAIClient) Embed(ctx context.Context, text string) ([]float64, error) {
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
func (c *OpenAIClient) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	req := map[string]interface{}{
		"model": "text-embedding-3-small",
		"input": texts,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		c.config.BaseURL+"/embeddings",
		strings.NewReader(string(body)))
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

	var embedResp struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	results := make([][]float64, len(embedResp.Data))
	for _, d := range embedResp.Data {
		if d.Index < len(results) {
			results[d.Index] = d.Embedding
		}
	}

	return results, nil
}

func (c *OpenAIClient) calculateCost(tokens int) float64 {
	// GPT-4 pricing: $0.03/1K input, $0.06/1K output
	// Simplified: assume $0.04/1K tokens average
	return float64(tokens) * 0.00004
}

// NewClient creates a new LLM client based on provider
func NewClient(cfg Config) (Client, error) {
	switch cfg.Provider {
	case "openai":
		return NewOpenAIClient(cfg), nil
	case "dashscope":
		return NewDashScopeClient(cfg), nil
	default:
		// Default to DashScope if using DashScope-compatible base URL
		if strings.Contains(cfg.BaseURL, "dashscope") {
			return NewDashScopeClient(cfg), nil
		}
		return NewOpenAIClient(cfg), nil
	}
}