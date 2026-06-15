// Package observability provides OpenTelemetry integration for the agent platform
package observability

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric/meterprovider"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// ============================================================
// OpenTelemetry Configuration
// ============================================================

// OTelConfig represents OpenTelemetry configuration
type OTelConfig struct {
	// Service identification
	ServiceName    string `json:"service_name"`
	ServiceVersion string `json:"service_version"`
	Environment    string `json:"environment"`

	// Trace configuration
	TraceEnabled       bool   `json:"trace_enabled"`
	TraceExporterType  string `json:"trace_exporter_type"` // "otlp-http", "otlp-grpc", "stdout"
	TraceEndpoint      string `json:"trace_endpoint"`
	TraceSampleRate    float64 `json:"trace_sample_rate"`
	TraceBatchTimeout  int    `json:"trace_batch_timeout_ms"`

	// Metric configuration
	MetricsEnabled      bool   `json:"metrics_enabled"`
	MetricsExporterType string `json:"metrics_exporter_type"` // "prometheus", "otlp"
	MetricsEndpoint     string `json:"metrics_endpoint"`
	MetricsPort         int    `json:"metrics_port"`

	// Resource attributes
	ResourceAttributes map[string]string `json:"resource_attributes"`
}

// DefaultOTelConfig returns default configuration
func DefaultOTelConfig(serviceName string) *OTelConfig {
	return &OTelConfig{
		ServiceName:        serviceName,
		ServiceVersion:     "1.0.0",
		Environment:        "production",
		TraceEnabled:       true,
		TraceExporterType:  "stdout",
		TraceSampleRate:    1.0,
		TraceBatchTimeout:  5000,
		MetricsEnabled:     true,
		MetricsExporterType: "prometheus",
		MetricsPort:        9090,
		ResourceAttributes: make(map[string]string),
	}
}

// ============================================================
// OpenTelemetry Manager
// ============================================================

// OTelManager manages OpenTelemetry components
type OTelManager struct {
	config           *OTelConfig
	tracerProvider   *trace.TracerProvider
	meterProvider    metric.MeterProvider
	exporter         trace.SpanExporter
	prometheusServer *http.Server
	resource         *resource.Resource
	tracers          map[string]trace.Tracer
	meters           map[string]metric.Meter
	mu               sync.RWMutex
	shutdownFuncs    []func(context.Context) error
}

// NewOTelManager creates a new OpenTelemetry manager
func NewOTelManager(config *OTelConfig) *OTelManager {
	if config == nil {
		config = DefaultOTelConfig("agent-platform")
	}
	return &OTelManager{
		config:  config,
		tracers: make(map[string]trace.Tracer),
		meters:  make(map[string]metric.Meter),
	}
}

// Initialize initializes OpenTelemetry components
func (m *OTelManager) Initialize(ctx context.Context) error {
	// Create resource
	res, err := m.createResource(ctx)
	if err != nil {
		return fmt.Errorf("create resource: %w", err)
	}
	m.resource = res

	// Initialize tracing
	if m.config.TraceEnabled {
		if err := m.initTracing(ctx, res); err != nil {
			return fmt.Errorf("init tracing: %w", err)
		}
	}

	// Initialize metrics
	if m.config.MetricsEnabled {
		if err := m.initMetrics(ctx, res); err != nil {
			return fmt.Errorf("init metrics: %w", err)
		}
	}

	// Set global providers
	if m.tracerProvider != nil {
		otel.SetTracerProvider(m.tracerProvider)
	}

	// Set text map propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return nil
}

// createResource creates the OpenTelemetry resource
func (m *OTelManager) createResource(ctx context.Context) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		semconv.ServiceName(m.config.ServiceName),
		semconv.ServiceVersion(m.config.ServiceVersion),
		semconv.DeploymentEnvironment(m.config.Environment),
	}

	// Add custom attributes
	for k, v := range m.config.ResourceAttributes {
		attrs = append(attrs, attribute.String(k, v))
	}

	return resource.New(ctx,
		resource.WithAttributes(attrs...),
		resource.WithHost(),
		resource.WithOS(),
		resource.WithProcess(),
	)
}

