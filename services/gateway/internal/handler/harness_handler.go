// Package handler provides HTTP handlers for Gateway
package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"agent-platform/pkg/agent/intervention"
	"agent-platform/pkg/config"
	common "agent-platform/pkg/pb/common"
	pb "agent-platform/pkg/pb/harness"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// RealHarnessHandler handles Harness requests with real gRPC calls
type RealHarnessHandler struct {
	cfg                *config.Config
	client             pb.HarnessServiceClient
	conn               *grpc.ClientConn
	interventionManager *intervention.InterventionManager
}

// NewRealHarnessHandler creates a new harness handler with gRPC connection
func NewRealHarnessHandler(cfg *config.Config) *RealHarnessHandler {
	// Connect to harness-service
	addr := cfg.Services.Harness
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		// Return stub handler if connection fails
		return &RealHarnessHandler{
			cfg:                cfg,
			client:             nil,
			conn:               nil,
			interventionManager: intervention.NewInterventionManager(),
		}
	}

	return &RealHarnessHandler{
		cfg:                cfg,
		client:             pb.NewHarnessServiceClient(conn),
		conn:               conn,
		interventionManager: intervention.NewInterventionManager(),
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
	_, err := h.client.DeleteSession(ctx, &pb.GetSessionRequest{SessionId: id})
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
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
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

// ==================== Approval Handlers ====================
// These proxy to the agent-service HTTP API on port 50009

// approvalProxy forwards a request to the agent-service approval API
func (h *RealHarnessHandler) approvalProxy(c *gin.Context, path string) {
	baseURL := "http://agent-service:50009"
	url := baseURL + path

	// Copy query params
	if c.Request.URL.RawQuery != "" {
		url += "?" + c.Request.URL.RawQuery
	}

	var req *http.Request
	var err error

	if c.Request.Method == "GET" {
		req, err = http.NewRequestWithContext(c.Request.Context(), "GET", url, nil)
	} else {
		req, err = http.NewRequestWithContext(c.Request.Context(), c.Request.Method, url, c.Request.Body)
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
		}
	}

	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(502, gin.H{"code": -1, "message": "agent-service approval API unavailable: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, "application/json", body)
}

// GetPendingApprovals lists pending approval requests
func (h *RealHarnessHandler) GetPendingApprovals(c *gin.Context) {
	h.approvalProxy(c, "/approval/pending")
}

// GetApprovalRules lists approval rules
func (h *RealHarnessHandler) GetApprovalRules(c *gin.Context) {
	h.approvalProxy(c, "/approval/rules")
}

// ApproveRequest approves a pending request
func (h *RealHarnessHandler) ApproveRequest(c *gin.Context) {
	h.approvalProxy(c, "/approval/approve")
}

// RejectRequest rejects a pending request
func (h *RealHarnessHandler) RejectRequest(c *gin.Context) {
	h.approvalProxy(c, "/approval/reject")
}

// AddApprovalRule adds a new approval rule
func (h *RealHarnessHandler) AddApprovalRule(c *gin.Context) {
	h.approvalProxy(c, "/approval/rules/add")
}

// ==================== Checkpoint Handlers ====================

// ListCheckpoints lists checkpoints for a session
func (h *RealHarnessHandler) ListCheckpoints(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	sessionID := c.Param("sessionId")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	checkpoints, err := h.client.ListCheckpoints(ctx, &pb.ListCheckpointsRequest{
		SessionId: sessionID,
	})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"checkpoints": checkpoints.Checkpoints},
	})
}

// GetCheckpoint gets a specific checkpoint
func (h *RealHarnessHandler) GetCheckpoint(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	checkpointID := c.Param("checkpointId")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.GetCheckpoint(ctx, &pb.GetCheckpointRequest{
		CheckpointId: checkpointID,
	})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"checkpoint": resp},
	})
}

