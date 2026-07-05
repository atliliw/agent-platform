// Package handler provides HTTP handlers for Gateway
package handler

import (
	"context"
	"strconv"
	"strings"
	"time"

	"agent-platform/pkg/config"
	common "agent-platform/pkg/pb/common"
	pb "agent-platform/pkg/pb/harness"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// RealHarnessHandler handles Harness requests with real gRPC calls
type RealHarnessHandler struct {
	cfg    *config.Config
	client pb.HarnessServiceClient
	conn   *grpc.ClientConn
}

// NewRealHarnessHandler creates a new harness handler with gRPC connection
func NewRealHarnessHandler(cfg *config.Config) *RealHarnessHandler {
	// Connect to harness-service
	addr := cfg.Services.Harness
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		// Return stub handler if connection fails
		return &RealHarnessHandler{
			cfg:    cfg,
			client: nil,
			conn:   nil,
		}
	}

	return &RealHarnessHandler{
		cfg:    cfg,
		client: pb.NewHarnessServiceClient(conn),
		conn:   conn,
	}
}

// Close closes the gRPC connection
func (h *RealHarnessHandler) Close() error {
	if h.conn != nil {
		return h.conn.Close()
	}
	return nil
}

// CreateRule creates a rule
func (h *RealHarnessHandler) CreateRule(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.CreateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.CreateRule(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"rule": gin.H{
				"id":         resp.Id,
				"agent_id":   resp.AgentId,
				"name":       resp.Name,
				"type":       resp.Type,
				"config":     resp.Config,
				"enabled":    resp.Enabled,
				"created_at": resp.CreatedAt,
			},
		},
	})
}

// ListRules lists rules
func (h *RealHarnessHandler) ListRules(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.ListRulesRequest{
		AgentId:  c.Query("agent_id"),
		TenantId: c.GetHeader("X-Tenant-ID"),
	}

	resp, err := h.client.ListRules(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	var rules []gin.H
	for _, r := range resp.Rules {
		rules = append(rules, gin.H{
			"id":         r.Id,
			"agent_id":   r.AgentId,
			"name":       r.Name,
			"type":       r.Type,
			"config":     r.Config,
			"enabled":    r.Enabled,
			"created_at": r.CreatedAt,
		})
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"rules": rules},
	})
}

// DeleteRule deletes a rule
func (h *RealHarnessHandler) DeleteRule(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.DeleteRuleRequest{
		Id:       c.Param("id"),
		TenantId: c.GetHeader("X-Tenant-ID"),
	}

	_, err := h.client.DeleteRule(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{"code": 0, "message": "deleted"})
}

// CheckGuardrail checks guardrail
func (h *RealHarnessHandler) CheckGuardrail(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.GuardrailCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.CheckGuardrail(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"passed":     resp.Passed,
			"violations": resp.Violations,
		},
	})
}

// RunEval runs evaluation
func (h *RealHarnessHandler) RunEval(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.RunEvalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := h.client.RunEval(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	var results []gin.H
	for _, r := range resp.Results {
		results = append(results, gin.H{
			"case_id": r.CaseId,
			"actual":  r.Actual,
			"score":   r.Score,
			"passed":  r.Passed,
		})
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"run_id":              resp.RunId,
			"results":             results,
			"avg_score":           resp.AvgScore,
			"regression_detected": resp.RegressionDetected,
		},
	})
}

// CreateABTest creates A/B test
func (h *RealHarnessHandler) CreateABTest(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.CreateABTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.CreateABTest(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"ab_test": gin.H{
				"id":            resp.Id,
				"name":          resp.Name,
				"control_model": resp.ControlModel,
				"variant_model": resp.VariantModel,
				"traffic_split": resp.TrafficSplit,
				"status":        resp.Status,
				"created_at":    resp.CreatedAt,
			},
		},
	})
}

// ListABTests lists A/B tests
func (h *RealHarnessHandler) ListABTests(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.ListABTestsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req = pb.ListABTestsRequest{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.ListABTests(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	var tests []gin.H
	for _, t := range resp.Tests {
		tests = append(tests, gin.H{
			"id":             t.Id,
			"name":           t.Name,
			"type":           t.Type,
			"control_config": t.ControlConfig,
			"variant_config": t.VariantConfig,
			"traffic_split":  t.TrafficSplit,
			"status":         t.Status,
			"created_at":     t.CreatedAt,
		})
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"tests": tests,
		},
	})
}

