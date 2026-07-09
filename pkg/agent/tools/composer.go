// Package tools provides tool orchestration and composition capabilities
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ============================================================
// Tool Definition
// ============================================================

// Tool represents a tool definition
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]Parameter   `json:"parameters"`
	Category    string                 `json:"category"`
	Version     string                 `json:"version"`
	Timeout     int                    `json:"timeout_ms"`
	Retryable   bool                   `json:"retryable"`
	MaxRetries  int                    `json:"max_retries"`
	Dependencies []string              `json:"dependencies"` // Tools this depends on
	Metadata    map[string]interface{} `json:"metadata"`
}

// Parameter represents a tool parameter
type Parameter struct {
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default"`
	Enum        []string    `json:"enum"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	ToolName    string      `json:"tool_name"`
	Success     bool        `json:"success"`
	Output      interface{} `json:"output"`
	Error       string      `json:"error,omitempty"`
	Duration    int64       `json:"duration_ms"`
	RetryCount  int         `json:"retry_count"`
	Cached      bool        `json:"cached"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ToolContext provides context for tool execution
type ToolContext struct {
	SessionID   string
	AgentID     string
	UserID      string
	TraceID     string
	Parameters  map[string]interface{}
	PreviousResults map[string]*ToolResult
	Timeout     time.Duration
	Metadata    map[string]interface{}
}

// ============================================================
// Tool Executor
// ============================================================

// Executor executes a single tool
type Executor interface {
	Execute(ctx context.Context, toolCtx *ToolContext) (*ToolResult, error)
	Name() string
}

// ExecutorFunc is a function that implements Executor
type ExecutorFunc func(ctx context.Context, toolCtx *ToolContext) (*ToolResult, error)

// Execute implements Executor
func (f ExecutorFunc) Execute(ctx context.Context, toolCtx *ToolContext) (*ToolResult, error) {
	return f(ctx, toolCtx)
}

// Name implements Executor
func (f ExecutorFunc) Name() string {
	return "executor"
}

// ============================================================
// Tool Composer
// ============================================================

// CompositionType defines how tools are composed
type CompositionType string

const (
	CompositionSequential CompositionType = "sequential" // Execute one after another
	CompositionParallel   CompositionType = "parallel"   // Execute simultaneously
	CompositionConditional CompositionType = "conditional" // Execute based on conditions
	CompositionLoop       CompositionType = "loop"       // Execute in loop
)

// CompositionStep represents a step in a composition
type CompositionStep struct {
	ID           string                 `json:"id"`
	ToolName     string                 `json:"tool_name"`
	Parameters   map[string]interface{} `json:"parameters"`
	Condition    string                 `json:"condition"`     // Condition for conditional execution
	OnError      string                 `json:"on_error"`      // "continue", "stop", "retry"
	MaxRetries   int                    `json:"max_retries"`
	Timeout      int                    `json:"timeout_ms"`
	Dependencies []string               `json:"dependencies"`  // Step IDs that must complete first
}

// CompositionPlan represents a tool composition plan
type CompositionPlan struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Type        CompositionType    `json:"type"`
	Steps       []CompositionStep  `json:"steps"`
	Variables   map[string]string  `json:"variables"`  // Variable mappings
	CreatedAt   time.Time          `json:"created_at"`
}

// CompositionResult represents the result of a composition execution
type CompositionResult struct {
	PlanID      string                  `json:"plan_id"`
	Success     bool                    `json:"success"`
	StepResults map[string]*ToolResult  `json:"step_results"`
	TotalDuration int64                  `json:"total_duration_ms"`
	Error       string                  `json:"error,omitempty"`
	FailedStep  string                  `json:"failed_step,omitempty"`
	Metadata    map[string]interface{}  `json:"metadata,omitempty"`
}

// Composer composes and executes multiple tools
type Composer struct {
	executors map[string]Executor
	plans    map[string]*CompositionPlan
	cache    *ResultCache
	mu       sync.RWMutex
}

// NewComposer creates a new tool composer
func NewComposer() *Composer {
	return &Composer{
		executors: make(map[string]Executor),
		plans:    make(map[string]*CompositionPlan),
		cache:    NewResultCache(1000, 5*time.Minute),
	}
}

// RegisterExecutor registers a tool executor
func (c *Composer) RegisterExecutor(toolName string, executor Executor) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.executors[toolName] = executor
}