// ResumeFromCheckpoint resumes execution from a checkpoint
func (h *RealHarnessHandler) ResumeFromCheckpoint(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	checkpointID := c.Param("checkpointId")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := h.client.ResumeFromCheckpoint(ctx, &pb.ResumeFromCheckpointRequest{
		CheckpointId: checkpointID,
	})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"context_id":    resp.ContextId,
			"session_id":    resp.SessionId,
			"response":      resp.Response,
			"total_tokens":  resp.TotalTokens,
			"total_cost":    resp.TotalCost,
			"status":        resp.Status,
			"agent_history": resp.AgentHistory,
		},
	})
}

// ==================== Workflow Handlers ====================

// CreateWorkflow creates a new workflow
func (h *RealHarnessHandler) CreateWorkflow(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req struct {
		Name        string                   `json:"name"`
		Description string                   `json:"description"`
		Nodes       []map[string]interface{} `json:"nodes"`
		Edges       []map[string]interface{} `json:"edges"`
		EntryNodeID string                   `json:"entry_node_id"`
		TenantID    string                   `json:"tenant_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	nodesJSON, _ := json.Marshal(req.Nodes)
	edgesJSON, _ := json.Marshal(req.Edges)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.CreateWorkflow(ctx, &pb.CreateWorkflowRequest{
		Name:        req.Name,
		Description: req.Description,
		Nodes:       string(nodesJSON),
		Edges:       string(edgesJSON),
		EntryNodeId: req.EntryNodeID,
		TenantId:    req.TenantID,
	})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"id":             resp.Id,
			"name":           resp.Name,
			"description":    resp.Description,
			"nodes":          resp.Nodes,
			"edges":          resp.Edges,
			"entry_node_id":  resp.EntryNodeId,
			"tenant_id":      resp.TenantId,
			"created_at":     resp.CreatedAt,
			"updated_at":     resp.UpdatedAt,
		},
	})
}

// GetWorkflow retrieves a workflow by ID
func (h *RealHarnessHandler) GetWorkflow(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	id := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.GetWorkflow(ctx, &pb.GetWorkflowRequest{
		Id: id,
	})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"id":            resp.Id,
			"name":          resp.Name,
			"description":   resp.Description,
			"nodes":         resp.Nodes,
			"edges":         resp.Edges,
			"entry_node_id": resp.EntryNodeId,
			"tenant_id":     resp.TenantId,
			"created_at":    resp.CreatedAt,
			"updated_at":    resp.UpdatedAt,
		},
	})
}

// ListWorkflows lists all workflows
func (h *RealHarnessHandler) ListWorkflows(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	tenantID := c.Query("tenant_id")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.ListWorkflows(ctx, &pb.ListWorkflowsRequest{
		TenantId: tenantID,
	})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	workflows := make([]gin.H, 0, len(resp.Workflows))
	for _, wf := range resp.Workflows {
		workflows = append(workflows, gin.H{
			"id":            wf.Id,
			"name":          wf.Name,
			"description":   wf.Description,
			"nodes":         wf.Nodes,
			"edges":         wf.Edges,
			"entry_node_id": wf.EntryNodeId,
			"tenant_id":     wf.TenantId,
			"created_at":    wf.CreatedAt,
			"updated_at":    wf.UpdatedAt,
		})
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"workflows": workflows},
	})
}

// DeleteWorkflow deletes a workflow
func (h *RealHarnessHandler) DeleteWorkflow(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	id := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := h.client.DeleteWorkflow(ctx, &pb.DeleteWorkflowRequest{
		Id: id,
	})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{"code": 0, "data": nil})
}

// ExecuteWorkflow executes a workflow
func (h *RealHarnessHandler) ExecuteWorkflow(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	id := c.Param("id")
	var req struct {
		Input           string `json:"input"`
		TimeoutSeconds  int32  `json:"timeout_seconds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	resp, err := h.client.ExecuteWorkflow(ctx, &pb.ExecuteWorkflowRequest{
		Id:             id,
		Input:          req.Input,
		TimeoutSeconds: req.TimeoutSeconds,
	})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	nodes := make([]gin.H, 0, len(resp.Nodes))
	for _, n := range resp.Nodes {
		nodeH := gin.H{
			"node_id": n.NodeId,
			"output":  n.Output,
			"error":   n.Error,
		}
		if n.NodeName != "" {
			nodeH["node_name"] = n.NodeName
		}
		if n.NodeType != "" {
			nodeH["node_type"] = n.NodeType
		}
		if n.DurationMs > 0 {
			nodeH["duration_ms"] = n.DurationMs
		}
		nodes = append(nodes, nodeH)
	}

	data := gin.H{
		"workflow_id":  resp.WorkflowId,
		"nodes":        nodes,
		"final_output": resp.FinalOutput,
		"error":        resp.Error,
	}
	if resp.ExecutionId != "" {
		data["execution_id"] = resp.ExecutionId
	}
	if resp.Status != "" {
		data["status"] = resp.Status
	}

	c.JSON(200, gin.H{"code": 0, "data": data})
}