// DeleteABTest deletes an A/B test
func (h *RealHarnessHandler) DeleteABTest(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.PromoteVariantRequest{
		TestId:   c.Param("id"),
		TenantId: c.GetHeader("X-Tenant-ID"),
	}

	_, err := h.client.DeleteABTest(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{"code": 0, "message": "deleted"})
}

// GetABTestResult gets A/B test result
func (h *RealHarnessHandler) GetABTestResult(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.GetABTestResultRequest{
		TestId: c.Param("id"),
	}

	resp, err := h.client.GetABTestResult(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"result": gin.H{
				"control_score": resp.ControlScore,
				"variant_score": resp.VariantScore,
				"delta":         resp.Delta,
				"p_value":       resp.PValue,
				"significant":   resp.Significant,
				"recommended":   resp.Recommended,
			},
		},
	})
}

// GetSLOStatus gets SLO status
func (h *RealHarnessHandler) GetSLOStatus(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.GetSLOStatusRequest{
		AgentId: c.Query("agent_id"),
	}

	resp, err := h.client.GetSLOStatus(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	var statuses []gin.H
	for _, s := range resp.Statuses {
		statuses = append(statuses, gin.H{
			"name":             s.Name,
			"target":           s.Target,
			"current":          s.Current,
			"status":           s.Status,
			"budget_remaining": s.BudgetRemaining,
		})
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"statuses": statuses},
	})
}

// GetLLMMetrics gets recent LLM call metrics
func (h *RealHarnessHandler) GetLLMMetrics(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.GetSLOStatus(ctx, &pb.GetSLOStatusRequest{})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	// Extract metrics from SLO statuses
	var totalCalls, successCalls int64
	var successRate, avgLatency float64

	for _, s := range resp.Statuses {
		switch {
		case s.Name == "Success Rate > 99%" || s.Name == "Success Rate":
			successRate = s.Current
		case s.Name == "Availability > 99.9%" || s.Name == "Availability":
			// Availability also represents success
		case strings.Contains(s.Name, "Latency"):
			avgLatency = s.Current
		}
	}

	// Calculate derived values
	if successRate > 0 {
		successCalls = 1 // At least one successful call
		totalCalls = 1
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"total_calls":   totalCalls,
			"success_calls": successCalls,
			"success_rate":  successRate,
			"avg_latency":   avgLatency,
			"slo_statuses":  resp.Statuses,
		},
	})
}

// CreateSLO creates an SLO
func (h *RealHarnessHandler) CreateSLO(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.CreateSLORequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.CreateSLO(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"slo": gin.H{
				"id":         resp.Id,
				"agent_id":   resp.AgentId,
				"name":       resp.Name,
				"target":     resp.Target,
				"type":       resp.Type,
				"created_at": resp.CreatedAt,
			},
		},
	})
}

// Chat handles harness chat with guardrails
func (h *RealHarnessHandler) Chat(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.HarnessChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	// Parse tenant_id from header
	req.TenantId = c.GetHeader("X-Tenant-ID")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := h.client.Chat(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"content":      resp.Content,
			"tokens":       resp.Tokens,
			"cost":         resp.Cost,
			"trace_id":     resp.TraceId,
			"input_guard":  resp.InputGuard,
			"rule_check":   resp.RuleCheck,
			"output_guard": resp.OutputGuard,
			"error":        resp.Error,
		},
	})
}

// ==================== Feature Flag Handlers ====================

// CreateFeatureFlag creates a feature flag
func (h *RealHarnessHandler) CreateFeatureFlag(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.CreateFeatureFlagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.CreateFeatureFlag(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"flag": resp},
	})
}

// ListFeatureFlags lists feature flags
func (h *RealHarnessHandler) ListFeatureFlags(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.ListFeatureFlagsRequest{
		TenantId: c.GetHeader("X-Tenant-ID"),
		Status:   c.Query("status"),
	}

	resp, err := h.client.ListFeatureFlags(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"flags": resp.Flags},
	})
}

