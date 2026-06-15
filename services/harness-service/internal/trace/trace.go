// Package trace provides distributed tracing for harness service
package trace

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SpanLayer defines the layer of a span
type SpanLayer int

const (
	LayerComponent  SpanLayer = 0 // Component layer (tools, agents)
	LayerExperience SpanLayer = 1 // Experience layer (user interactions)
	LayerDecision   SpanLayer = 2 // Decision layer (routing, planning)
	LayerTool       SpanLayer = 3 // Tool execution layer
	LayerLLM        SpanLayer = 4 // LLM call layer
)

// SpanStatus defines the status of a span
type SpanStatus string

const (
	StatusOK      SpanStatus = "ok"
	StatusError   SpanStatus = "error"
	StatusBlocked SpanStatus = "blocked"
	StatusPending SpanStatus = "pending"
)

// Span represents a single operation in a trace
type Span struct {
	ID           string                 `json:"id"`
	TraceID      string                 `json:"trace_id"`
	ParentID     string                 `json:"parent_id,omitempty"`
	Layer        SpanLayer              `json:"layer"`
	Operation    string                 `json:"operation"`
	AgentType    string                 `json:"agent_type,omitempty"`
	Model        string                 `json:"model,omitempty"`
	InputTokens  int                    `json:"input_tokens"`
	OutputTokens int                    `json:"output_tokens"`
	CostUSD      float64                `json:"cost_usd"`
	LatencyMs    int64                  `json:"latency_ms"`
	Status       SpanStatus             `json:"status"` // ok, error, blocked, pending
	ErrorMsg     string                 `json:"error_msg,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	StartedAt    time.Time              `json:"started_at"`
	EndedAt      time.Time              `json:"ended_at,omitempty"`
	Children     []*Span                `json:"children,omitempty"` // child spans
	Events       []SpanEvent            `json:"events,omitempty"`   // events within span
	Attributes   map[string]string      `json:"attributes,omitempty"`
	ToolName     string                 `json:"tool_name,omitempty"`
	ToolArgs     string                 `json:"tool_args,omitempty"`
	ToolResult   string                 `json:"tool_result,omitempty"`
	LLMRequest   string                 `json:"llm_request,omitempty"`
	LLMResponse  string                 `json:"llm_response,omitempty"`
	RetryCount   int                    `json:"retry_count"`
	CacheHit     bool                   `json:"cache_hit"`
}

// SpanEvent represents an event within a span
type SpanEvent struct {
	Name      string                 `json:"name"`
	Timestamp time.Time              `json:"timestamp"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// TraceContext holds the current trace context
type TraceContext struct {
	TraceID  string
	ParentID string
	SpanID   string
	spans    []*Span
	spanMap  map[string]*Span // Map for quick lookup by ID
	mu       sync.Mutex
	root     *Span
}

// NewTraceContext creates a new trace context
func NewTraceContext() *TraceContext {
	return &TraceContext{
		TraceID: uuid.New().String(),
		spans:   make([]*Span, 0),
		spanMap: make(map[string]*Span),
	}
}

// StartSpan starts a new span
func StartSpan(traceID, operation string, layer SpanLayer, parentID string) *Span {
	return &Span{
		ID:         uuid.New().String(),
		TraceID:    traceID,
		ParentID:   parentID,
		Layer:      layer,
		Operation:  operation,
		Status:     StatusPending,
		StartedAt:  time.Now(),
		Metadata:   make(map[string]interface{}),
		Attributes: make(map[string]string),
		Children:   make([]*Span, 0),
		Events:     make([]SpanEvent, 0),
	}
}

// End ends the span and records duration
func (s *Span) End() {
	s.EndedAt = time.Now()
	s.LatencyMs = time.Since(s.StartedAt).Milliseconds()
	if s.Status == StatusPending {
		s.Status = StatusOK
	}
}

// EndWithError ends the span with an error
func (s *Span) EndWithError(err string) {
	s.End()
	s.Status = StatusError
	s.ErrorMsg = err
}

// EndBlocked ends the span as blocked by guardrail
func (s *Span) EndBlocked(reason string) {
	s.End()
	s.Status = StatusBlocked
	s.ErrorMsg = reason
}

// SetError marks the span as failed
func (s *Span) SetError(err string) {
	s.Status = StatusError
	s.ErrorMsg = err
}

// SetBlocked marks the span as blocked by guardrail
func (s *Span) SetBlocked(reason string) {
	s.Status = StatusBlocked
	s.ErrorMsg = reason
}

// SetAgent sets agent type
func (s *Span) SetAgent(agentType string) {
	s.AgentType = agentType
}

// SetModel sets the model used
func (s *Span) SetModel(model string) {
	s.Model = model
}

// SetTokens sets token counts
func (s *Span) SetTokens(input, output int) {
	s.InputTokens = input
	s.OutputTokens = output
}

// SetCost sets the cost
func (s *Span) SetCost(cost float64) {
	s.CostUSD = cost
}

// SetTool sets tool information
func (s *Span) SetTool(name, args, result string) {
	s.ToolName = name
	s.ToolArgs = args
	s.ToolResult = result
}

// SetLLM sets LLM request/response
func (s *Span) SetLLM(request, response string) {
	s.LLMRequest = request
	s.LLMResponse = response
}

// AddEvent adds an event to the span
func (s *Span) AddEvent(name string, attributes map[string]interface{}) {
	s.Events = append(s.Events, SpanEvent{
		Name:       name,
		Timestamp:  time.Now(),
		Attributes: attributes,
	})
}

// SetAttribute sets an attribute
func (s *Span) SetAttribute(key, value string) {
	s.Attributes[key] = value
}

// SetMetadata sets metadata
func (s *Span) SetMetadata(key string, value interface{}) {
	s.Metadata[key] = value
}

// AddChild adds a child span
func (s *Span) AddChild(child *Span) {
	s.Children = append(s.Children, child)
}

// ToJSON converts span to JSON
func (s *Span) ToJSON() string {
	data, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	return string(data)
}

// GetDuration returns the span duration
func (s *Span) GetDuration() time.Duration {
	if s.EndedAt.IsZero() {
		return time.Since(s.StartedAt)
	}
	return s.EndedAt.Sub(s.StartedAt)
}

// IsSuccess returns true if span succeeded
func (s *Span) IsSuccess() bool {
	return s.Status == StatusOK
}

// IsError returns true if span has error
func (s *Span) IsError() bool {
	return s.Status == StatusError || s.Status == StatusBlocked
}

// ============================================================
// Trace Context Methods
// ============================================================

// AddSpan adds a span to the trace context
func (tc *TraceContext) AddSpan(span *Span) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.spans = append(tc.spans, span)
	tc.spanMap[span.ID] = span

	// Set as root if first span
	if len(tc.spans) == 1 {
		tc.root = span
	}

	// Add to parent's children if parent exists
	if span.ParentID != "" {
		if parent, ok := tc.spanMap[span.ParentID]; ok {
			parent.Children = append(parent.Children, span)
		}
	}
}

