// Package observability provides metrics and tracing utilities
package observability

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ============================================================
// Metrics Collector
// ============================================================

// MetricsCollector collects and aggregates metrics
type MetricsCollector struct {
	registry    *MetricsRegistry
	counters    map[string]int64
	floatValues map[string]float64
	histograms  map[string][]float64
	timestamps  map[string]time.Time
	mu          sync.RWMutex
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(manager *OTelManager) *MetricsCollector {
	return &MetricsCollector{
		registry:    manager.NewMetricsRegistry("collector"),
		counters:    make(map[string]int64),
		floatValues: make(map[string]float64),
		histograms:  make(map[string][]float64),
		timestamps:  make(map[string]time.Time),
	}
}

// Increment increments a counter
func (c *MetricsCollector) Increment(name string, value int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.counters[name] += value
	c.timestamps[name] = time.Now()
}

// SetFloat sets a float value
func (c *MetricsCollector) SetFloat(name string, value float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.floatValues[name] = value
	c.timestamps[name] = time.Now()
}

// Record records a value for histogram
func (c *MetricsCollector) Record(name string, value float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.histograms[name] = append(c.histograms[name], value)
	c.timestamps[name] = time.Now()
}

// GetCounter gets a counter value
func (c *MetricsCollector) GetCounter(name string) int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.counters[name]
}

// GetFloat gets a float value
func (c *MetricsCollector) GetFloat(name string) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.floatValues[name]
}

// GetHistogramStats gets histogram statistics
func (c *MetricsCollector) GetHistogramStats(name string) (min, max, avg float64, count int) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	values := c.histograms[name]
	if len(values) == 0 {
		return 0, 0, 0, 0
	}

	min = values[0]
	max = values[0]
	var sum float64

	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += v
	}

	return min, max, sum / float64(len(values)), len(values)
}

// Export exports all metrics to OpenTelemetry
func (c *MetricsCollector) Export(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Export counters
	for name, value := range c.counters {
		c.registry.AddCounter(ctx, name, value)
	}

	// Export histograms
	for name, values := range c.histograms {
		for _, v := range values {
			c.registry.RecordHistogram(ctx, name, v)
		}
	}

	return nil
}

// Reset resets all metrics
func (c *MetricsCollector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.counters = make(map[string]int64)
	c.floatValues = make(map[string]float64)
	c.histograms = make(map[string][]float64)
	c.timestamps = make(map[string]time.Time)
}

// ============================================================
// System Metrics
// ============================================================

// SystemMetrics collects system-level metrics
type SystemMetrics struct {
	registry          *MetricsRegistry
	collectInterval   time.Duration
	stopChan          chan struct{}
	wg                sync.WaitGroup
}

// NewSystemMetrics creates system metrics collector
func NewSystemMetrics(manager *OTelManager) *SystemMetrics {
	registry := manager.NewMetricsRegistry("system")

	// Register metrics
	registry.RegisterUpDownCounter("system.goroutines", "Number of goroutines", "1")
	registry.RegisterHistogram("system.memory.alloc", "Memory allocation", "bytes")
	registry.RegisterHistogram("system.memory.heap", "Heap size", "bytes")
	registry.RegisterHistogram("system.memory.stack", "Stack size", "bytes")
	registry.RegisterUpDownCounter("system.gc.count", "GC count", "1")
	registry.RegisterHistogram("system.gc.pause", "GC pause duration", "ns")

	return &SystemMetrics{
		registry:        registry,
		collectInterval: 10 * time.Second,
		stopChan:        make(chan struct{}),
	}
}

// Start starts collecting system metrics
func (s *SystemMetrics) Start() {
	s.wg.Add(1)
	go s.collectLoop()
}

// Stop stops collecting
func (s *SystemMetrics) Stop() {
	close(s.stopChan)
	s.wg.Wait()
}

// collectLoop periodically collects metrics
func (s *SystemMetrics) collectLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.collectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.collect()
		}
	}
}

// collect collects current system metrics
func (s *SystemMetrics) collect() {
	ctx := context.Background()

	// Goroutines
	s.registry.AddUpDownCounter(ctx, "system.goroutines", int64(runtime.NumGoroutine()))

	// Memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(m)

	s.registry.RecordHistogram(ctx, "system.memory.alloc", float64(m.Alloc))
	s.registry.RecordHistogram(ctx, "system.memory.heap", float64(m.HeapAlloc))
	s.registry.RecordHistogram(ctx, "system.memory.stack", float64(m.StackInuse))

	// GC
	s.registry.AddUpDownCounter(ctx, "system.gc.count", int64(m.NumGC))
	s.registry.RecordHistogram(ctx, "system.gc.pause", float64(m.PauseTotalNs))
}

// ============================================================
// HTTP Metrics Middleware
// ============================================================

// HTTPMetrics provides HTTP request metrics
type HTTPMetrics struct {
	registry *MetricsRegistry
}