// ToggleFeatureFlag toggles a feature flag
func (h *RealHarnessHandler) ToggleFeatureFlag(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.ToggleFeatureFlagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.ToggleFeatureFlag(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"flag": resp},
	})
}

// EvaluateFeatureFlag evaluates a feature flag
func (h *RealHarnessHandler) EvaluateFeatureFlag(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.EvaluateFeatureFlagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.EvaluateFeatureFlag(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"key":    resp.Key,
			"value":  resp.Value,
			"reason": resp.Reason,
		},
	})
}

// ==================== Chaos Handlers ====================

// CreateChaosExperiment creates a chaos experiment
func (h *RealHarnessHandler) CreateChaosExperiment(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.CreateChaosExperimentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.CreateChaosExperiment(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"experiment": resp},
	})
}

// StartChaosExperiment starts a chaos experiment
func (h *RealHarnessHandler) StartChaosExperiment(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.StartChaosExperimentRequest{
		ExperimentId: c.Param("id"),
	}

	resp, err := h.client.StartChaosExperiment(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"experiment": resp},
	})
}

// StopChaosExperiment stops a chaos experiment
func (h *RealHarnessHandler) StopChaosExperiment(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.StopChaosExperimentRequest{
		ExperimentId: c.Param("id"),
	}

	resp, err := h.client.StopChaosExperiment(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"experiment": resp},
	})
}

// ListChaosExperiments lists chaos experiments
func (h *RealHarnessHandler) ListChaosExperiments(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.ListChaosExperimentsRequest{
		AgentId: c.Query("agent_id"),
		Status:  c.Query("status"),
	}

	resp, err := h.client.ListChaosExperiments(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"experiments": resp.Experiments},
	})
}

// ==================== Cost Handlers ====================

// GetCostReport gets a cost report
func (h *RealHarnessHandler) GetCostReport(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Parse start and end time from query parameters
	var startTime, endTime int64

	if startStr := c.Query("start"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			startTime = t.Unix()
		} else if ts, err := strconv.ParseInt(startStr, 10, 64); err == nil {
			startTime = ts
		}
	}

	if endStr := c.Query("end"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			endTime = t.Unix()
		} else if ts, err := strconv.ParseInt(endStr, 10, 64); err == nil {
			endTime = ts
		}
	}

	// Default to last 30 days if not specified
	if startTime == 0 {
		startTime = time.Now().AddDate(0, 0, -30).Unix()
	}
	if endTime == 0 {
		endTime = time.Now().Unix()
	}

	req := &pb.CostReportRequest{
		AgentId:   c.Query("agent_id"),
		StartTime: startTime,
		EndTime:   endTime,
	}

	resp, err := h.client.GetCostReport(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"report": resp},
	})
}

// SetModelPricing sets model pricing
func (h *RealHarnessHandler) SetModelPricing(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.SetModelPricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.SetModelPricing(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"pricing": resp},
	})
}

// ListModelPricing lists model pricing
func (h *RealHarnessHandler) ListModelPricing(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.ListModelPricing(ctx, &common.Empty{})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"pricings": resp.Pricings},
	})
}

// GetCostRecommendations gets cost recommendations
func (h *RealHarnessHandler) GetCostRecommendations(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.GetCostRecommendations(ctx, &common.Empty{})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"recommendations": resp.Recommendations},
	})
}

// RecordCostUsage records cost usage
func (h *RealHarnessHandler) RecordCostUsage(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.RecordCostUsageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := h.client.RecordCostUsage(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code":    0,
		"message": "ok",
	})
}

// ==================== Proposal Handlers ====================

// CreateProposal creates a proposal
func (h *RealHarnessHandler) CreateProposal(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.CreateProposalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.CreateProposal(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"proposal": resp},
	})
}

// ListProposals lists proposals
func (h *RealHarnessHandler) ListProposals(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.ListProposalsRequest{
		AgentId: c.Query("agent_id"),
		Status:  c.Query("status"),
	}

	resp, err := h.client.ListProposals(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"proposals": resp.Proposals},
	})
}