// GetSpans returns all spans in the trace
func (tc *TraceContext) GetSpans() []*Span {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.spans
}

// GetRootSpan returns the first span (root)
func (tc *TraceContext) GetRootSpan() *Span {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.root
}

// GetSpanByID returns a span by its ID
func (tc *TraceContext) GetSpanByID(id string) *Span {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.spanMap[id]
}

// TotalCost calculates total cost across all spans
func (tc *TraceContext) TotalCost() float64 {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	var total float64
	for _, s := range tc.spans {
		total += s.CostUSD
	}
	return total
}

// TotalTokens calculates total tokens across all spans
func (tc *TraceContext) TotalTokens() (int, int) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	var input, output int
	for _, s := range tc.spans {
		input += s.InputTokens
		output += s.OutputTokens
	}
	return input, output
}

// TotalLatency calculates total latency
func (tc *TraceContext) TotalLatency() int64 {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if tc.root != nil {
		return tc.root.GetDuration().Milliseconds()
	}
	var total int64
	for _, s := range tc.spans {
		total += s.LatencyMs
	}
	return total
}

// GetErrorCount returns number of error spans
func (tc *TraceContext) GetErrorCount() int {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	var count int
	for _, s := range tc.spans {
		if s.IsError() {
			count++
		}
	}
	return count
}

