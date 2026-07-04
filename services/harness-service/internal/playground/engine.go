// Package playground provides Prompt Playground engine for testing and comparing LLM responses
package playground

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"agent-platform/pkg/llm"
)

// PlaygroundEngine provides Prompt Playground functionality
type PlaygroundEngine struct {
	llmClient llm.Client
	recorder  *Recorder
	mu        sync.RWMutex
}

// PlaygroundRequest represents a playground execution request
type PlaygroundRequest struct {
	Model       string                 `json:"model"`
	Messages    []llm.Message          `json:"messages"`
	Temperature float64                `json:"temperature"`
	MaxTokens   int                    `json:"max_tokens"`
	TopP        float64                `json:"top_p"`
	Parameters  map[string]interface{} `json:"parameters"`
	TenantID    string                 `json:"tenant_id"`
	UserID      string                 `json:"user_id"`
	SessionID   string                 `json:"session_id"`
}

// PlaygroundResult represents a playground execution result
type PlaygroundResult struct {
	Content      string    `json:"content"`
	TotalTokens  int64     `json:"total_tokens"`
	InputTokens  int64     `json:"input_tokens"`
	OutputTokens int64     `json:"output_tokens"`
	Cost         float64   `json:"cost"`
	Latency      int64     `json:"latency"` // milliseconds
	Model        string    `json:"model"`
	FinishReason string    `json:"finish_reason"`
	LogID        string    `json:"log_id"`
	CreatedAt    time.Time `json:"created_at"`
}

// CompareModelsRequest represents a multi-model comparison request
type CompareModelsRequest struct {
	Models      []string    `json:"models"`
	Messages    []llm.Message `json:"messages"`
	Temperature float64     `json:"temperature"`
	MaxTokens   int         `json:"max_tokens"`
	TopP        float64     `json:"top_p"`
	TenantID    string      `json:"tenant_id"`
	UserID      string      `json:"user_id"`
}

// CompareModelsResponse represents multi-model comparison results
type CompareModelsResponse struct {
	Results    []*PlaygroundResult `json:"results"`
	Comparison *ModelComparison    `json:"comparison"`
	CreatedAt  time.Time           `json:"created_at"`
}

// ModelComparison provides comparison metrics across models
type ModelComparison struct {
	BestModel     string  `json:"best_model"`     // Lowest latency or best quality
	FastestModel  string  `json:"fastest_model"`  // Lowest latency
	CheapestModel string  `json:"cheapest_model"` // Lowest cost
	AvgLatency    float64 `json:"avg_latency"`
	AvgCost       float64 `json:"avg_cost"`
	AvgTokens     float64 `json:"avg_tokens"`
}

// StreamChunk represents a streaming response chunk
type StreamChunk struct {
	Content   string    `json:"content"`
	Done      bool      `json:"done"`
	Error     error     `json:"error,omitempty"`
	LogID     string    `json:"log_id"`
	CreatedAt time.Time `json:"created_at"`
}

// NewPlaygroundEngine creates a new playground engine
func NewPlaygroundEngine(llmClient llm.Client, recorder *Recorder) *PlaygroundEngine {
	return &PlaygroundEngine{
		llmClient: llmClient,
		recorder:  recorder,
	}
}

// Execute executes a single prompt request
func (e *PlaygroundEngine) Execute(ctx context.Context, req *PlaygroundRequest) (*PlaygroundResult, error) {
	start := time.Now()
	logID := generateLogID()

	// Build LLM request
	llmReq := &llm.ChatRequest{
		Model:       req.Model,
		Messages:    req.Messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}

	// Call LLM
	resp, err := e.llmClient.Chat(ctx, llmReq)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	latency := time.Since(start).Milliseconds()

	// Estimate input/output tokens (rough split)
	inputTokens := int64(resp.TotalTokens * 6 / 10)  // ~60% input
	outputTokens := int64(resp.TotalTokens * 4 / 10) // ~40% output

	result := &PlaygroundResult{
		Content:      resp.Content,
		TotalTokens:  int64(resp.TotalTokens),
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Cost:         resp.Cost,
		Latency:      latency,
		Model:        req.Model,
		FinishReason: resp.FinishReason,
		LogID:        logID,
		CreatedAt:    time.Now(),
	}

	// Record history
	if e.recorder != nil {
		history := &PlaygroundHistory{
			ID:          logID,
			TenantID:    req.TenantID,
			UserID:      req.UserID,
			SessionID:   req.SessionID,
			Model:       req.Model,
			Messages:    req.Messages,
			Result:      result,
			Temperature: req.Temperature,
			MaxTokens:   req.MaxTokens,
			TopP:        req.TopP,
			Parameters:  req.Parameters,
			CreatedAt:   time.Now(),
		}
		if err := e.recorder.Save(ctx, history); err != nil {
			fmt.Printf("Warning: failed to save playground history: %v\n", err)
		}
	}

	return result, nil
}

