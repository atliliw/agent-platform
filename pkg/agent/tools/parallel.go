// Package tools provides tool orchestration capabilities
package tools

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// Parallel Executor
// ============================================================

// ParallelExecutor executes multiple tools in parallel
type ParallelExecutor struct {
	composer   *Composer
	maxWorkers int
	timeout    time.Duration
}

// NewParallelExecutor creates a new parallel executor
func NewParallelExecutor(composer *Composer, maxWorkers int) *ParallelExecutor {
	if maxWorkers <= 0 {
		maxWorkers = 10
	}
	return &ParallelExecutor{
		composer:   composer,
		maxWorkers: maxWorkers,
		timeout:    30 * time.Second,
	}
}

// SetTimeout sets the default timeout
func (p *ParallelExecutor) SetTimeout(timeout time.Duration) {
	p.timeout = timeout
}

// ParallelRequest represents a parallel execution request
type ParallelRequest struct {
	ToolName   string                 `json:"tool_name"`
	Parameters map[string]interface{} `json:"parameters"`
	ID         string                 `json:"id"`
	Timeout    time.Duration          `json:"timeout"`
}

// ParallelResult represents the result of parallel execution
type ParallelResult struct {
	TotalRequests  int                       `json:"total_requests"`
	Successful     int                       `json:"successful"`
	Failed         int                       `json:"failed"`
	TotalDuration  int64                     `json:"total_duration_ms"`
	Results        map[string]*ToolResult    `json:"results"`
	Errors         map[string]string         `json:"errors"`
}

// Execute executes multiple requests in parallel
func (p *ParallelExecutor) Execute(ctx context.Context, requests []ParallelRequest, toolCtx *ToolContext) (*ParallelResult, error) {
	start := time.Now()

	result := &ParallelResult{
		TotalRequests: len(requests),
		Results:       make(map[string]*ToolResult),
		Errors:        make(map[string]string),
	}

	if len(requests) == 0 {
		return result, nil
	}

	// Create semaphore for limiting concurrency
	sem := make(chan struct{}, p.maxWorkers)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, req := range requests {
		wg.Add(1)

		go func(r ParallelRequest) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Create step
			step := CompositionStep{
				ID:          r.ID,
				ToolName:    r.ToolName,
				Parameters:  r.Parameters,
				Timeout:     int(r.Timeout.Milliseconds()),
			}

			// Execute with timeout
			timeout := r.Timeout
			if timeout == 0 {
				timeout = p.timeout
			}

			execCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			toolResult, err := p.composer.executeStep(execCtx, step, toolCtx)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				result.Failed++
				result.Errors[r.ID] = err.Error()
			} else {
				result.Results[r.ID] = toolResult
				if toolResult.Success {
					result.Successful++
				} else {
					result.Failed++
				}
			}
		}(req)
	}

	// Wait for all to complete
	wg.Wait()

	result.TotalDuration = time.Since(start).Milliseconds()
	return result, nil
}

// ExecuteWithCallback executes with a callback for each result
func (p *ParallelExecutor) ExecuteWithCallback(ctx context.Context, requests []ParallelRequest, toolCtx *ToolContext, callback func(id string, result *ToolResult)) error {
	sem := make(chan struct{}, p.maxWorkers)
	var wg sync.WaitGroup
	var hasError atomic.Bool

	for _, req := range requests {
		wg.Add(1)

		go func(r ParallelRequest) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			step := CompositionStep{
				ID:         r.ID,
				ToolName:   r.ToolName,
				Parameters: r.Parameters,
				Timeout:    int(r.Timeout.Milliseconds()),
			}

			timeout := r.Timeout
			if timeout == 0 {
				timeout = p.timeout
			}

			execCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			toolResult, err := p.composer.executeStep(execCtx, step, toolCtx)
			if err != nil {
				hasError.Store(true)
				toolResult = &ToolResult{
					ToolName: r.ToolName,
					Success:  false,
					Error:    err.Error(),
				}
			}

			if callback != nil {
				callback(r.ID, toolResult)
			}
		}(req)
	}

	wg.Wait()

	if hasError.Load() {
		return fmt.Errorf("one or more parallel executions failed")
	}
	return nil
}

// ============================================================
// Retry Executor
// ============================================================

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxRetries      int           `json:"max_retries"`
	InitialDelay    time.Duration `json:"initial_delay"`
	MaxDelay        time.Duration `json:"max_delay"`
	BackoffFactor   float64       `json:"backoff_factor"`
	RetryableErrors []string      `json:"retryable_errors"`
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:    3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
	}
}

// RetryExecutor executes tools with retry logic
type RetryExecutor struct {
	composer *Composer
	config   *RetryConfig
}

