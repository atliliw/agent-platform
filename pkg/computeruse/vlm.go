package computeruse

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// VLMClient is a vision LLM that decides the next action from a screenshot.
type VLMClient interface {
	// Chat sends systemPrompt + userPrompt + screenshot and returns the model's
	// raw text response, expected to contain a single JSON action.
	Chat(ctx context.Context, systemPrompt, userPrompt string, screenshot []byte) (string, error)
}

// OpenAIVLMClient implements VLMClient against an OpenAI-compatible vision API
// (DashScope Qwen-VL, GPT-4o, etc.). The screenshot is embedded as a base64
// data URL in the OpenAI vision message format.
type OpenAIVLMClient struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// NewOpenAIVLMClient creates a new vision LLM client. The default base URL is
// DashScope's OpenAI-compatible endpoint and the default model is qwen-vl-max.
func NewOpenAIVLMClient(apiKey, baseURL, model string) *OpenAIVLMClient {
	if baseURL == "" {
		baseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	}
	if model == "" {
		model = "qwen-vl-max"
	}
	return &OpenAIVLMClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{Timeout: 90 * time.Second},
	}
}

// Chat sends the screenshot and prompts to the vision model.
func (c *OpenAIVLMClient) Chat(ctx context.Context, systemPrompt, userPrompt string, screenshot []byte) (string, error) {
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(screenshot)

	req := map[string]interface{}{
		"model": c.model,
		"messages": []map[string]interface{}{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": []map[string]interface{}{
				{"type": "text", "text": userPrompt},
				{"type": "image_url", "image_url": map[string]string{"url": dataURL}},
			}},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}
	if chatResp.Error != nil {
		return "", fmt.Errorf("API error: %s", chatResp.Error.Message)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices")
	}
	return chatResp.Choices[0].Message.Content, nil
}
