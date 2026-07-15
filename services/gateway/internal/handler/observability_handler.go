// Package handler provides HTTP handlers for Gateway
package handler

import (
	"agent-platform/pkg/config"
	pb "agent-platform/pkg/pb/harness"
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ObservabilityHandler handles observability requests.
// When harness-service is available, it proxies real data;
// otherwise it falls back to empty responses.
type ObservabilityHandler struct {
	cfg           *config.Config
	harnessConn   *grpc.ClientConn
	harnessClient pb.HarnessServiceClient
}

// NewObservabilityHandler creates a new observability handler (stub, no harness connection)
func NewObservabilityHandler() *ObservabilityHandler {
	return &ObservabilityHandler{}
}

// NewRealObservabilityHandler creates an observability handler with harness-service gRPC client
func NewRealObservabilityHandler(cfg *config.Config) *ObservabilityHandler {
	h := &ObservabilityHandler{cfg: cfg}

	if addr := cfg.Services.Harness; addr != "" {
		conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			fmt.Printf("[Observability] Failed to connect to harness-service: %v\n", err)
			return h
		}
		h.harnessConn = conn
		h.harnessClient = pb.NewHarnessServiceClient(conn)
		fmt.Printf("[Observability] Connected to harness-service at %s\n", addr)
	}

	return h
}

// emptyResponse returns a standard empty data envelope
func emptyResponse(fields gin.H) gin.H {
	return gin.H{"code": 0, "data": fields}
}

// GetTraces returns trace list — proxies to harness-service LLM metrics
func (h *ObservabilityHandler) GetTraces(c *gin.Context) {
	summary := h.fetchMetricsSummary(c)
	if summary != nil {
		c.JSON(200, emptyResponse(gin.H{
			"total_traces":  summary.TotalCalls,
			"success_traces": summary.SuccessCalls,
			"success_rate":  summary.SuccessRate,
			"avg_latency_ms": summary.AvgLatency,
			"total_cost":    summary.TotalCost,
			"slo_statuses":  summary.SloStatuses,
		}))
		return
	}
	c.JSON(200, emptyResponse(gin.H{
		"traces": []interface{}{},
		"total":  0,
	}))
}

// GetTrace returns single trace detail
func (h *ObservabilityHandler) GetTrace(c *gin.Context) {
	traceID := c.Param("id")
	c.JSON(200, emptyResponse(gin.H{
		"trace_id": traceID,
		"spans":    []interface{}{},
	}))
}

// GetMetrics returns performance metrics — proxies to harness-service
func (h *ObservabilityHandler) GetMetrics(c *gin.Context) {
	summary := h.fetchMetricsSummary(c)
	if summary != nil {
		c.JSON(200, emptyResponse(gin.H{
			"total_traces":   summary.TotalCalls,
			"success_traces": summary.SuccessCalls,
			"success_rate":   summary.SuccessRate,
			"avg_latency_ms": summary.AvgLatency,
			"total_cost":     summary.TotalCost,
			"slo_statuses":   summary.SloStatuses,
		}))
		return
	}
	c.JSON(200, emptyResponse(gin.H{
		"metrics":      []interface{}{},
		"total_traces": 0,
	}))
}

// GetProfile returns performance profile
func (h *ObservabilityHandler) GetProfile(c *gin.Context) {
	sessionID := c.Param("id")
	c.JSON(200, emptyResponse(gin.H{
		"session_id": sessionID,
		"spans":      []interface{}{},
	}))
}

// GetStats returns trace statistics — proxies to harness-service
func (h *ObservabilityHandler) GetStats(c *gin.Context) {
	summary := h.fetchMetricsSummary(c)
	if summary != nil {
		c.JSON(200, emptyResponse(gin.H{
			"total_traces":    summary.TotalCalls,
			"success_traces":  summary.SuccessCalls,
			"error_traces":    summary.TotalCalls - summary.SuccessCalls,
			"success_rate":    summary.SuccessRate,
			"avg_latency_ms":  summary.AvgLatency,
			"total_cost":      summary.TotalCost,
		}))
		return
	}
	c.JSON(200, emptyResponse(gin.H{
		"total_traces":   0,
		"success_traces": 0,
		"error_traces":   0,
		"success_rate":   0,
		"avg_latency_ms": 0,
		"total_cost":     0,
	}))
}

// fetchMetricsSummary is a helper that calls harness-service GetLLMMetrics
func (h *ObservabilityHandler) fetchMetricsSummary(c *gin.Context) *pb.LLMMetricsSummary {
	if h.harnessClient == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.harnessClient.GetLLMMetrics(ctx, &pb.GetLLMMetricsRequest{})
	if err != nil {
		return nil
	}
	return resp
}

// Close closes the gRPC connection
func (h *ObservabilityHandler) Close() {
	if h.harnessConn != nil {
		h.harnessConn.Close()
	}
}