// NewRetryExecutor creates a new retry executor
func NewRetryExecutor(composer *Composer, config *RetryConfig) *RetryExecutor {
	if config == nil {
		config = DefaultRetryConfig()
	}
	return &RetryExecutor{
		composer: composer,
		config:   config,
	}
}

// ExecuteWithRetry executes a tool with retries
func (r *RetryExecutor) ExecuteWithRetry(ctx context.Context, step CompositionStep, toolCtx *ToolContext) (*ToolResult, error) {
	var lastResult *ToolResult
	var lastErr error

	delay := r.config.InitialDelay

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		result, err := r.composer.executeStep(ctx, step, toolCtx)

		if err == nil && result.Success {
			return result, nil
		}

		lastResult = result
		lastErr = err

		// Check if error is retryable
		if !r.isRetryable(result, err) {
			break
		}

		// Wait before retry
		if attempt < r.config.MaxRetries {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}

			// Calculate next delay with exponential backoff
			delay = time.Duration(float64(delay) * r.config.BackoffFactor)
			if delay > r.config.MaxDelay {
				delay = r.config.MaxDelay
			}
		}
	}

	if lastResult != nil {
		lastResult.RetryCount = r.config.MaxRetries
		return lastResult, nil
	}

	return nil, lastErr
}

// isRetryable checks if an error is retryable
func (r *RetryExecutor) isRetryable(result *ToolResult, err error) bool {
	// Network errors are generally retryable
	if err != nil {
		errStr := err.Error()
		for _, retryable := range r.config.RetryableErrors {
			if strings.Contains(errStr, retryable) {
				return true
			}
		}

		// Default retryable errors
		defaultRetryable := []string{"timeout", "connection refused", "temporary", "rate limit"}
		for _, retryable := range defaultRetryable {
			if strings.Contains(strings.ToLower(errStr), retryable) {
				return true
			}
		}
	}

	return false
}

// ============================================================
// Circuit Breaker
// ============================================================

// CircuitState represents circuit breaker state
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // Normal operation
	CircuitOpen                          // Failing, reject all requests
	CircuitHalfOpen                      // Testing if service recovered
)

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name          string
	maxFailures   int
	timeout       time.Duration
	state         CircuitState
	failures      int
	lastFailTime  time.Time
	successCount  int
	mu            sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, maxFailures int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:        name,
		maxFailures: maxFailures,
		timeout:     timeout,
		state:       CircuitClosed,
	}
}

// Execute executes a function through the circuit breaker
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() (*ToolResult, error)) (*ToolResult, error) {
	// Check if circuit is open
	if !cb.allowRequest() {
		return nil, fmt.Errorf("circuit breaker %s is open", cb.name)
	}

	result, err := fn()

	if err != nil || (result != nil && !result.Success) {
		cb.recordFailure()
		return result, err
	}

	cb.recordSuccess()
	return result, nil
}

// allowRequest checks if requests should be allowed
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.RLock()
	state := cb.state
	lastFail := cb.lastFailTime
	cb.mu.RUnlock()

	switch state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// Check if timeout has passed
		if time.Since(lastFail) > cb.timeout {
			cb.mu.Lock()
			cb.state = CircuitHalfOpen
			cb.mu.Unlock()
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	default:
		return false
	}
}

// recordFailure records a failure
func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailTime = time.Now()
	cb.successCount = 0

	if cb.failures >= cb.maxFailures {
		cb.state = CircuitOpen
	}
}

// recordSuccess records a success
func (cb *CircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.successCount++

	if cb.state == CircuitHalfOpen && cb.successCount >= 3 {
		cb.state = CircuitClosed
	}
}

// GetState returns the current state
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset resets the circuit breaker
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = CircuitClosed
	cb.failures = 0
	cb.successCount = 0
}

// ============================================================
// Rate Limiter
// ============================================================

// RateLimiter limits the rate of tool executions
type RateLimiter struct {
	name       string
	maxPerSec  float64
	tokens     float64
	lastUpdate time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(name string, maxPerSec float64) *RateLimiter {
	return &RateLimiter{
		name:       name,
		maxPerSec:  maxPerSec,
		tokens:     maxPerSec,
		lastUpdate: time.Now(),
	}
}

// Allow checks if a request should be allowed
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastUpdate).Seconds()
	rl.lastUpdate = now

	// Add tokens based on elapsed time
	rl.tokens += elapsed * rl.maxPerSec
	if rl.tokens > rl.maxPerSec {
		rl.tokens = rl.maxPerSec
	}

	if rl.tokens >= 1 {
		rl.tokens -= 1
		return true
	}

	return false
}