// ApproveProposal approves a proposal
func (h *RealHarnessHandler) ApproveProposal(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	req := &pb.ApproveProposalRequest{
		ProposalId: c.Param("id"),
		ApprovedBy: c.GetHeader("X-User"),
	}
	// Override from JSON body if provided
	var body struct {
		ApprovedBy string `json:"approved_by"`
	}
	if err := c.ShouldBindJSON(&body); err == nil && body.ApprovedBy != "" {
		req.ApprovedBy = body.ApprovedBy
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.ApproveProposal(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"proposal": resp},
	})
}

// RejectProposal rejects a proposal
func (h *RealHarnessHandler) RejectProposal(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	req := &pb.RejectProposalRequest{
		ProposalId: c.Param("id"),
	}
	// Override from JSON body if provided
	var body struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&body); err == nil && body.Reason != "" {
		req.Reason = body.Reason
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.RejectProposal(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"proposal": resp},
	})
}

// ExecuteProposal executes an approved proposal
func (h *RealHarnessHandler) ExecuteProposal(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	req := &pb.ApproveProposalRequest{
		ProposalId: c.Param("id"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := h.client.ApproveProposal(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"proposal": resp},
	})
}

// AnalyzeAndPropose analyzes cost/SLO data and generates proposals automatically
func (h *RealHarnessHandler) AnalyzeAndPropose(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &pb.AnalyzeAndProposeRequest{
		AgentId: c.Query("agent_id"),
	}

	resp, err := h.client.AnalyzeAndPropose(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"proposals":       resp.Proposals,
			"analysis_summary": resp.AnalysisSummary,
		},
	})
}

// ==================== Catalog Handlers ====================

// ListCatalogAgents lists catalog agents
func (h *RealHarnessHandler) ListCatalogAgents(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.ListCatalogAgentsRequest{
		Type:   c.Query("type"),
		Status: c.Query("status"),
	}

	resp, err := h.client.ListCatalogAgents(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"agents": resp.Agents},
	})
}

// ==================== RCA Handlers ====================

// RecordChange records a change event
func (h *RealHarnessHandler) RecordChange(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.RecordChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.RecordChange(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"change": resp},
	})
}

// AnalyzeIncident analyzes an incident
func (h *RealHarnessHandler) AnalyzeIncident(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &pb.AnalyzeRequest{
		IncidentId: c.Param("id"),
	}

	resp, err := h.client.Analyze(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"report": resp},
	})
}

// ==================== Catalog Handlers ====================

// GetCatalogAgent gets a catalog agent
func (h *RealHarnessHandler) GetCatalogAgent(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.GetFeatureFlagRequest{
		Key: c.Param("id"),
	}

	resp, err := h.client.GetCatalogAgent(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"agent": resp},
	})
}

// RegisterCatalogAgent registers a catalog agent
func (h *RealHarnessHandler) RegisterCatalogAgent(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.RegisterCatalogAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.RegisterCatalogAgent(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"agent": resp},
	})
}

// RecordCatalogUsage records catalog usage
func (h *RealHarnessHandler) RecordCatalogUsage(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.RecordCatalogUsageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := h.client.RecordCatalogUsage(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code":    0,
		"message": "ok",
	})
}

// RateCatalogAgent rates a catalog agent
func (h *RealHarnessHandler) RateCatalogAgent(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.RateCatalogAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := h.client.RateCatalogAgent(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code":    0,
		"message": "ok",
	})
}

// ==================== Rollback Handlers ====================

// CreateRollbackConfig creates a rollback config
func (h *RealHarnessHandler) CreateRollbackConfig(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.CreateRollbackConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.CreateRollbackConfig(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"config": resp},
	})
}

// GetRollbackConfig gets a rollback config
func (h *RealHarnessHandler) GetRollbackConfig(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.GetFeatureFlagRequest{
		Key: c.Param("id"),
	}

	resp, err := h.client.GetRollbackConfig(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"config": resp},
	})
}

// TakeSnapshot takes a snapshot
func (h *RealHarnessHandler) TakeSnapshot(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.TakeSnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.TakeSnapshot(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"snapshot": resp},
	})
}

// ListSnapshots lists snapshots
func (h *RealHarnessHandler) ListSnapshots(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.ListSnapshotsRequest{
		ConfigId: c.Param("id"),
		Limit:    20,
	}

	resp, err := h.client.ListSnapshots(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"snapshots": resp.Snapshots},
	})
}