// initTracing initializes trace provider
func (m *OTelManager) initTracing(ctx context.Context, res *resource.Resource) error {
	// Create exporter based on type
	var exporter trace.SpanExporter
	var err error

	switch m.config.TraceExporterType {
	case "otlp-http":
		exporter, err = otlptracehttp.New(ctx,
			otlptracehttp.WithEndpoint(m.config.TraceEndpoint),
			otlptracehttp.WithInsecure(),
		)
	case "otlp-grpc":
		exporter, err = otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(m.config.TraceEndpoint),
			otlptracegrpc.WithInsecure(),
		)
	case "stdout":
		exporter, err = otlptrace.New(ctx, &stdoutExporter{})
	default:
		exporter, err = otlptrace.New(ctx, &stdoutExporter{})
	}

	if err != nil {
		return fmt.Errorf("create trace exporter: %w", err)
	}

	m.exporter = exporter

	// Create sampler
	sampler := trace.ParentBased(trace.TraceIDRatioBased(m.config.TraceSampleRate))

	// Create trace provider
	m.tracerProvider = trace.NewTracerProvider(
		trace.WithResource(res),
		trace.WithBatcher(exporter,
			trace.WithBatchTimeout(time.Duration(m.config.TraceBatchTimeout)*time.Millisecond),
		),
		trace.WithSampler(sampler),
	)

	// Add shutdown function
	m.shutdownFuncs = append(m.shutdownFuncs, m.tracerProvider.Shutdown)

	return nil
}

// initMetrics initializes metric provider
func (m *OTelManager) initMetrics(ctx context.Context, res *resource.Resource) error {
	switch m.config.MetricsExporterType {
	case "prometheus":
		return m.initPrometheusMetrics(ctx, res)
	case "otlp":
		return m.initOTLPMetrics(ctx, res)
	default:
		return m.initPrometheusMetrics(ctx, res)
	}
}

// initPrometheusMetrics initializes Prometheus exporter
func (m *OTelManager) initPrometheusMetrics(ctx context.Context, res *resource.Resource) error {
	// Create Prometheus exporter
	exporter, err := prometheus.New()
	if err != nil {
		return fmt.Errorf("create prometheus exporter: %w", err)
	}

	// Create meter provider
	m.meterProvider = meterprovider.New(
		meterprovider.WithReader(exporter),
		meterprovider.WithResource(res),
	)

	// Start Prometheus HTTP server
	m.startPrometheusServer()

	return nil
}

// initOTLPMetrics initializes OTLP metrics exporter
func (m *OTelManager) initOTLPMetrics(ctx context.Context, res *resource.Resource) error {
	// Placeholder for OTLP metrics exporter
	// Would use go.opentelemetry.io/otel/exporters/otlp/otlpmetric
	return fmt.Errorf("OTLP metrics exporter not implemented")
}

// startPrometheusServer starts Prometheus metrics HTTP server
func (m *OTelManager) startPrometheusServer() {
	addr := fmt.Sprintf(":%d", m.config.MetricsPort)
	m.prometheusServer = &http.Server{
		Addr:    addr,
		Handler: nil, // Default handler exposes /metrics
	}

	go func() {
		if err := m.prometheusServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Prometheus server error: %v\n", err)
		}
	}()
}