// CompareModels executes prompts across multiple models in parallel and compares results
func (e *PlaygroundEngine) CompareModels(ctx context.Context, req *CompareModelsRequest) (*CompareModelsResponse, error) {
	if len(req.Models) == 0 {
		return nil, fmt.Errorf("at least one model required")
	}

	start := time.Now()
	logID := generateLogID()

	// Execute in parallel
	var wg sync.WaitGroup
	results := make([]*PlaygroundResult, len(req.Models))
	errors := make([]error, len(req.Models))

	for i, model := range req.Models {
		wg.Add(1)
		go func(idx int, modelName string) {
			defer wg.Done()

			playgroundReq := &PlaygroundRequest{
				Model:       modelName,
				Messages:    req.Messages,
				Temperature: req.Temperature,
				MaxTokens:   req.MaxTokens,
				TopP:        req.TopP,
				TenantID:    req.TenantID,
				UserID:      req.UserID,
			}

			result, err := e.Execute(ctx, playgroundReq)
			results[idx] = result
			errors[idx] = err
		}(i, model)
	}

	wg.Wait()

	// Check for errors
	for i, err := range errors {
		if err != nil {
			fmt.Printf("Warning: model %s execution failed: %v\n", req.Models[i], err)
		}
	}

	// Build comparison metrics
	comparison := e.buildComparison(results)

	response := &CompareModelsResponse{
		Results:    results,
		Comparison: comparison,
		CreatedAt:  time.Now(),
	}

	// Record comparison history
	if e.recorder != nil {
		history := &PlaygroundHistory{
			ID:          logID,
			TenantID:    req.TenantID,
			UserID:      req.UserID,
			Model:       "comparison",
			Messages:    req.Messages,
			Comparison:  response,
			Temperature: req.Temperature,
			MaxTokens:   req.MaxTokens,
			TopP:        req.TopP,
			CreatedAt:   time.Now(),
		}
		if err := e.recorder.Save(ctx, history); err != nil {
			fmt.Printf("Warning: failed to save comparison history: %v\n", err)
		}
	}

	_ = time.Since(start) // Track total comparison latency (unused for now)

	return response, nil
}