// GetSuccessRate returns success rate
func (tc *TraceContext) GetSuccessRate() float64 {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if len(tc.spans) == 0 {
		return 1.0
	}
	var success int
	for _, s := range tc.spans {
		if s.IsSuccess() {
			success++
		}
	}
	return float64(success) / float64(len(tc.spans))
}

// ============================================================
// Trace Context Helpers
// ============================================================

type contextKey struct{}

var traceContextKey = contextKey{}

// WithContext attaches trace context to context
func WithContext(ctx context.Context, tc *TraceContext) context.Context {
	return context.WithValue(ctx, traceContextKey, tc)
}

// FromContext retrieves trace context from context
func FromContext(ctx context.Context) *TraceContext {
	if tc, ok := ctx.Value(traceContextKey).(*TraceContext); ok {
		return tc
	}
	return nil
}

// StartTrace starts a new trace and attaches to context
func StartTrace(ctx context.Context, operation string) (context.Context, *Span) {
	tc := NewTraceContext()
	span := StartSpan(tc.TraceID, operation, LayerExperience, "")
	tc.AddSpan(span)
	return WithContext(ctx, tc), span
}

// StartChildSpan starts a child span within an existing trace
func StartChildSpan(ctx context.Context, operation string, layer SpanLayer) (context.Context, *Span) {
	tc := FromContext(ctx)
	if tc == nil {
		// No trace context, start new trace
		return StartTrace(ctx, operation)
	}

	// Get current span as parent
	currentSpan := tc.GetRootSpan()
	parentID := ""
	if currentSpan != nil {
		parentID = currentSpan.ID
	}

	span := StartSpan(tc.TraceID, operation, layer, parentID)
	tc.AddSpan(span)
	return ctx, span
}

// StartToolSpan starts a tool execution span
func StartToolSpan(ctx context.Context, toolName string) (context.Context, *Span) {
	tc := FromContext(ctx)
	if tc == nil {
		return StartTrace(ctx, fmt.Sprintf("tool:%s", toolName))
	}

	// Find latest span as parent
	parentID := ""
	if len(tc.spans) > 0 {
		parentID = tc.spans[len(tc.spans)-1].ID
	}

	span := StartSpan(tc.TraceID, fmt.Sprintf("tool:%s", toolName), LayerTool, parentID)
	span.ToolName = toolName
	tc.AddSpan(span)
	return ctx, span
}

// StartLLMSpan starts an LLM call span
func StartLLMSpan(ctx context.Context, model string) (context.Context, *Span) {
	tc := FromContext(ctx)
	if tc == nil {
		return StartTrace(ctx, fmt.Sprintf("llm:%s", model))
	}

	parentID := ""
	if len(tc.spans) > 0 {
		parentID = tc.spans[len(tc.spans)-1].ID
	}

	span := StartSpan(tc.TraceID, fmt.Sprintf("llm:%s", model), LayerLLM, parentID)
	span.Model = model
	tc.AddSpan(span)
	return ctx, span
}

// ============================================================
// Performance Profiler
// ============================================================

// ProfileResult represents performance profiling result
type ProfileResult struct {
	TraceID         string            `json:"trace_id"`
	TotalDuration   int64             `json:"total_duration_ms"`
	TotalTokens     int               `json:"total_tokens"`
	TotalCost       float64           `json:"total_cost"`
	SuccessRate     float64           `json:"success_rate"`
	Bottlenecks     []Bottleneck      `json:"bottlenecks"`
	TimeByLayer     map[SpanLayer]int64 `json:"time_by_layer"`
	TimeByOperation map[string]int64  `json:"time_by_operation"`
	CriticalPath    []string          `json:"critical_path"` // IDs of spans on critical path
	Recommendations []string          `json:"recommendations"`
	Analysis        string            `json:"analysis"`
}

// Bottleneck represents a performance bottleneck
type Bottleneck struct {
	SpanID      string  `json:"span_id"`
	Operation   string  `json:"operation"`
	Layer       SpanLayer `json:"layer"`
	Duration    int64   `json:"duration_ms"`
	PercentOfTotal float64 `json:"percent_of_total"`
	Severity    string  `json:"severity"` // low, medium, high, critical
	Cause       string  `json:"cause"`
	Suggestion  string  `json:"suggestion"`
}