// Shutdown shuts down OpenTelemetry components
func (m *OTelManager) Shutdown(ctx context.Context) error {
	var errs []error

	// Shutdown all components
	for _, shutdown := range m.shutdownFuncs {
		if err := shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	// Shutdown Prometheus server
	if m.prometheusServer != nil {
		if err := m.prometheusServer.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	return nil
}

// ============================================================
// Tracer Management
// ============================================================

// GetTracer gets or creates a tracer for a component
func (m *OTelManager) GetTracer(name string) trace.Tracer {
	m.mu.RLock()
	tracer, ok := m.tracers[name]
	m.mu.RUnlock()

	if ok {
		return tracer
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double check
	if tracer, ok := m.tracers[name]; ok {
		return tracer
	}

	if m.tracerProvider != nil {
		tracer = m.tracerProvider.Tracer(name)
	} else {
		tracer = otel.Tracer(name)
	}

	m.tracers[name] = tracer
	return tracer
}

// GetMeter gets or creates a meter for a component
func (m *OTelManager) GetMeter(name string) metric.Meter {
	m.mu.RLock()
	meter, ok := m.meters[name]
	m.mu.RUnlock()

	if ok {
		return meter
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double check
	if meter, ok := m.meters[name]; ok {
		return meter
	}

	if m.meterProvider != nil {
		meter = m.meterProvider.Meter(name)
	} else {
		meter = otel.Meter(name)
	}

	m.meters[name] = meter
	return meter
}

// ============================================================
// Span Helpers
// ============================================================

// SpanBuilder helps build spans with common attributes
type SpanBuilder struct {
	tracer    trace.Tracer
	name      string
	attrs     []attribute.KeyValue
	parentCtx context.Context
	options   []trace.SpanStartOption
}

// NewSpanBuilder creates a new span builder
func (m *OTelManager) NewSpanBuilder(tracerName, spanName string) *SpanBuilder {
	return &SpanBuilder{
		tracer: m.GetTracer(tracerName),
		name:   spanName,
		attrs:  make([]attribute.KeyValue, 0),
	}
}

// WithParent sets the parent context
func (b *SpanBuilder) WithParent(ctx context.Context) *SpanBuilder {
	b.parentCtx = ctx
	return b
}

// WithAttribute adds an attribute
func (b *SpanBuilder) WithAttribute(key, value string) *SpanBuilder {
	b.attrs = append(b.attrs, attribute.String(key, value))
	return b
}

// WithIntAttribute adds an integer attribute
func (b *SpanBuilder) WithIntAttribute(key string, value int) *SpanBuilder {
	b.attrs = append(b.attrs, attribute.Int(key, value))
	return b
}

// WithFloatAttribute adds a float attribute
func (b *SpanBuilder) WithFloatAttribute(key string, value float64) *SpanBuilder {
	b.attrs = append(b.attrs, attribute.Float64(key, value))
	return b
}

// WithBoolAttribute adds a boolean attribute
func (b *SpanBuilder) WithBoolAttribute(key string, value bool) *SpanBuilder {
	b.attrs = append(b.attrs, attribute.Bool(key, value))
	return b
}

// Build starts the span
func (b *SpanBuilder) Build(ctx context.Context) (context.Context, trace.Span) {
	if b.parentCtx != nil {
		ctx = b.parentCtx
	}

	opts := []trace.SpanStartOption{
		trace.WithAttributes(b.attrs...),
	}
	opts = append(opts, b.options...)

	return b.tracer.Start(ctx, b.name, opts...)
}

// ============================================================
// Metrics Helpers
// ============================================================

// MetricsRegistry manages common metrics
type MetricsRegistry struct {
	meter         metric.Meter
_counters      map[string]metric.Int64Counter
_upDownCounters map[string]metric.Int64UpDownCounter
_histograms    map[string]metric.Float64Histogram
_gauges        map[string]metric.Float64ObservableGauge
	mu            sync.RWMutex
}

// NewMetricsRegistry creates a new metrics registry
func (m *OTelManager) NewMetricsRegistry(name string) *MetricsRegistry {
	return &MetricsRegistry{
		meter:           m.GetMeter(name),
		_counters:       make(map[string]metric.Int64Counter),
		_upDownCounters: make(map[string]metric.Int64UpDownCounter),
		_histograms:     make(map[string]metric.Float64Histogram),
		_gauges:         make(map[string]metric.Float64ObservableGauge),
	}
}

// RegisterCounter registers a counter metric
func (r *MetricsRegistry) RegisterCounter(name, description string, unit string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r._counters[name]; ok {
		return nil // Already registered
	}

	counter, err := r.meter.Int64Counter(name,
		metric.WithDescription(description),
		metric.WithUnit(unit),
	)
	if err != nil {
		return err
	}

	r._counters[name] = counter
	return nil
}

// IncrementCounter increments a counter
func (r *MetricsRegistry) IncrementCounter(ctx context.Context, name string, attrs ...attribute.KeyValue) error {
	r.mu.RLock()
	counter, ok := r._counters[name]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("counter not registered: %s", name)
	}

	counter.Add(ctx, 1, metric.WithAttributes(attrs...))
	return nil
}

// AddCounter adds to a counter
func (r *MetricsRegistry) AddCounter(ctx context.Context, name string, value int64, attrs ...attribute.KeyValue) error {
	r.mu.RLock()
	counter, ok := r._counters[name]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("counter not registered: %s", name)
	}

	counter.Add(ctx, value, metric.WithAttributes(attrs...))
	return nil
}

// RegisterHistogram registers a histogram metric
func (r *MetricsRegistry) RegisterHistogram(name, description string, unit string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r._histograms[name]; ok {
		return nil
	}

	histogram, err := r.meter.Float64Histogram(name,
		metric.WithDescription(description),
		metric.WithUnit(unit),
	)
	if err != nil {
		return err
	}

	r._histograms[name] = histogram
	return nil
}

// RecordHistogram records a histogram value
func (r *MetricsRegistry) RecordHistogram(ctx context.Context, name string, value float64, attrs ...attribute.KeyValue) error {
	r.mu.RLock()
	histogram, ok := r._histograms[name]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("histogram not registered: %s", name)
	}

	histogram.Record(ctx, value, metric.WithAttributes(attrs...))
	return nil
}

// RegisterUpDownCounter registers an up-down counter
func (r *MetricsRegistry) RegisterUpDownCounter(name, description string, unit string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r._upDownCounters[name]; ok {
		return nil
	}

	counter, err := r.meter.Int64UpDownCounter(name,
		metric.WithDescription(description),
		metric.WithUnit(unit),
	)
	if err != nil {
		return err
	}

	r._upDownCounters[name] = counter
	return nil
}

// AddUpDownCounter adds to an up-down counter
func (r *MetricsRegistry) AddUpDownCounter(ctx context.Context, name string, value int64, attrs ...attribute.KeyValue) error {
	r.mu.RLock()
	counter, ok := r._upDownCounters[name]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("up-down counter not registered: %s", name)
	}

	counter.Add(ctx, value, metric.WithAttributes(attrs...))
	return nil
}