// RegisterPlan registers a composition plan
func (c *Composer) RegisterPlan(plan *CompositionPlan) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if plan.CreatedAt.IsZero() {
		plan.CreatedAt = time.Now()
	}

	c.plans[plan.ID] = plan
}

// ExecutePlan executes a composition plan
func (c *Composer) ExecutePlan(ctx context.Context, planID string, toolCtx *ToolContext) (*CompositionResult, error) {
	c.mu.RLock()
	plan, ok := c.plans[planID]
	c.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	return c.executePlan(ctx, plan, toolCtx)
}

// ExecuteSteps executes a list of steps
func (c *Composer) ExecuteSteps(ctx context.Context, steps []CompositionStep, toolCtx *ToolContext) (*CompositionResult, error) {
	plan := &CompositionPlan{
		ID:        fmt.Sprintf("adhoc-%d", time.Now().UnixNano()),
		Type:      CompositionSequential,
		Steps:     steps,
		CreatedAt: time.Now(),
	}

	return c.executePlan(ctx, plan, toolCtx)
}

// executePlan executes a plan
func (c *Composer) executePlan(ctx context.Context, plan *CompositionPlan, toolCtx *ToolContext) (*CompositionResult, error) {
	start := time.Now()

	result := &CompositionResult{
		PlanID:      plan.ID,
		StepResults: make(map[string]*ToolResult),
		Success:     true,
	}

	// Initialize previous results map
	if toolCtx.PreviousResults == nil {
		toolCtx.PreviousResults = make(map[string]*ToolResult)
	}

	// Build dependency graph
	_, err := c.buildDependencyGraph(plan.Steps)
	if err != nil {
		return nil, err
	}

	// Execute based on composition type
	switch plan.Type {
	case CompositionSequential:
		err = c.executeSequential(ctx, plan, toolCtx, result)
	case CompositionParallel:
		err = c.executeParallel(ctx, plan, toolCtx, result)
	case CompositionConditional:
		err = c.executeConditional(ctx, plan, toolCtx, result)
	default:
		err = c.executeSequential(ctx, plan, toolCtx, result)
	}

	if err != nil {
		result.Success = false
		result.Error = err.Error()
	}

	result.TotalDuration = time.Since(start).Milliseconds()
	return result, nil
}

// executeSequential executes steps sequentially
func (c *Composer) executeSequential(ctx context.Context, plan *CompositionPlan, toolCtx *ToolContext, result *CompositionResult) error {
	for _, step := range plan.Steps {
		// Check dependencies
		for _, depID := range step.Dependencies {
			if depResult, ok := result.StepResults[depID]; ok {
				if !depResult.Success {
					return fmt.Errorf("dependency step %s failed", depID)
				}
				toolCtx.PreviousResults[depID] = depResult
			}
		}

		// Execute step
		stepResult, err := c.executeStep(ctx, step, toolCtx)
		if err != nil {
			return err
		}

		result.StepResults[step.ID] = stepResult
		toolCtx.PreviousResults[step.ID] = stepResult

		if !stepResult.Success {
			result.FailedStep = step.ID
			return fmt.Errorf("step %s failed: %s", step.ID, stepResult.Error)
		}
	}

	return nil
}

// executeParallel executes steps in parallel
func (c *Composer) executeParallel(ctx context.Context, plan *CompositionPlan, toolCtx *ToolContext, result *CompositionResult) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstError error

	for _, step := range plan.Steps {
		wg.Add(1)

		go func(s CompositionStep) {
			defer wg.Done()

			stepResult, err := c.executeStep(ctx, s, toolCtx)
			if err != nil {
				mu.Lock()
				if firstError == nil {
					firstError = err
				}
				mu.Unlock()
				return
			}

			mu.Lock()
			result.StepResults[s.ID] = stepResult
			if !stepResult.Success && result.FailedStep == "" {
				result.FailedStep = s.ID
			}
			mu.Unlock()
		}(step)
	}

	wg.Wait()
	return firstError
}

// executeConditional executes steps based on conditions
func (c *Composer) executeConditional(ctx context.Context, plan *CompositionPlan, toolCtx *ToolContext, result *CompositionResult) error {
	for _, step := range plan.Steps {
		// Evaluate condition
		if step.Condition != "" {
			shouldExecute, err := c.evaluateCondition(step.Condition, toolCtx.PreviousResults)
			if err != nil {
				return fmt.Errorf("evaluate condition for step %s: %w", step.ID, err)
			}

			if !shouldExecute {
				continue
			}
		}

		// Execute step
		stepResult, err := c.executeStep(ctx, step, toolCtx)
		if err != nil {
			return err
		}

		result.StepResults[step.ID] = stepResult
		toolCtx.PreviousResults[step.ID] = stepResult

		if !stepResult.Success && step.OnError == "stop" {
			result.FailedStep = step.ID
			return fmt.Errorf("step %s failed: %s", step.ID, stepResult.Error)
		}
	}

	return nil
}