// Profiler analyzes trace performance
type Profiler struct {
	bottleneckThreshold float64 // percentage threshold for bottleneck detection
	criticalThreshold   float64 // percentage threshold for critical bottlenecks
}

// NewProfiler creates a new profiler
func NewProfiler() *Profiler {
	return &Profiler{
		bottleneckThreshold: 0.1,  // 10% of total time
		criticalThreshold:   0.25, // 25% of total time
	}
}

// Profile analyzes a trace and finds bottlenecks
func (p *Profiler) Profile(tc *TraceContext) *ProfileResult {
	if tc == nil || len(tc.spans) == 0 {
		return nil
	}

	result := &ProfileResult{
		TraceID:         tc.TraceID,
		TimeByLayer:     make(map[SpanLayer]int64),
		TimeByOperation: make(map[string]int64),
		Bottlenecks:     make([]Bottleneck, 0),
		CriticalPath:    make([]string, 0),
		Recommendations: make([]string, 0),
	}

	// Calculate totals
	result.TotalDuration = tc.TotalLatency()
	inTok, outTok := tc.TotalTokens()
	result.TotalTokens = inTok + outTok
	result.TotalCost = tc.TotalCost()
	result.SuccessRate = tc.GetSuccessRate()

	// Aggregate time by layer and operation
	for _, span := range tc.spans {
		result.TimeByLayer[span.Layer] += span.LatencyMs
		result.TimeByOperation[span.Operation] += span.LatencyMs
	}

	// Find bottlenecks
	p.findBottlenecks(tc, result)

	// Calculate critical path
	p.calculateCriticalPath(tc, result)

	// Generate recommendations
	p.generateRecommendations(result)

	return result
}

func (p *Profiler) findBottlenecks(tc *TraceContext, result *ProfileResult) {
	totalDuration := result.TotalDuration
	if totalDuration == 0 {
		return
	}

	for _, span := range tc.spans {
		if span.LatencyMs == 0 {
			continue
		}

		percentOfTotal := float64(span.LatencyMs) / float64(totalDuration)

		if percentOfTotal < p.bottleneckThreshold {
			continue
		}

		severity := "low"
		if percentOfTotal >= p.criticalThreshold {
			severity = "critical"
		} else if percentOfTotal >= 0.2 {
			severity = "high"
		} else if percentOfTotal >= 0.15 {
			severity = "medium"
		}

		bottleneck := Bottleneck{
			SpanID:         span.ID,
			Operation:      span.Operation,
			Layer:          span.Layer,
			Duration:       span.LatencyMs,
			PercentOfTotal: percentOfTotal,
			Severity:       severity,
			Cause:          p.identifyCause(span),
			Suggestion:     p.suggestFix(span),
		}

		result.Bottlenecks = append(result.Bottlenecks, bottleneck)
	}

	// Sort by severity and duration
	sort.Slice(result.Bottlenecks, func(i, j int) bool {
		if result.Bottlenecks[i].Severity != result.Bottlenecks[j].Severity {
			severityOrder := map[string]int{"critical": 4, "high": 3, "medium": 2, "low": 1}
			return severityOrder[result.Bottlenecks[i].Severity] > severityOrder[result.Bottlenecks[j].Severity]
		}
		return result.Bottlenecks[i].Duration > result.Bottlenecks[j].Duration
	})
}

func (p *Profiler) identifyCause(span *Span) string {
	switch span.Layer {
	case LayerLLM:
		if span.InputTokens > 4000 {
			return "Large input context causing slow LLM response"
		}
		if span.RetryCount > 0 {
			return fmt.Sprintf("LLM call required %d retries", span.RetryCount)
		}
		return "Slow LLM response time"
	case LayerTool:
		if span.IsError() {
			return fmt.Sprintf("Tool error: %s", span.ErrorMsg)
		}
		if span.RetryCount > 0 {
			return fmt.Sprintf("Tool call required %d retries", span.RetryCount)
		}
		return "Slow tool execution"
	case LayerDecision:
		return "Complex routing/planning decision"
	default:
		return "Unknown cause"
	}
}