// ============================================================
// Agent Metrics
// ============================================================

// AgentMetrics provides common metrics for agents
type AgentMetrics struct {
registry        *MetricsRegistry

	// Execution metrics
	ExecutionCount  metric.Int64Counter
	ExecutionErrors metric.Int64Counter
	ExecutionLatency metric.Float64Histogram

	// Token metrics
	InputTokens     metric.Int64Counter
	OutputTokens    metric.Int64Counter

	// Tool metrics
	ToolCalls       metric.Int64Counter
	ToolErrors      metric.Int64Counter

	// Cost metrics
	CostAccumulated metric.Float64Counter

	// Memory metrics
	MemoryOperations metric.Int64Counter
}

// NewAgentMetrics creates agent metrics
func NewAgentMetrics(m *OTelManager) *AgentMetrics {
	registry := m.NewMetricsRegistry("agent-platform")

	// Register all metrics
	registry.RegisterCounter("agent.execution.count", "Number of agent executions", "1")
	registry.RegisterCounter("agent.execution.errors", "Number of execution errors", "1")
	registry.RegisterHistogram("agent.execution.latency", "Execution latency", "ms")
	registry.RegisterCounter("agent.tokens.input", "Input tokens consumed", "tokens")
	registry.RegisterCounter("agent.tokens.output", "Output tokens produced", "tokens")
	registry.RegisterCounter("agent.tool.calls", "Tool call count", "1")
	registry.RegisterCounter("agent.tool.errors", "Tool call errors", "1")
	registry.RegisterCounter("agent.cost.total", "Total cost accumulated", "USD")
	registry.RegisterCounter("agent.memory.operations", "Memory operations", "1")

	return &AgentMetrics{
		registry: registry,
	}
}

// RecordExecution records an agent execution
func (m *AgentMetrics) RecordExecution(ctx context.Context, agentID string, success bool, latencyMs float64, attrs ...attribute.KeyValue) {
	baseAttrs := []attribute.KeyValue{
		attribute.String("agent.id", agentID),
	}
	baseAttrs = append(baseAttrs, attrs...)

	m.registry.IncrementCounter(ctx, "agent.execution.count", baseAttrs...)

	if !success {
		m.registry.IncrementCounter(ctx, "agent.execution.errors", baseAttrs...)
	}

	m.registry.RecordHistogram(ctx, "agent.execution.latency", latencyMs, baseAttrs...)
}