// UpdateWorkflow updates an existing workflow
func (h *RealHarnessHandler) UpdateWorkflow(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	id := c.Param("id")
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Nodes       string `json:"nodes"`
		Edges       string `json:"edges"`
		EntryNodeID string `json:"entry_node_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.UpdateWorkflow(ctx, &pb.UpdateWorkflowRequest{
		Id:          id,
		Name:        req.Name,
		Description: req.Description,
		Nodes:       req.Nodes,
		Edges:       req.Edges,
		EntryNodeId: req.EntryNodeID,
	})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"id":            resp.Id,
			"name":          resp.Name,
			"description":   resp.Description,
			"nodes":         resp.Nodes,
			"edges":         resp.Edges,
			"entry_node_id": resp.EntryNodeId,
			"updated_at":    resp.UpdatedAt,
		},
	})
}

// ValidateWorkflow validates a workflow's DAG structure
func (h *RealHarnessHandler) ValidateWorkflow(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	var req struct {
		Nodes       string `json:"nodes"`
		Edges       string `json:"edges"`
		EntryNodeID string `json:"entry_node_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.ValidateWorkflow(ctx, &pb.ValidateWorkflowRequest{
		Nodes:       req.Nodes,
		Edges:       req.Edges,
		EntryNodeId: req.EntryNodeID,
	})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"valid": resp.Valid,
			"errors": resp.Errors,
		},
	})
}

// GetWorkflowExecution retrieves a workflow execution by ID
func (h *RealHarnessHandler) GetWorkflowExecution(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	executionID := c.Param("executionId")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.GetWorkflowExecution(ctx, &pb.GetWorkflowExecutionRequest{ExecutionId: executionID})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{"code": 0, "data": executionToJSON(resp)})
}

// ListWorkflowExecutions lists executions for a workflow
func (h *RealHarnessHandler) ListWorkflowExecutions(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	workflowID := c.Param("id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.ListWorkflowExecutions(ctx, &pb.ListWorkflowExecutionsRequest{
		WorkflowId: workflowID,
		Limit:      int32(limit),
	})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	executions := make([]gin.H, 0, len(resp.Executions))
	for _, exec := range resp.Executions {
		executions = append(executions, executionToJSON(exec))
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"executions": executions},
	})
}

// CancelWorkflowExecution cancels a running workflow execution
func (h *RealHarnessHandler) CancelWorkflowExecution(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": -1, "message": "harness service not available"})
		return
	}

	executionID := c.Param("executionId")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := h.client.CancelWorkflowExecution(ctx, &pb.CancelWorkflowExecutionRequest{ExecutionId: executionID})
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{"code": 0, "data": nil})
}

// executionToJSON converts a pb.WorkflowExecution to a gin.H map
func executionToJSON(exec *pb.WorkflowExecution) gin.H {
	nodeResults := make([]gin.H, 0, len(exec.NodeResults))
	for _, nr := range exec.NodeResults {
		nodeResults = append(nodeResults, gin.H{
			"node_id":  nr.NodeId,
			"output":   nr.Output,
			"error":    nr.Error,
			"node_type": nr.NodeType,
		})
	}

	return gin.H{
		"id":            exec.Id,
		"workflow_id":   exec.WorkflowId,
		"status":        exec.Status,
		"input":         exec.Input,
		"final_output":  exec.FinalOutput,
		"error":         exec.Error,
		"node_results":  nodeResults,
		"started_at":    exec.StartedAt,
		"completed_at":  exec.CompletedAt,
		"duration_ms":   exec.DurationMs,
	}
}