// NewHTTPMetrics creates HTTP metrics
func NewHTTPMetrics(manager *OTelManager) *HTTPMetrics {
	registry := manager.NewMetricsRegistry("http")

	registry.RegisterCounter("http.requests.total", "Total HTTP requests", "1")
	registry.RegisterCounter("http.requests.errors", "HTTP request errors", "1")
	registry.RegisterHistogram("http.request.duration", "HTTP request duration", "ms")
	registry.RegisterHistogram("http.request.size", "HTTP request size", "bytes")
	registry.RegisterHistogram("http.response.size", "HTTP response size", "bytes")

	return &HTTPMetrics{registry: registry}
}

// RecordRequest records an HTTP request
func (h *HTTPMetrics) RecordRequest(ctx context.Context, method, path string, statusCode int, durationMs float64, attrs ...attribute.KeyValue) {
	baseAttrs := []attribute.KeyValue{
		attribute.String("http.method", method),
		attribute.String("http.path", path),
		attribute.Int("http.status_code", statusCode),
	}
	baseAttrs = append(baseAttrs, attrs...)

	h.registry.IncrementCounter(ctx, "http.requests.total", baseAttrs...)

	if statusCode >= 400 {
		h.registry.IncrementCounter(ctx, "http.requests.errors", baseAttrs...)
	}

	h.registry.RecordHistogram(ctx, "http.request.duration", durationMs, baseAttrs...)
}

// ============================================================
// gRPC Metrics
// ============================================================

// GRPCMetrics provides gRPC request metrics
type GRPCMetrics struct {
	registry *MetricsRegistry
}

// NewGRPCMetrics creates gRPC metrics
func NewGRPCMetrics(manager *OTelManager) *GRPCMetrics {
	registry := manager.NewMetricsRegistry("grpc")

	registry.RegisterCounter("grpc.requests.total", "Total gRPC requests", "1")
	registry.RegisterCounter("grpc.requests.errors", "gRPC request errors", "1")
	registry.RegisterHistogram("grpc.request.duration", "gRPC request duration", "ms")
	registry.RegisterCounter("grpc.messages.sent", "gRPC messages sent", "1")
	registry.RegisterCounter("grpc.messages.received", "gRPC messages received", "1")

	return &GRPCMetrics{registry: registry}
}

// RecordRequest records a gRPC request
func (g *GRPCMetrics) RecordRequest(ctx context.Context, service, method string, success bool, durationMs float64, attrs ...attribute.KeyValue) {
	baseAttrs := []attribute.KeyValue{
		attribute.String("grpc.service", service),
		attribute.String("grpc.method", method),
	}
	baseAttrs = append(baseAttrs, attrs...)

	g.registry.IncrementCounter(ctx, "grpc.requests.total", baseAttrs...)

	if !success {
		g.registry.IncrementCounter(ctx, "grpc.requests.errors", baseAttrs...)
	}

	g.registry.RecordHistogram(ctx, "grpc.request.duration", durationMs, baseAttrs...)
}

// RecordMessages records message counts
func (g *GRPCMetrics) RecordMessages(ctx context.Context, service, method string, sent, received int64, attrs ...attribute.KeyValue) {
	baseAttrs := []attribute.KeyValue{
		attribute.String("grpc.service", service),
		attribute.String("grpc.method", method),
	}
	baseAttrs = append(baseAttrs, attrs...)

	if sent > 0 {
		g.registry.AddCounter(ctx, "grpc.messages.sent", sent, baseAttrs...)
	}
	if received > 0 {
		g.registry.AddCounter(ctx, "grpc.messages.received", received, baseAttrs...)
	}
}

// ============================================================
// Business Metrics
// ============================================================

// BusinessMetrics provides business-level metrics
type BusinessMetrics struct {
	registry *MetricsRegistry
}

// NewBusinessMetrics creates business metrics
func NewBusinessMetrics(manager *OTelManager) *BusinessMetrics {
	registry := manager.NewMetricsRegistry("business")

	// Agent metrics
	registry.RegisterCounter("agent.sessions.total", "Total agent sessions", "1")
	registry.RegisterCounter("agent.sessions.active", "Active agent sessions", "1")
	registry.RegisterHistogram("agent.session.duration", "Agent session duration", "ms")
	registry.RegisterCounter("agent.messages.total", "Total agent messages", "1")

	// Memory metrics
	registry.RegisterCounter("memory.operations.total", "Total memory operations", "1")
	registry.RegisterCounter("memory.stored.total", "Total memories stored", "1")
	registry.RegisterCounter("memory.recalled.total", "Total memories recalled", "1")
	registry.RegisterHistogram("memory.operation.latency", "Memory operation latency", "ms")

	// Tool metrics
	registry.RegisterCounter("tools.invocations.total", "Total tool invocations", "1")
	registry.RegisterCounter("tools.invocations.errors", "Tool invocation errors", "1")
	registry.RegisterHistogram("tools.invocation.latency", "Tool invocation latency", "ms")

	// Evaluation metrics
	registry.RegisterCounter("eval.runs.total", "Total evaluation runs", "1")
	registry.RegisterCounter("eval.tests.passed", "Passed tests", "1")
	registry.RegisterCounter("eval.tests.failed", "Failed tests", "1")
	registry.RegisterHistogram("eval.score", "Evaluation scores", "1")

	return &BusinessMetrics{registry: registry}
}