// executeStep executes a single step
func (c *Composer) executeStep(ctx context.Context, step CompositionStep, toolCtx *ToolContext) (*ToolResult, error) {
	start := time.Now()

	result := &ToolResult{
		ToolName: step.ToolName,
	}

	// Get executor
	c.mu.RLock()
	executor, ok := c.executors[step.ToolName]
	c.mu.RUnlock()

	if !ok {
		result.Success = false
		result.Error = fmt.Sprintf("executor not found: %s", step.ToolName)
		return result, nil
	}

	// Check cache
	cacheKey := c.buildCacheKey(step.ToolName, step.Parameters)
	if cached, ok := c.cache.Get(cacheKey); ok {
		cached.Cached = true
		cached.Duration = time.Since(start).Milliseconds()
		return cached, nil
	}

	// Build execution context
	execCtx := &ToolContext{
		SessionID:        toolCtx.SessionID,
		AgentID:          toolCtx.AgentID,
		UserID:           toolCtx.UserID,
		TraceID:          toolCtx.TraceID,
		Parameters:       step.Parameters,
		PreviousResults:  toolCtx.PreviousResults,
		Metadata:         toolCtx.Metadata,
	}

	// Set timeout
	timeout := time.Duration(step.Timeout) * time.Millisecond
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Execute with retries
	var lastErr error
	maxRetries := step.MaxRetries
	if maxRetries == 0 {
		maxRetries = 1
	}

	for i := 0; i < maxRetries; i++ {
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)

		stepResult, err := executor.Execute(timeoutCtx, execCtx)
		cancel()

		if err == nil && stepResult.Success {
			result = stepResult
			result.RetryCount = i
			break
		}

		lastErr = err
		result.RetryCount = i + 1
	}

	if lastErr != nil {
		result.Success = false
		result.Error = lastErr.Error()
	}

	result.Duration = time.Since(start).Milliseconds()

	// Cache successful result
	if result.Success {
		c.cache.Set(cacheKey, result)
	}

	return result, nil
}

// buildDependencyGraph builds a DAG of step dependencies
func (c *Composer) buildDependencyGraph(steps []CompositionStep) (*DAG, error) {
	dag := NewDAG()

	// Add all steps as nodes
	for _, step := range steps {
		dag.AddNode(step.ID)
	}

	// Add edges for dependencies
	for _, step := range steps {
		for _, dep := range step.Dependencies {
			if err := dag.AddEdge(dep, step.ID); err != nil {
				return nil, fmt.Errorf("invalid dependency: %w", err)
			}
		}
	}

	// Check for cycles
	if dag.HasCycle() {
		return nil, fmt.Errorf("circular dependency detected in steps")
	}

	return dag, nil
}

// evaluateCondition evaluates a condition expression
func (c *Composer) evaluateCondition(condition string, results map[string]*ToolResult) (bool, error) {
	// Simple condition evaluation
	// Supports: "step_id.success", "step_id.output.field == value"

	// Check for .success
	if strings.HasSuffix(condition, ".success") {
		stepID := strings.TrimSuffix(condition, ".success")
		if result, ok := results[stepID]; ok {
			return result.Success, nil
		}
		return false, nil
	}

	// Check for equality
	if strings.Contains(condition, "==") {
		parts := strings.Split(condition, "==")
		if len(parts) == 2 {
			left := strings.TrimSpace(parts[0])
			right := strings.TrimSpace(parts[1])

			// Handle step_id.output.field
			if strings.Contains(left, ".output.") {
				// Parse step_id.output.field
				tokens := strings.Split(left, ".")
				if len(tokens) >= 3 {
					stepID := tokens[0]
					field := strings.Join(tokens[2:], ".")
					if result, ok := results[stepID]; ok {
						return c.compareOutput(result.Output, field, right), nil
					}
				}
			}

			return left == right, nil
		}
	}

	return true, nil
}

// compareOutput compares output field with value
func (c *Composer) compareOutput(output interface{}, field, value string) bool {
	// Simple implementation - would need more sophisticated comparison
	if output == nil {
		return false
	}

	outputStr := fmt.Sprintf("%v", output)
	return outputStr == value
}