// ==================== Intervention Handlers ====================

// InterveneSession handles pause/stop/modify/inject interventions on a session
func (h *RealHarnessHandler) InterveneSession(c *gin.Context) {
	sessionID := c.Param("sessionId")

	var req struct {
		Type       string                 `json:"type"`                 // pause, stop, modify, inject
		Reason     string                 `json:"reason,omitempty"`
		UserID     string                 `json:"user_id,omitempty"`
		Parameters map[string]interface{} `json:"parameters,omitempty"` // For modify
		Message    string                 `json:"message,omitempty"`    // For inject
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch intervention.InterventionType(req.Type) {
	case intervention.InterventionPause:
		if err := h.interventionManager.PauseSession(ctx, sessionID, req.UserID, req.Reason); err != nil {
			c.JSON(500, gin.H{"code": -1, "message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"code": 0, "data": gin.H{"status": "paused", "session_id": sessionID}})

	case intervention.InterventionStop:
		if err := h.interventionManager.StopSession(ctx, sessionID, req.UserID, req.Reason); err != nil {
			c.JSON(500, gin.H{"code": -1, "message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"code": 0, "data": gin.H{"status": "stopped", "session_id": sessionID}})

	case intervention.InterventionModify:
		if req.Parameters == nil {
			c.JSON(400, gin.H{"code": -1, "message": "parameters required for modify intervention"})
			return
		}
		if err := h.interventionManager.ModifyParameters(ctx, sessionID, req.Parameters); err != nil {
			c.JSON(500, gin.H{"code": -1, "message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"code": 0, "data": gin.H{"status": "modified", "session_id": sessionID}})

	case intervention.InterventionInject:
		if req.Message == "" {
			c.JSON(400, gin.H{"code": -1, "message": "message required for inject intervention"})
			return
		}
		if err := h.interventionManager.InjectMessage(ctx, sessionID, req.Message); err != nil {
			c.JSON(500, gin.H{"code": -1, "message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"code": 0, "data": gin.H{"status": "injected", "session_id": sessionID}})

	default:
		c.JSON(400, gin.H{"code": -1, "message": "invalid intervention type: " + req.Type})
	}
}

// GetSessionState gets the running state of a session
func (h *RealHarnessHandler) GetSessionState(c *gin.Context) {
	sessionID := c.Param("sessionId")

	state, err := h.interventionManager.GetSessionState(sessionID)
	if err != nil {
		c.JSON(404, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"session_id":    state.SessionID,
			"agent_id":      state.AgentID,
			"status":        state.Status,
			"current_step":  state.CurrentStep,
			"total_steps":   state.TotalSteps,
			"start_time":    state.StartTime,
			"pause_time":    state.PauseTime,
			"resume_time":   state.ResumeTime,
			"variables":     state.Variables,
			"execution_log": state.ExecutionLog,
			"last_update":   state.LastUpdate,
		},
	})
}

// ResumeSession resumes a paused session
func (h *RealHarnessHandler) ResumeSession(c *gin.Context) {
	sessionID := c.Param("sessionId")

	var req struct {
		UserID string `json:"user_id,omitempty"`
	}
	// Body is optional for resume
	c.ShouldBindJSON(&req)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := h.interventionManager.ResumeSession(ctx, sessionID, req.UserID); err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"status":     "running",
			"session_id": sessionID,
		},
	})
}

// InjectMessage injects a message into a running session
func (h *RealHarnessHandler) InjectMessage(c *gin.Context) {
	sessionID := c.Param("sessionId")

	var req struct {
		Message string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	if req.Message == "" {
		c.JSON(400, gin.H{"code": -1, "message": "message is required"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := h.interventionManager.InjectMessage(ctx, sessionID, req.Message); err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"status":     "injected",
			"session_id": sessionID,
		},
	})
}
