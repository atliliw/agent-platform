// Package handler provides HTTP handlers for Gateway
package handler

import (
	"context"
	"time"

	"agent-platform/pkg/config"
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
	addr := "harness-service:50007"
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
			"run_id":               resp.RunId,
			"results":              results,
			"avg_score":            resp.AvgScore,
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
				"id":             resp.Id,
				"name":           resp.Name,
				"control_model":  resp.ControlModel,
				"variant_model":  resp.VariantModel,
				"traffic_split":  resp.TrafficSplit,
				"status":         resp.Status,
				"created_at":     resp.CreatedAt,
			},
		},
	})
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
				"control_score":  resp.ControlScore,
				"variant_score":  resp.VariantScore,
				"delta":          resp.Delta,
				"p_value":        resp.PValue,
				"significant":    resp.Significant,
				"recommended":    resp.Recommended,
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

	resp, err := h.Client.EvaluateFeatureFlag(ctx, &req)
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

	req := &pb.CostReportRequest{
		AgentId:   c.Query("agent_id"),
		StartTime: int64(c.GetFloat64("start_time")),
		EndTime:   int64(c.GetFloat64("end_time")),
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

	var req pb.ApproveProposalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.ApproveProposal(ctx, &req)
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

	var req pb.RejectProposalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": -1, "message": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.client.RejectProposal(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{"proposal": resp},
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