// Wait waits until a request can be made
func (rl *RateLimiter) Wait(ctx context.Context) error {
	for {
		if rl.Allow() {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Millisecond * 10):
		}
	}
}

// ============================================================
// Tool Registry
// ============================================================

// ToolRegistry manages tool definitions and executors
type ToolRegistry struct {
	tools     map[string]*Tool
	executors map[string]Executor
	metadata  map[string]map[string]interface{}
	mu        sync.RWMutex
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools:     make(map[string]*Tool),
		executors: make(map[string]Executor),
		metadata:  make(map[string]map[string]interface{}),
	}
}

// Register registers a tool with its executor
func (r *ToolRegistry) Register(tool *Tool, executor Executor) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if tool.Name == "" {
		return fmt.Errorf("tool name is required")
	}

	r.tools[tool.Name] = tool
	r.executors[tool.Name] = executor
	r.metadata[tool.Name] = make(map[string]interface{})

	return nil
}

// Unregister removes a tool
func (r *ToolRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.tools, name)
	delete(r.executors, name)
	delete(r.metadata, name)
}

// GetTool gets a tool definition
func (r *ToolRegistry) GetTool(name string) (*Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, ok := r.tools[name]
	return tool, ok
}

// GetExecutor gets a tool executor
func (r *ToolRegistry) GetExecutor(name string) (Executor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	executor, ok := r.executors[name]
	return executor, ok
}

// ListTools lists all registered tools
func (r *ToolRegistry) ListTools() []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]*Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// ListByCategory lists tools by category
func (r *ToolRegistry) ListByCategory(category string) []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var tools []*Tool
	for _, tool := range r.tools {
		if tool.Category == category {
			tools = append(tools, tool)
		}
	}
	return tools
}

// SetMetadata sets metadata for a tool
func (r *ToolRegistry) SetMetadata(toolName string, key string, value interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if meta, ok := r.metadata[toolName]; ok {
		meta[key] = value
	}
}

// GetMetadata gets metadata for a tool
func (r *ToolRegistry) GetMetadata(toolName string) map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	meta, ok := r.metadata[toolName]
	if !ok {
		return nil
	}

	// Return a copy
	result := make(map[string]interface{})
	for k, v := range meta {
		result[k] = v
	}
	return result
}

// ============================================================
// Tool Statistics
// ============================================================

// ToolStats tracks tool usage statistics
type ToolStats struct {
	CallCount      int64         `json:"call_count"`
	SuccessCount   int64         `json:"success_count"`
	FailureCount   int64         `json:"failure_count"`
	TotalDuration  int64         `json:"total_duration_ms"`
	AvgDuration    float64       `json:"avg_duration_ms"`
	MaxDuration    int64         `json:"max_duration_ms"`
	MinDuration    int64         `json:"min_duration_ms"`
	LastCalled     time.Time     `json:"last_called"`
	LastError      string        `json:"last_error"`
}

// StatsCollector collects tool statistics
type StatsCollector struct {
	stats    map[string]*ToolStats
	mu       sync.RWMutex
}

// NewStatsCollector creates a new stats collector
func NewStatsCollector() *StatsCollector {
	return &StatsCollector{
		stats: make(map[string]*ToolStats),
	}
}

// Record records a tool execution
func (s *StatsCollector) Record(toolName string, duration int64, success bool, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stats, ok := s.stats[toolName]
	if !ok {
		stats = &ToolStats{
			MinDuration: -1, // Sentinel for "not set"
		}
		s.stats[toolName] = stats
	}

	stats.CallCount++
	stats.TotalDuration += duration
	stats.AvgDuration = float64(stats.TotalDuration) / float64(stats.CallCount)
	stats.LastCalled = time.Now()

	if duration > stats.MaxDuration {
		stats.MaxDuration = duration
	}
	if stats.MinDuration < 0 || duration < stats.MinDuration {
		stats.MinDuration = duration
	}

	if success {
		stats.SuccessCount++
	} else {
		stats.FailureCount++
		stats.LastError = errMsg
	}
}

// GetStats gets statistics for a tool
func (s *StatsCollector) GetStats(toolName string) *ToolStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats, ok := s.stats[toolName]
	if !ok {
		return nil
	}

	// Return a copy
	result := *stats
	return &result
}

// GetAllStats gets all statistics
func (s *StatsCollector) GetAllStats() map[string]*ToolStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*ToolStats)
	for name, stats := range s.stats {
		result[name] = stats
	}
	return result
}

// Reset resets all statistics
func (s *StatsCollector) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stats = make(map[string]*ToolStats)
}
