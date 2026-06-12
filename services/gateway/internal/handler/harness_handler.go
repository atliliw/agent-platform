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