// RecordTokens records token usage
func (m *AgentMetrics) RecordTokens(ctx context.Context, agentID string, inputTokens, outputTokens int64, attrs ...attribute.KeyValue) {
	baseAttrs := []attribute.KeyValue{
		attribute.String("agent.id", agentID),
	}
	baseAttrs = append(baseAttrs, attrs...)

	m.registry.AddCounter(ctx, "agent.tokens.input", inputTokens, baseAttrs...)
	m.registry.AddCounter(ctx, "agent.tokens.output", outputTokens, baseAttrs...)
}

// RecordToolCall records a tool call
func (m *AgentMetrics) RecordToolCall(ctx context.Context, agentID, toolName string, success bool, attrs ...attribute.KeyValue) {
	baseAttrs := []attribute.KeyValue{
		attribute.String("agent.id", agentID),
		attribute.String("tool.name", toolName),
	}
	baseAttrs = append(baseAttrs, attrs...)

	m.registry.IncrementCounter(ctx, "agent.tool.calls", baseAttrs...)

	if !success {
		m.registry.IncrementCounter(ctx, "agent.tool.errors", baseAttrs...)
	}
}

// RecordCost records cost
func (m *AgentMetrics) RecordCost(ctx context.Context, agentID string, cost float64, attrs ...attribute.KeyValue) {
	baseAttrs := []attribute.KeyValue{
		attribute.String("agent.id", agentID),
	}
	baseAttrs = append(baseAttrs, attrs...)

	m.registry.RecordHistogram(ctx, "agent.cost.total", cost, baseAttrs...)
}

// ============================================================
// stdoutExporter - Simple exporter for development
// ============================================================

type stdoutExporter struct{}

func (e *stdoutExporter) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
	for _, span := range spans {
		fmt.Printf("[TRACE] %s: %s (duration: %dms)\n",
			span.SpanContext().TraceID(),
			span.Name(),
			span.EndTime().Sub(span.StartTime()).Milliseconds(),
		)
	}
	return nil
}

func (e *stdoutExporter) Shutdown(ctx context.Context) error {
	return nil
}

// ============================================================
// Global Instance
// ============================================================

var (
	globalManager *OTelManager
	globalMu      sync.RWMutex
)

// InitGlobal initializes global OpenTelemetry manager
func InitGlobal(ctx context.Context, config *OTelConfig) error {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalManager != nil {
		return nil // Already initialized
	}

	manager := NewOTelManager(config)
	if err := manager.Initialize(ctx); err != nil {
		return err
	}

	globalManager = manager
	return nil
}

// GetGlobal returns global OpenTelemetry manager
func GetGlobal() *OTelManager {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalManager
}

// ShutdownGlobal shuts down global manager
func ShutdownGlobal(ctx context.Context) error {
	globalMu.RLock()
	manager := globalManager
	globalMu.RUnlock()

	if manager == nil {
		return nil
	}

	return manager.Shutdown(ctx)
}

// ============================================================
// Context Helpers
// ============================================================

// StartSpan starts a span with the global tracer
func StartSpan(ctx context.Context, tracerName, spanName string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	manager := GetGlobal()
	if manager == nil {
		// Return no-op span if not initialized
		return ctx, trace.SpanFromContext(ctx)
	}

	builder := manager.NewSpanBuilder(tracerName, spanName)
	for _, attr := range attrs {
		builder.WithAttribute(string(attr.Key), attr.Value.Emit())
	}

	return builder.Build(ctx)
}

// EndSpan ends a span with error status if provided
func EndSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(trace.StatusCodeError, err.Error())
	} else {
		span.SetStatus(trace.StatusCodeOk, "")
	}
	span.End()
}

// AddSpanEvent adds an event to the current span
func AddSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

// GetTraceID gets the trace ID from context
func GetTraceID(ctx context.Context) string {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		return spanCtx.TraceID().String()
	}
	return ""
}

// GetSpanID gets the span ID from context
func GetSpanID(ctx context.Context) string {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		return spanCtx.SpanID().String()
	}
	return ""
}

// Float64Counter is a helper type for float counters
type Float64Counter struct {
	counter metric.Float64Counter
}

// Add adds a value to the counter
func (c *Float64Counter) Add(ctx context.Context, value float64, attrs ...attribute.KeyValue) {
	c.counter.Add(ctx, value, metric.WithAttributes(attrs...))
}