func (p *Profiler) suggestFix(span *Span) string {
	switch span.Layer {
	case LayerLLM:
		if span.InputTokens > 4000 {
			return "Consider reducing input context or using context compression"
		}
		if !span.CacheHit {
			return "Enable prompt caching for repeated requests"
		}
		return "Consider using a faster model for simple tasks"
	case LayerTool:
		if span.IsError() {
			return "Add error handling and fallback strategies"
		}
		return "Optimize tool implementation or parallelize independent calls"
	case LayerDecision:
		return "Simplify routing logic or pre-compute decisions"
	default:
		return "Review and optimize this operation"
	}
}

func (p *Profiler) calculateCriticalPath(tc *TraceContext, result *ProfileResult) {
	// Find the path with longest cumulative latency
	// Start from root and traverse children
	root := tc.GetRootSpan()
	if root == nil {
		return
	}

	p.traverseCriticalPath(root, result.CriticalPath)
}

func (p *Profiler) traverseCriticalPath(span *Span, path []string) []string {
	path = append(path, span.ID)

	if len(span.Children) == 0 {
		return path
	}

	// Find child with longest duration
	var longestChild *Span
	var maxDuration int64
	for _, child := range span.Children {
		if child.LatencyMs > maxDuration {
			maxDuration = child.LatencyMs
			longestChild = child
		}
	}

	if longestChild != nil {
		return p.traverseCriticalPath(longestChild, path)
	}

	return path
}

func (p *Profiler) generateRecommendations(result *ProfileResult) {
	// Recommendations based on bottlenecks
	for _, b := range result.Bottlenecks {
		if b.Suggestion != "" {
			result.Recommendations = append(result.Recommendations, b.Suggestion)
		}
	}

	// Recommendations based on success rate
	if result.SuccessRate < 0.8 {
		result.Recommendations = append(result.Recommendations,
			"Success rate is low, review error handling and retry strategies")
	}

	// Recommendations based on layer distribution
	llmTime := result.TimeByLayer[LayerLLM]
	totalTime := result.TotalDuration
	if totalTime > 0 && float64(llmTime)/float64(totalTime) > 0.7 {
		result.Recommendations = append(result.Recommendations,
			"LLM calls dominate execution time, consider caching or faster models")
	}

	toolTime := result.TimeByLayer[LayerTool]
	if totalTime > 0 && float64(toolTime)/float64(totalTime) > 0.5 {
		result.Recommendations = append(result.Recommendations,
			"Tool calls are slow, consider parallelizing independent operations")
	}

	// Generate analysis summary
	p.generateAnalysis(result)
}

func (p *Profiler) generateAnalysis(result *ProfileResult) {
	var analysis strings.Builder

	analysis.WriteString(fmt.Sprintf("Total execution took %d ms with %d tokens and cost %.4f.\n",
		result.TotalDuration, result.TotalTokens, result.TotalCost))

	if len(result.Bottlenecks) > 0 {
		analysis.WriteString(fmt.Sprintf("Found %d bottlenecks: ", len(result.Bottlenecks)))
		for i, b := range result.Bottlenecks {
			if i < 3 {
				analysis.WriteString(fmt.Sprintf("%s (%d ms, %s), ", b.Operation, b.Duration, b.Severity))
			}
		}
		analysis.WriteString("\n")
	} else {
		analysis.WriteString("No significant bottlenecks detected.\n")
	}

	if result.SuccessRate < 1.0 {
		analysis.WriteString(fmt.Sprintf("Success rate: %.1f%%\n", result.SuccessRate*100))
	}

	result.Analysis = analysis.String()
}

// ============================================================
// Trace Recorder
// ============================================================

// TraceRecorder records and persists traces
type TraceRecorder struct {
	traces   map[string]*TraceContext
	profiles map[string]*ProfileResult
	profiler *Profiler
	mu       sync.RWMutex
	maxTraces int // Maximum traces to keep in memory
}

// NewTraceRecorder creates a new trace recorder
func NewTraceRecorder(maxTraces int) *TraceRecorder {
	if maxTraces <= 0 {
		maxTraces = 1000
	}
	return &TraceRecorder{
		traces:    make(map[string]*TraceContext),
		profiles:  make(map[string]*ProfileResult),
		profiler:  NewProfiler(),
		maxTraces: maxTraces,
	}
}