// ExecuteRollback executes a rollback
func (h *RealHarnessHandler) ExecuteRollback(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.ExecuteRollbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := h.client.ExecuteRollback(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"event": resp},
	})
}

// ==================== Golden Path Handlers ====================

// CreateGoldenPathTemplate creates a golden path template
func (h *RealHarnessHandler) CreateGoldenPathTemplate(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.CreateGoldenPathTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.CreateGoldenPathTemplate(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"template": resp},
	})
}

// ListGoldenPathTemplates lists golden path templates
func (h *RealHarnessHandler) ListGoldenPathTemplates(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.ListGoldenPathTemplatesRequest{
		Type:     c.Query("type"),
		Category: c.Query("category"),
	}

	resp, err := h.client.ListGoldenPathTemplates(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"templates": resp.Templates},
	})
}

// InstantiateTemplate instantiates a template
func (h *RealHarnessHandler) InstantiateTemplate(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.InstantiateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := h.client.InstantiateTemplate(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"message": "Template instantiated successfully"},
	})
}

// ==================== Optimizer Handlers ====================

// RunOptimizer runs the optimizer
func (h *RealHarnessHandler) RunOptimizer(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.RunOptimizerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := h.client.RunOptimizer(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"result": resp},
	})
}

// ==================== Scheduler Handlers ====================

// SetEvalSchedule creates or updates an evaluation schedule
func (h *RealHarnessHandler) SetEvalSchedule(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.SetEvalScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.SetEvalSchedule(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"schedule": resp},
	})
}

// GetEvalSchedule gets an evaluation schedule
func (h *RealHarnessHandler) GetEvalSchedule(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.GetEvalScheduleRequest{
		Id: c.Param("id"),
	}

	resp, err := h.client.GetEvalSchedule(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"schedule": resp},
	})
}

// ListEvalSchedules lists evaluation schedules
func (h *RealHarnessHandler) ListEvalSchedules(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.ListEvalSchedulesRequest{
		AgentId: c.Query("agent_id"),
		Status:  c.Query("status"),
	}

	resp, err := h.client.ListEvalSchedules(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"schedules": resp.Schedules},
	})
}

// PauseEvalSchedule pauses an evaluation schedule
func (h *RealHarnessHandler) PauseEvalSchedule(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.PauseScheduleRequest{
		Id: c.Param("id"),
	}

	resp, err := h.client.PauseEvalSchedule(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"schedule": resp},
	})
}

// ResumeEvalSchedule resumes an evaluation schedule
func (h *RealHarnessHandler) ResumeEvalSchedule(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.ResumeScheduleRequest{
		Id: c.Param("id"),
	}

	resp, err := h.client.ResumeEvalSchedule(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"schedule": resp},
	})
}

// DeleteEvalSchedule deletes an evaluation schedule
func (h *RealHarnessHandler) DeleteEvalSchedule(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.GetEvalScheduleRequest{
		Id: c.Param("id"),
	}

	_, err := h.client.DeleteEvalSchedule(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code":    0,
		"message": "Schedule deleted successfully",
	})
}

// RunEvalScheduleNow manually triggers a schedule run
func (h *RealHarnessHandler) RunEvalScheduleNow(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := &pb.RunScheduleNowRequest{
		Id: c.Param("id"),
	}

	resp, err := h.client.RunEvalScheduleNow(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"result": resp},
	})
}

// GetEvalScheduleResults gets results for a schedule
func (h *RealHarnessHandler) GetEvalScheduleResults(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.GetScheduleResultsRequest{
		ScheduleId: c.Param("id"),
		Limit:      int32(c.GetInt("limit")),
	}

	resp, err := h.client.GetEvalScheduleResults(ctx, req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"results": resp.Results},
	})
}

// GetSchedulerStatus gets the scheduler status
func (h *RealHarnessHandler) GetSchedulerStatus(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.GetSchedulerStatus(ctx, &common.Empty{})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"status": resp},
	})
}

// SchedulerControl starts or stops the scheduler
func (h *RealHarnessHandler) SchedulerControl(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req pb.SchedulerControlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.SchedulerControl(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"status": resp},
	})
}