// RecordAgentSession records agent session metrics
func (b *BusinessMetrics) RecordAgentSession(ctx context.Context, agentID string, created bool, durationMs float64, attrs ...attribute.KeyValue) {
	baseAttrs := []attribute.KeyValue{
		attribute.String("agent.id", agentID),
	}
	baseAttrs = append(baseAttrs, attrs...)

	if created {
		b.registry.IncrementCounter(ctx, "agent.sessions.total", baseAttrs...)
		b.registry.IncrementCounter(ctx, "agent.sessions.active", baseAttrs...)
	}

	if durationMs > 0 {
		b.registry.RecordHistogram(ctx, "agent.session.duration", durationMs, baseAttrs...)
	}
}

// RecordMemoryOperation records memory operation metrics
func (b *BusinessMetrics) RecordMemoryOperation(ctx context.Context, operation string, latencyMs float64, attrs ...attribute.KeyValue) {
	baseAttrs := []attribute.KeyValue{
		attribute.String("memory.operation", operation),
	}
	baseAttrs = append(baseAttrs, attrs...)

	b.registry.IncrementCounter(ctx, "memory.operations.total", baseAttrs...)

	if operation == "store" {
		b.registry.IncrementCounter(ctx, "memory.stored.total", baseAttrs...)
	} else if operation == "recall" {
		b.registry.IncrementCounter(ctx, "memory.recalled.total", baseAttrs...)
	}

	b.registry.RecordHistogram(ctx, "memory.operation.latency", latencyMs, baseAttrs...)
}

// RecordToolInvocation records tool invocation metrics
func (b *BusinessMetrics) RecordToolInvocation(ctx context.Context, toolName string, success bool, latencyMs float64, attrs ...attribute.KeyValue) {
	baseAttrs := []attribute.KeyValue{
		attribute.String("tool.name", toolName),
	}
	baseAttrs = append(baseAttrs, attrs...)

	b.registry.IncrementCounter(ctx, "tools.invocations.total", baseAttrs...)

	if !success {
		b.registry.IncrementCounter(ctx, "tools.invocations.errors", baseAttrs...)
	}

	b.registry.RecordHistogram(ctx, "tools.invocation.latency", latencyMs, baseAttrs...)
}

// RecordEvalRun records evaluation run metrics
func (b *BusinessMetrics) RecordEvalRun(ctx context.Context, suiteID string, passed, failed int, avgScore float64, attrs ...attribute.KeyValue) {
	baseAttrs := []attribute.KeyValue{
		attribute.String("eval.suite_id", suiteID),
	}
	baseAttrs = append(baseAttrs, attrs...)

	b.registry.IncrementCounter(ctx, "eval.runs.total", baseAttrs...)
	b.registry.AddCounter(ctx, "eval.tests.passed", int64(passed), baseAttrs...)
	b.registry.AddCounter(ctx, "eval.tests.failed", int64(failed), baseAttrs...)
	b.registry.RecordHistogram(ctx, "eval.score", avgScore, baseAttrs...)
}

// ============================================================
// Metrics Aggregator
// ============================================================

// MetricsAggregator aggregates metrics from multiple sources
type MetricsAggregator struct {
	collectors []*MetricsCollector
	mu         sync.RWMutex
}

// NewMetricsAggregator creates a new metrics aggregator
func NewMetricsAggregator() *MetricsAggregator {
	return &MetricsAggregator{
		collectors: make([]*MetricsCollector, 0),
	}
}

// AddCollector adds a collector
func (a *MetricsAggregator) AddCollector(collector *MetricsCollector) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.collectors = append(a.collectors, collector)
}

// ExportAll exports all collectors
func (a *MetricsAggregator) ExportAll(ctx context.Context) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, collector := range a.collectors {
		if err := collector.Export(ctx); err != nil {
			return err
		}
	}

	return nil
}

// ResetAll resets all collectors
func (a *MetricsAggregator) ResetAll() {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, collector := range a.collectors {
		collector.Reset()
	}
}

// GetSummary gets a summary of all metrics
func (a *MetricsAggregator) GetSummary() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	summary := make(map[string]interface{})

	for i, collector := range a.collectors {
		collector.mu.RLock()

		summary[fmt.Sprintf("collector_%d", i)] = map[string]interface{}{
			"counters":     collector.counters,
			"float_values": collector.floatValues,
			"histograms":   collector.histograms,
		}

		collector.mu.RUnlock()
	}

	return summary
}