// Record records a trace context
func (r *TraceRecorder) Record(tc *TraceContext) {
	if tc == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check capacity
	if len(r.traces) >= r.maxTraces {
		// Remove oldest trace (simple FIFO, could be improved with LRU)
		var oldestID string
		var oldestTime time.Time
		for id, ctx := range r.traces {
			root := ctx.GetRootSpan()
			if root != nil && (oldestTime.IsZero() || root.StartedAt.Before(oldestTime)) {
				oldestID = id
				oldestTime = root.StartedAt
			}
		}
		if oldestID != "" {
			delete(r.traces, oldestID)
			delete(r.profiles, oldestID)
		}
	}

	r.traces[tc.TraceID] = tc

	// Profile the trace
	profile := r.profiler.Profile(tc)
	if profile != nil {
		r.profiles[tc.TraceID] = profile
	}
}

// GetTrace retrieves a trace by ID
func (r *TraceRecorder) GetTrace(traceID string) *TraceContext {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.traces[traceID]
}

// GetProfile retrieves a profile by trace ID
func (r *TraceRecorder) GetProfile(traceID string) *ProfileResult {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.profiles[traceID]
}

// ListTraces lists all trace IDs
func (r *TraceRecorder) ListTraces() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.traces))
	for id := range r.traces {
		ids = append(ids, id)
	}
	return ids
}

// GetRecentTraces returns the most recent traces
func (r *TraceRecorder) GetRecentTraces(limit int) []*TraceContext {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Sort by start time
	traces := make([]*TraceContext, 0, len(r.traces))
	for _, tc := range r.traces {
		traces = append(traces, tc)
	}

	sort.Slice(traces, func(i, j int) bool {
		rootI := traces[i].GetRootSpan()
		rootJ := traces[j].GetRootSpan()
		if rootI == nil || rootJ == nil {
			return false
		}
		return rootI.StartedAt.After(rootJ.StartedAt)
	})

	if limit > 0 && len(traces) > limit {
		traces = traces[:limit]
	}

	return traces
}

// GetTracesByStatus returns traces filtered by status
func (r *TraceRecorder) GetTracesByStatus(status SpanStatus) []*TraceContext {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []*TraceContext
	for _, tc := range r.traces {
		root := tc.GetRootSpan()
		if root != nil && root.Status == status {
			filtered = append(filtered, tc)
		}
	}
	return filtered
}

// GetTracesWithError returns all traces with errors
func (r *TraceRecorder) GetTracesWithError() []*TraceContext {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []*TraceContext
	for _, tc := range r.traces {
		if tc.GetErrorCount() > 0 {
			filtered = append(filtered, tc)
		}
	}
	return filtered
}

// GetSlowTraces returns traces above a latency threshold
func (r *TraceRecorder) GetSlowTraces(thresholdMs int64) []*TraceContext {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []*TraceContext
	for _, tc := range r.traces {
		if tc.TotalLatency() > thresholdMs {
			filtered = append(filtered, tc)
		}
	}
	return filtered
}

// GetExpensiveTraces returns traces above a cost threshold
func (r *TraceRecorder) GetExpensiveTraces(thresholdCost float64) []*TraceContext {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []*TraceContext
	for _, tc := range r.traces {
		if tc.TotalCost() > thresholdCost {
			filtered = append(filtered, tc)
		}
	}
	return filtered
}