// GetSchedulerStats gets scheduler statistics
func (h *RealHarnessHandler) GetSchedulerStats(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.GetSchedulerStats(ctx, &common.Empty{})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"stats": resp},
	})
}

// ==================== Session Replay ====================

// CreateSession creates a session for replay recording
func (h *RealHarnessHandler) CreateSession(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	var req pb.CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.CreateSession(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// GetSessionReplay gets session detail for replay
func (h *RealHarnessHandler) GetSessionReplay(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.GetSession(ctx, &pb.GetSessionRequest{SessionId: id})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// ListSessionReplays lists sessions for replay
func (h *RealHarnessHandler) ListSessionReplays(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	agentID := c.Query("agent_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.ListSessions(ctx, &pb.ListSessionsRequest{AgentId: agentID, PageSize: int32(limit)})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// GetSessionGraph gets the execution graph for a session
func (h *RealHarnessHandler) GetSessionGraph(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.GetSessionGraph(ctx, &pb.GetSessionGraphRequest{SessionId: id})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// ReplaySession replays a session
func (h *RealHarnessHandler) ReplaySession(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := h.client.ReplaySession(ctx, &pb.ReplaySessionRequest{SessionId: id})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// ExportSession exports a session
func (h *RealHarnessHandler) ExportSession(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.ExportSession(ctx, &pb.ExportSessionRequest{SessionId: id})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// DeleteSessionReplay deletes a session
func (h *RealHarnessHandler) DeleteSessionReplay(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := h.client.DeleteSessionGRPC(ctx, &pb.GetSessionRequest{SessionId: id})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": gin.H{"deleted": true}})
}

// GetSessionStats gets session statistics
func (h *RealHarnessHandler) GetSessionStats(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.ListSessions(ctx, &pb.ListSessionsRequest{PageSize: 1000})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	sessions := resp.GetSessions()
	var totalSessions, runningSessions, completedSessions, failedSessions int64
	var totalTokens int64
	var totalCost float64
	var totalDuration int64

	for _, s := range sessions {
		totalSessions++
		switch s.Status {
		case "running":
			runningSessions++
		case "completed":
			completedSessions++
		case "failed":
			failedSessions++
		}
		totalTokens += s.TotalTokens
		totalCost += s.TotalCost
		totalDuration += s.Duration
	}

	var avgDuration int64
	if totalSessions > 0 {
		avgDuration = totalDuration / totalSessions
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"total_sessions":    totalSessions,
			"running_sessions":  runningSessions,
			"completed_sessions": completedSessions,
			"failed_sessions":   failedSessions,
			"total_tokens":      totalTokens,
			"total_cost":        totalCost,
			"avg_duration":      avgDuration,
		},
	})
}

// ==================== Prompt Management ====================

// CreatePrompt creates a new prompt template
func (h *RealHarnessHandler) CreatePrompt(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	var req pb.CreatePromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.CreatePrompt(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// ListPrompts lists prompt templates
func (h *RealHarnessHandler) ListPrompts(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	category := c.Query("category")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.ListPrompts(ctx, &pb.ListPromptsRequest{Category: category})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// GetPrompt gets a prompt by key
func (h *RealHarnessHandler) GetPrompt(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	key := c.Param("key")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.GetPrompt(ctx, &pb.GetPromptRequest{Key: key})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// DeletePrompt deletes a prompt
func (h *RealHarnessHandler) DeletePrompt(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	key := c.Param("key")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := h.client.DeletePrompt(ctx, &pb.GetPromptRequest{Key: key})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": gin.H{"deleted": true}})
}

// CreatePromptVersion creates a new prompt version
func (h *RealHarnessHandler) CreatePromptVersion(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	var req pb.CreatePromptVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}
	req.PromptKey = c.Param("key")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.CreatePromptVersion(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// ListPromptVersions lists prompt versions
func (h *RealHarnessHandler) ListPromptVersions(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	key := c.Param("key")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.ListPromptVersions(ctx, &pb.ListPromptVersionsRequest{PromptKey: key})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// GetActivePromptVersion gets the active version of a prompt
func (h *RealHarnessHandler) GetActivePromptVersion(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	key := c.Param("key")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.GetActivePromptVersion(ctx, &pb.GetActivePromptVersionRequest{PromptKey: key})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// ActivatePromptVersion activates a prompt version
func (h *RealHarnessHandler) ActivatePromptVersion(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	var req pb.ActivatePromptVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.ActivatePromptVersion(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// ComparePromptVersions compares two prompt versions
func (h *RealHarnessHandler) ComparePromptVersions(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	var req pb.ComparePromptVersionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.ComparePromptVersions(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// RenderPrompt renders a prompt with variables
func (h *RealHarnessHandler) RenderPrompt(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	var req pb.RenderPromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.RenderPrompt(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// GetPromptPerformance gets prompt performance metrics
func (h *RealHarnessHandler) GetPromptPerformance(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	versionID := c.Param("versionId")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.GetPromptPerformance(ctx, &pb.GetPromptPerformanceRequest{VersionId: versionID})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// ==================== Red Team Testing ====================

// CreateRedTeamTest creates a red team test
func (h *RealHarnessHandler) CreateRedTeamTest(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	var req pb.CreateRedTeamTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.CreateRedTeamTest(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// ListRedTeamTests lists red team tests
func (h *RealHarnessHandler) ListRedTeamTests(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	agentID := c.Query("agent_id")
	status := c.Query("status")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.ListRedTeamTests(ctx, &pb.ListRedTeamTestsRequest{AgentId: agentID, Status: status})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// GetRedTeamTest gets a red team test
func (h *RealHarnessHandler) GetRedTeamTest(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.GetRedTeamTest(ctx, &pb.GetRedTeamTestRequest{Id: id})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// RunRedTeamTest runs a red team test
func (h *RealHarnessHandler) RunRedTeamTest(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	resp, err := h.client.RunRedTeamTest(ctx, &pb.RunRedTeamTestRequest{TestId: id})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// GetRedTeamReport gets a red team report
func (h *RealHarnessHandler) GetRedTeamReport(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	testID := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.GetRedTeamReportByTest(ctx, &pb.GetRedTeamReportByTestRequest{TestId: testID})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// ListRedTeamAttacks lists red team attacks for a test
func (h *RealHarnessHandler) ListRedTeamAttacks(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	testID := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.ListRedTeamAttacks(ctx, &pb.ListRedTeamAttacksRequest{TestId: testID})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// GetAttackPayloads gets available attack payloads
func (h *RealHarnessHandler) GetAttackPayloads(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	category := c.Query("category")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.GetAttackPayloads(ctx, &pb.GetAttackPayloadsRequest{Category: category})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// DeleteRedTeamTest deletes a red team test
func (h *RealHarnessHandler) DeleteRedTeamTest(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := h.client.DeleteRedTeamTest(ctx, &pb.DeleteRedTeamTestRequest{TestId: id})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": gin.H{"deleted": true}})
}

// ==================== RAG Metrics ====================

// EvaluateRAG evaluates RAG metrics
func (h *RealHarnessHandler) EvaluateRAG(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	var req pb.EvaluateRAGRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	resp, err := h.client.EvaluateRAG(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// BatchEvaluateRAG batch evaluates RAG metrics
func (h *RealHarnessHandler) BatchEvaluateRAG(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	var req pb.BatchEvaluateRAGRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	resp, err := h.client.BatchEvaluateRAG(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// ListRAGMetrics lists RAG metrics
func (h *RealHarnessHandler) ListRAGMetrics(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.ListRAGMetrics(ctx, &pb.ListRAGMetricsRequest{Limit: int32(limit)})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// GetRAGMetrics gets specific RAG metrics
func (h *RealHarnessHandler) GetRAGMetrics(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.GetRAGMetrics(ctx, &pb.GetRAGMetricsRequest{Id: id})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// CreateRAGEvaluation creates a RAG evaluation
func (h *RealHarnessHandler) CreateRAGEvaluation(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	var req pb.CreateRAGEvaluationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.CreateRAGEvaluation(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// ListRAGEvaluations lists RAG evaluations
func (h *RealHarnessHandler) ListRAGEvaluations(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	status := c.Query("status")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.ListRAGEvaluations(ctx, &pb.ListRAGEvaluationsRequest{Status: status})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// RunRAGEvaluation runs a RAG evaluation
func (h *RealHarnessHandler) RunRAGEvaluation(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	resp, err := h.client.RunRAGEvaluation(ctx, &pb.RunRAGEvaluationRequest{EvaluationId: id})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// ==================== LLM Gateway ====================

// GatewayChat sends a chat request through the gateway
func (h *RealHarnessHandler) GatewayChat(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	var req pb.GatewayChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	resp, err := h.client.GatewayChat(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// CreateGatewayConfig creates a gateway provider config
func (h *RealHarnessHandler) CreateGatewayConfig(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	var req pb.CreateGatewayConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.CreateGatewayConfig(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// ListGatewayConfigs lists gateway provider configs
func (h *RealHarnessHandler) ListGatewayConfigs(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.ListGatewayConfigs(ctx, &pb.ListGatewayConfigsRequest{})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// GetGatewayConfig gets a gateway provider config
func (h *RealHarnessHandler) GetGatewayConfig(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.GetGatewayConfig(ctx, &pb.GetGatewayConfigRequest{Id: id})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// UpdateGatewayConfig updates a gateway provider config
func (h *RealHarnessHandler) UpdateGatewayConfig(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	var req pb.UpdateGatewayConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}
	// Set ID from URL param - the JSON body may not include it
	req.Id = c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.UpdateGatewayConfig(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// DeleteGatewayConfig deletes a gateway provider config
func (h *RealHarnessHandler) DeleteGatewayConfig(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := h.client.DeleteGatewayConfig(ctx, &pb.DeleteGatewayConfigRequest{Id: id})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": gin.H{"deleted": true}})
}

// GetGatewayStats gets gateway statistics
func (h *RealHarnessHandler) GetGatewayStats(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.GetGatewayStats(ctx, &common.Empty{})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// ListGatewayRoutes lists gateway routes
func (h *RealHarnessHandler) ListGatewayRoutes(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.ListGatewayRoutes(ctx, &pb.ListGatewayRoutesRequest{})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// CreateGatewayRoute creates a gateway route
func (h *RealHarnessHandler) CreateGatewayRoute(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	var req pb.CreateGatewayRouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.CreateGatewayRoute(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// DeleteGatewayRoute deletes a gateway route
func (h *RealHarnessHandler) DeleteGatewayRoute(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := h.client.DeleteGatewayRoute(ctx, &pb.DeleteGatewayRouteRequest{Id: id})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": gin.H{"deleted": true}})
}

// SetLoadBalanceStrategy sets the load balance strategy
func (h *RealHarnessHandler) SetLoadBalanceStrategy(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	var req pb.SetLoadBalanceStrategyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := h.client.SetLoadBalanceStrategy(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": gin.H{"success": true}})
}

// ==================== Playground ====================

// ExecutePlayground executes a playground request
func (h *RealHarnessHandler) ExecutePlayground(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	var req pb.PlaygroundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	resp, err := h.client.ExecutePlayground(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// CompareModels compares multiple models
func (h *RealHarnessHandler) CompareModels(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	var req pb.CompareModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	resp, err := h.client.CompareModels(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// StreamPlayground handles streaming playground execution via SSE
func (h *RealHarnessHandler) StreamPlayground(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	var req pb.PlaygroundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}
	req.TenantId = c.GetHeader("X-Tenant-ID")

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	stream, err := h.client.StreamPlayground(ctx, &req)
	if err != nil {
		c.SSEvent("error", gin.H{"error": err.Error()})
		c.Writer.Flush()
		return
	}

	for {
		resp, err := stream.Recv()
		if err != nil {
			break
		}
		c.SSEvent("message", gin.H{
			"content":    resp.Content,
			"done":       resp.Done,
			"error":      resp.Error,
			"log_id":     resp.LogId,
			"created_at": resp.CreatedAt,
		})
		c.Writer.Flush()
	}
}

// GetPlaygroundHistory gets playground execution history
func (h *RealHarnessHandler) GetPlaygroundHistory(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.GetPlaygroundHistory(ctx, &pb.GetPlaygroundHistoryRequest{Limit: int32(limit)})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}

// GetPlaygroundStats gets playground statistics
func (h *RealHarnessHandler) GetPlaygroundStats(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := h.client.GetPlaygroundStats(ctx, &pb.GetPlaygroundStatsRequest{})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": resp})
}