// buildCacheKey builds a cache key
func (c *Composer) buildCacheKey(toolName string, params map[string]interface{}) string {
	paramsJSON, _ := json.Marshal(params)
	return fmt.Sprintf("%s:%s", toolName, string(paramsJSON))
}

// ListPlans lists all registered plans
func (c *Composer) ListPlans() []*CompositionPlan {
	c.mu.RLock()
	defer c.mu.RUnlock()

	plans := make([]*CompositionPlan, 0, len(c.plans))
	for _, plan := range c.plans {
		plans = append(plans, plan)
	}
	return plans
}

// ============================================================
// DAG - Directed Acyclic Graph
// ============================================================

// DAG represents a directed acyclic graph
type DAG struct {
	nodes map[string]bool
	edges map[string][]string
	mu    sync.RWMutex
}

// NewDAG creates a new DAG
func NewDAG() *DAG {
	return &DAG{
		nodes: make(map[string]bool),
		edges: make(map[string][]string),
	}
}

// AddNode adds a node to the DAG
func (d *DAG) AddNode(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.nodes[id] = true
}

// AddEdge adds a directed edge from one node to another
func (d *DAG) AddEdge(from, to string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.nodes[from] {
		return fmt.Errorf("node %s does not exist", from)
	}
	if !d.nodes[to] {
		return fmt.Errorf("node %s does not exist", to)
	}

	d.edges[from] = append(d.edges[from], to)
	return nil
}

// HasCycle checks if the graph has a cycle
func (d *DAG) HasCycle() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for node := range d.nodes {
		if d.hasCycleDFS(node, visited, recStack) {
			return true
		}
	}

	return false
}

// hasCycleDFS performs DFS to detect cycles
func (d *DAG) hasCycleDFS(node string, visited, recStack map[string]bool) bool {
	visited[node] = true
	recStack[node] = true

	for _, neighbor := range d.edges[node] {
		if !visited[neighbor] {
			if d.hasCycleDFS(neighbor, visited, recStack) {
				return true
			}
		} else if recStack[neighbor] {
			return true
		}
	}

	recStack[node] = false
	return false
}

// TopologicalSort returns nodes in topological order
func (d *DAG) TopologicalSort() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	visited := make(map[string]bool)
	var result []string

	for node := range d.nodes {
		if !visited[node] {
			d.topologicalSortDFS(node, visited, &result)
		}
	}

	// Reverse result
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// topologicalSortDFS performs DFS for topological sort
func (d *DAG) topologicalSortDFS(node string, visited map[string]bool, result *[]string) {
	visited[node] = true

	for _, neighbor := range d.edges[node] {
		if !visited[neighbor] {
			d.topologicalSortDFS(neighbor, visited, result)
		}
	}

	*result = append(*result, node)
}

// ============================================================
// Result Cache
// ============================================================

// CacheEntry represents a cache entry
type CacheEntry struct {
	Result    *ToolResult
	ExpiresAt time.Time
}

// ResultCache caches tool results
type ResultCache struct {
	entries  map[string]*CacheEntry
	maxSize  int
	ttl      time.Duration
	mu       sync.RWMutex
}

// NewResultCache creates a new result cache
func NewResultCache(maxSize int, ttl time.Duration) *ResultCache {
	cache := &ResultCache{
		entries: make(map[string]*CacheEntry),
		maxSize: maxSize,
		ttl:     ttl,
	}

	// Start cleanup goroutine
	go cache.cleanup()

	return cache
}

// Get retrieves a cached result
func (c *ResultCache) Get(key string) (*ToolResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}

	return entry.Result, true
}

// Set stores a result in cache
func (c *ResultCache) Set(key string, result *ToolResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check capacity
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	c.entries[key] = &CacheEntry{
		Result:    result,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

// Delete removes a cached result
func (c *ResultCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)
}

// Clear clears all cached results
func (c *ResultCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
}

// evictOldest removes the oldest entry
func (c *ResultCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestTime.IsZero() || entry.ExpiresAt.Before(oldestTime) {
			oldestTime = entry.ExpiresAt
			oldestKey = key
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// cleanup periodically removes expired entries
func (c *ResultCache) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.entries {
			if now.After(entry.ExpiresAt) {
				delete(c.entries, key)
			}
		}
		c.mu.Unlock()
	}
}

// Need to import strings
// End of composer.go