// GetStatistics returns aggregated statistics
func (r *TraceRecorder) GetStatistics() *TraceStatistics {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := &TraceStatistics{
		TotalTraces:   len(r.traces),
		ByStatus:      make(map[SpanStatus]int),
		ByLayer:       make(map[SpanLayer]int),
		AvgLatency:    0,
		AvgTokens:     0,
		AvgCost:       0,
		AvgSuccessRate: 0,
		TopBottlenecks: make([]Bottleneck, 0),
	}

	var totalLatency int64
	var totalTokens int
	var totalCost float64
	var totalSuccessRate float64

	bottleneckCounts := make(map[string]int)

	for _, tc := range r.traces {
		root := tc.GetRootSpan()
		if root != nil {
			stats.ByStatus[root.Status]++
		}

		totalLatency += tc.TotalLatency()
		inTok, outTok := tc.TotalTokens()
		totalTokens += inTok + outTok
		totalCost += tc.TotalCost()
		totalSuccessRate += tc.GetSuccessRate()

		for _, span := range tc.GetSpans() {
			stats.ByLayer[span.Layer]++
		}

		// Aggregate bottlenecks
		profile := r.profiles[tc.TraceID]
		if profile != nil {
			for _, b := range profile.Bottlenecks {
				bottleneckCounts[b.Operation]++
			}
		}
	}

	if stats.TotalTraces > 0 {
		stats.AvgLatency = float64(totalLatency) / float64(stats.TotalTraces)
		stats.AvgTokens = float64(totalTokens) / float64(stats.TotalTraces)
		stats.AvgCost = totalCost / float64(stats.TotalTraces)
		stats.AvgSuccessRate = totalSuccessRate / float64(stats.TotalTraces)
	}

	// Find top bottlenecks
	for op, count := range bottleneckCounts {
		if count >= 3 {
			stats.TopBottlenecks = append(stats.TopBottlenecks, Bottleneck{
				Operation: op,
				Severity:  "recurring",
			})
		}
	}

	return stats
}

// TraceStatistics represents aggregated trace statistics
type TraceStatistics struct {
	TotalTraces    int                `json:"total_traces"`
	ByStatus       map[SpanStatus]int `json:"by_status"`
	ByLayer        map[SpanLayer]int  `json:"by_layer"`
	AvgLatency     float64            `json:"avg_latency_ms"`
	AvgTokens      float64            `json:"avg_tokens"`
	AvgCost        float64            `json:"avg_cost"`
	AvgSuccessRate float64            `json:"avg_success_rate"`
	TopBottlenecks []Bottleneck       `json:"top_bottlenecks"`
}

// ============================================================
// Database Models
// ============================================================

// SpanModel is the database model for spans
type SpanModel struct {
	ID           string    `gorm:"primaryKey"`
	TraceID      string    `gorm:"index;not null"`
	ParentID     string    `gorm:"index"`
	Layer        int       `gorm:"not null"`
	Operation    string    `gorm:"not null"`
	AgentType    string    `gorm:"index"`
	Model        string
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	LatencyMs    int64
	Status       string    `gorm:"index;not null"`
	ErrorMsg     string
	Metadata     string    `gorm:"type:text"`
	StartedAt    time.Time `gorm:"index"`
	EndedAt      time.Time
	CreatedAt    time.Time `gorm:"index"`
	ToolName     string    `gorm:"index"`
	ToolArgs     string    `gorm:"type:text"`
	ToolResult   string    `gorm:"type:text"`
	LLMRequest   string    `gorm:"type:text"`
	LLMResponse  string    `gorm:"type:text"`
	RetryCount   int
	CacheHit     bool
}

// ToModel converts Span to SpanModel
func (s *Span) ToModel() *SpanModel {
	metaStr := ""
	if s.Metadata != nil {
		if data, err := json.Marshal(s.Metadata); err == nil {
			metaStr = string(data)
		}
	}

	attrStr := ""
	if s.Attributes != nil {
		if data, err := json.Marshal(s.Attributes); err == nil {
			attrStr = string(data)
		}
	}

	eventsStr := ""
	if s.Events != nil {
		if data, err := json.Marshal(s.Events); err == nil {
			eventsStr = string(data)
		}
	}

	return &SpanModel{
		ID:           s.ID,
		TraceID:      s.TraceID,
		ParentID:     s.ParentID,
		Layer:        int(s.Layer),
		Operation:    s.Operation,
		AgentType:    s.AgentType,
		Model:        s.Model,
		InputTokens:  s.InputTokens,
		OutputTokens: s.OutputTokens,
		CostUSD:      s.CostUSD,
		LatencyMs:    s.LatencyMs,
		Status:       string(s.Status),
		ErrorMsg:     s.ErrorMsg,
		Metadata:     metaStr + "|" + attrStr + "|" + eventsStr,
		StartedAt:    s.StartedAt,
		EndedAt:      s.EndedAt,
		CreatedAt:    time.Now(),
		ToolName:     s.ToolName,
		ToolArgs:     s.ToolArgs,
		ToolResult:   s.ToolResult,
		LLMRequest:   s.LLMRequest,
		LLMResponse:  s.LLMResponse,
		RetryCount:   s.RetryCount,
		CacheHit:     s.CacheHit,
	}
}