// StreamExecute executes a prompt with streaming response
func (e *PlaygroundEngine) StreamExecute(ctx context.Context, req *PlaygroundRequest) (<-chan StreamChunk, error) {
	logID := generateLogID()

	// Build LLM request with streaming enabled
	llmReq := &llm.ChatRequest{
		Model:       req.Model,
		Messages:    req.Messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      true,
	}

	// Get streaming channel from LLM
	llmStream, err := e.llmClient.ChatStream(ctx, llmReq)
	if err != nil {
		return nil, fmt.Errorf("LLM stream call failed: %w", err)
	}

	// Create output channel
	output := make(chan StreamChunk, 100)

	// Forward LLM stream chunks
	go func() {
		defer close(output)

		var fullContent string
		var totalTokens int64
		start := time.Now()

		for chunk := range llmStream {
			if chunk.Error != nil {
				output <- StreamChunk{
					Error:     chunk.Error,
					LogID:     logID,
					CreatedAt: time.Now(),
				}
				return
			}

			fullContent += chunk.Content
			totalTokens++

			output <- StreamChunk{
				Content:   chunk.Content,
				Done:      chunk.Done,
				LogID:     logID,
				CreatedAt: time.Now(),
			}

			if chunk.Done {
				// Record streaming history
				if e.recorder != nil {
					result := &PlaygroundResult{
						Content:     fullContent,
						TotalTokens: totalTokens,
						Model:       req.Model,
						Latency:     time.Since(start).Milliseconds(),
						LogID:       logID,
						CreatedAt:   time.Now(),
					}
					history := &PlaygroundHistory{
						ID:          logID,
						TenantID:    req.TenantID,
						UserID:      req.UserID,
						SessionID:   req.SessionID,
						Model:       req.Model,
						Messages:    req.Messages,
						Result:      result,
						Temperature: req.Temperature,
						MaxTokens:   req.MaxTokens,
						TopP:        req.TopP,
						Parameters:  req.Parameters,
						Streamed:    true,
						CreatedAt:   time.Now(),
					}
					if err := e.recorder.Save(ctx, history); err != nil {
						fmt.Printf("Warning: failed to save streaming history: %v\n", err)
					}
				}
				break
			}
		}
	}()

	return output, nil
}

// buildComparison builds comparison metrics from results
func (e *PlaygroundEngine) buildComparison(results []*PlaygroundResult) *ModelComparison {
	if len(results) == 0 {
		return &ModelComparison{}
	}

	var totalLatency, totalCost, totalTokens float64
	var fastestModel, cheapestModel string
	var minLatency int64 = int64(^uint64(0) >> 1) // Max int64
	var minCost float64 = 1e10                   // Large number

	validCount := 0

	for _, r := range results {
		if r == nil {
			continue
		}

		validCount++
		totalLatency += float64(r.Latency)
		totalCost += r.Cost
		totalTokens += float64(r.TotalTokens)

		if r.Latency < minLatency {
			minLatency = r.Latency
			fastestModel = r.Model
		}

		if r.Cost < minCost {
			minCost = r.Cost
			cheapestModel = r.Model
		}
	}

	if validCount == 0 {
		return &ModelComparison{}
	}

	comparison := &ModelComparison{
		FastestModel:  fastestModel,
		CheapestModel: cheapestModel,
		BestModel:     fastestModel, // Default to fastest, could be enhanced with quality scoring
		AvgLatency:    totalLatency / float64(validCount),
		AvgCost:       totalCost / float64(validCount),
		AvgTokens:     totalTokens / float64(validCount),
	}

	return comparison
}

// generateLogID generates a unique log ID
func generateLogID() string {
	return fmt.Sprintf("playground-%d", time.Now().UnixNano())
}

// EstimateTokens estimates token count for messages (rough approximation)
func EstimateTokens(messages []llm.Message) int {
	var totalChars int
	for _, m := range messages {
		totalChars += len(m.Role) + len(m.Content)
	}
	// Rough estimate: ~4 characters per token for English, ~2 for Chinese
	return totalChars / 3
}

// GetHistory retrieves playground history
func (e *PlaygroundEngine) GetHistory(ctx context.Context, tenantID, userID string, limit int) ([]*PlaygroundHistory, error) {
	if e.recorder == nil {
		return nil, fmt.Errorf("recorder not configured")
	}
	return e.recorder.List(ctx, tenantID, userID, limit)
}

// GetRecorder returns the recorder instance
func (e *PlaygroundEngine) GetRecorder() *Recorder {
	return e.recorder
}

// DeleteHistory deletes playground history
func (e *PlaygroundEngine) DeleteHistory(ctx context.Context, historyID string) error {
	if e.recorder == nil {
		return fmt.Errorf("recorder not configured")
	}
	return e.recorder.Delete(ctx, historyID)
}

// ToJSON converts PlaygroundResult to JSON string
func (r *PlaygroundResult) ToJSON() string {
	data, err := json.Marshal(r)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// ToJSON converts CompareModelsResponse to JSON string
func (r *CompareModelsResponse) ToJSON() string {
	data, err := json.Marshal(r)
	if err != nil {
		return "{}"
	}
	return string(data)
}