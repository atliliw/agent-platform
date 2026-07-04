// Package gateway provides LLM Gateway functionality for multi-provider management
package gateway

import (
	"time"
)

// GatewayConfig represents a gateway configuration for a provider
type GatewayConfig struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Provider    string    `json:"provider"`     // openai, anthropic, azure, dashscope
	APIKey      string    `json:"api_key"`      // encrypted
	BaseURL     string    `json:"base_url"`
	Models      string    `json:"models"`       // JSON: []ModelConfig
	RateLimit   int       `json:"rate_limit"`   // requests per minute
	Timeout     int       `json:"timeout"`      // seconds
	RetryCount  int       `json:"retry_count"`
	Priority    int       `json:"priority"`     // for fallback ordering
	Enabled     bool      `json:"enabled"`
	TenantID    string    `json:"tenant_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ModelConfig represents a model configuration
type ModelConfig struct {
	ModelID     string  `json:"model_id"`
	ModelName   string  `json:"model_name"`
	MaxTokens   int     `json:"max_tokens"`
	InputPrice  float64 `json:"input_price"`  // per 1M tokens
	OutputPrice float64 `json:"output_price"` // per 1M tokens
}

// GatewayRoute represents a routing rule for requests
type GatewayRoute struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Pattern   string    `json:"pattern"`    // routing pattern (e.g., "high-latency", "cost-sensitive")
	ModelID   string    `json:"model_id"`   // primary model for this route
	Fallbacks string    `json:"fallbacks"`  // JSON: []fallback model IDs
	TenantID  string    `json:"tenant_id"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GatewayStats represents statistics for a provider
type GatewayStats struct {
	Provider       string    `json:"provider"`
	TotalRequests  int64     `json:"total_requests"`
	SuccessCount   int64     `json:"success_count"`
	ErrorCount     int64     `json:"error_count"`
	AvgLatency     float64   `json:"avg_latency"`     // milliseconds
	TotalTokens    int64     `json:"total_tokens"`
	TotalCost      float64   `json:"total_cost"`
	LastActiveTime time.Time `json:"last_active_time"`
}

// ChatRequest represents a gateway chat request
type ChatRequest struct {
	Provider    string            `json:"provider"`    // optional, auto-select if empty
	Model       string            `json:"model"`
	Messages    []Message         `json:"messages"`
	Temperature float64           `json:"temperature"`
	MaxTokens   int               `json:"max_tokens"`
	Parameters  map[string]string `json:"parameters"`
	TenantID    string            `json:"tenant_id"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse represents a gateway chat response
type ChatResponse struct {
	Content       string    `json:"content"`
	Model         string    `json:"model"`
	Provider      string    `json:"provider"`
	TotalTokens   int64     `json:"total_tokens"`
	Cost          float64   `json:"cost"`
	Latency       int64     `json:"latency"`        // milliseconds
	UsedFallback  bool      `json:"used_fallback"`
	OriginalModel string    `json:"original_model"` // the originally requested model
	Error         string    `json:"error,omitempty"`
}

// FallbackConfig represents fallback configuration
type FallbackConfig struct {
	Enabled       bool     `json:"enabled"`
	FallbackModels []string `json:"fallback_models"`
	MaxRetries    int      `json:"max_retries"`
}

// LoadBalanceStrategy represents load balancing strategy
type LoadBalanceStrategy string

const (
	RoundRobin     LoadBalanceStrategy = "round_robin"
	LeastLatency   LoadBalanceStrategy = "least_latency"
	LeastCost      LoadBalanceStrategy = "least_cost"
	WeightedRandom LoadBalanceStrategy = "weighted_random"
)

// ProviderType represents the LLM provider type
type ProviderType string

const (
	ProviderOpenAI     ProviderType = "openai"
	ProviderAnthropic  ProviderType = "anthropic"
	ProviderAzure      ProviderType = "azure"
	ProviderDashScope  ProviderType = "dashscope"
)

// IsFallbackError checks if an error should trigger fallback
func IsFallbackError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Common error patterns that should trigger fallback
	fallbackPatterns := []string{
		"rate limit",
		"timeout",
		"connection refused",
		"service unavailable",
		"internal server error",
		"overloaded",
		"capacity",
	}
	for _, pattern := range fallbackPatterns {
		if containsIgnoreCase(errStr, pattern) {
			return true
		}
	}
	return false
}

func containsIgnoreCase(s, substr string) bool {
	sLower := make([]byte, len(s))
	substrLower := make([]byte, len(substr))
	for i := range s {
		sLower[i] = toLower(s[i])
	}
	for i := range substr {
		substrLower[i] = toLower(substr[i])
	}
	for i := 0; i <= len(sLower)-len(substrLower); i++ {
		match := true
		for j := range substrLower {
			if sLower[i+j] != substrLower[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func toLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}