// FromModel converts SpanModel to Span
func FromModel(m *SpanModel) *Span {
	s := &Span{
		ID:           m.ID,
		TraceID:      m.TraceID,
		ParentID:     m.ParentID,
		Layer:        SpanLayer(m.Layer),
		Operation:    m.Operation,
		AgentType:    m.AgentType,
		Model:        m.Model,
		InputTokens:  m.InputTokens,
		OutputTokens: m.OutputTokens,
		CostUSD:      m.CostUSD,
		LatencyMs:    m.LatencyMs,
		Status:       SpanStatus(m.Status),
		ErrorMsg:     m.ErrorMsg,
		StartedAt:    m.StartedAt,
		EndedAt:      m.EndedAt,
		ToolName:     m.ToolName,
		ToolArgs:     m.ToolArgs,
		ToolResult:   m.ToolResult,
		LLMRequest:   m.LLMRequest,
		LLMResponse:  m.LLMResponse,
		RetryCount:   m.RetryCount,
		CacheHit:     m.CacheHit,
		Metadata:     make(map[string]interface{}),
		Attributes:   make(map[string]string),
		Events:       make([]SpanEvent, 0),
		Children:     make([]*Span, 0),
	}

	// Parse metadata if available
	if m.Metadata != "" {
		parts := strings.Split(m.Metadata, "|")
		if len(parts) >= 1 && parts[0] != "" {
			json.Unmarshal([]byte(parts[0]), &s.Metadata)
		}
		if len(parts) >= 2 && parts[1] != "" {
			json.Unmarshal([]byte(parts[1]), &s.Attributes)
		}
		if len(parts) >= 3 && parts[2] != "" {
			json.Unmarshal([]byte(parts[2]), &s.Events)
		}
	}

	return s
}

// TraceModel is the database model for traces
type TraceModel struct {
	ID           string    `gorm:"primaryKey"`
	TotalCost    float64
	InputTokens  int
	OutputTokens int
	LatencyMs    int64
	Status       string    `gorm:"index"`
	CreatedAt    time.Time `gorm:"index"`
	Operation    string    `gorm:"index"`
	AgentType    string    `gorm:"index"`
}

// TraceCollector collects and persists spans
type TraceCollector struct {
	ch     chan *Span
	done   chan struct{}
	wg     sync.WaitGroup
	start  sync.Once
	stop   sync.Once
	recorder *TraceRecorder
}

// NewTraceCollector creates a new trace collector
func NewTraceCollector() *TraceCollector {
	return &TraceCollector{
		ch:       make(chan *Span, 10000),
		done:     make(chan struct{}),
		recorder: NewTraceRecorder(1000),
	}
}

// Record records a span to the collector
func (c *TraceCollector) Record(span *Span) {
	select {
	case c.ch <- span:
	default:
		// Channel full, drop span
	}
}

// Start starts the collector processing
func (c *TraceCollector) Start() {
	c.start.Do(func() {
		c.wg.Add(1)
		go c.process()
	})
}

// process processes incoming spans
func (c *TraceCollector) process() {
	defer c.wg.Done()

	// Group spans by trace ID
	traceSpans := make(map[string][]*Span)

	for {
		select {
		case span := <-c.ch:
			traceSpans[span.TraceID] = append(traceSpans[span.TraceID], span)

			// Check if trace is complete (root span ended)
			if span.ParentID == "" && !span.EndedAt.IsZero() {
				// Build trace context and record
				tc := NewTraceContext()
				tc.TraceID = span.TraceID
				for _, s := range traceSpans[span.TraceID] {
					tc.AddSpan(s)
				}
				c.recorder.Record(tc)
				delete(traceSpans, span.TraceID)
			}

		case <-c.done:
			// Process remaining spans
			return
		}
	}
}

// Stop stops the collector
func (c *TraceCollector) Stop() {
	c.stop.Do(func() {
		close(c.done)
		c.wg.Wait()
	})
}

// GetRecorder returns the trace recorder
func (c *TraceCollector) GetRecorder() *TraceRecorder {
	return c.recorder
}