// Package handler provides HTTP handlers for Gateway
package handler

import (
	"context"
	"time"

	"agent-platform/pkg/config"
	pb "agent-platform/pkg/pb/agent"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AgentHandler handles agent requests
type AgentHandler struct {
	cfg         *config.Config
	agentClient pb.AgentServiceClient
	conn        *grpc.ClientConn
}

// NewAgentHandler creates a new agent handler
func NewAgentHandler(cfg *config.Config) *AgentHandler {
	// Create gRPC connection to agent service
	conn, err := grpc.Dial(cfg.Services.Agent, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	return &AgentHandler{
		cfg:         cfg,
		agentClient: pb.NewAgentServiceClient(conn),
		conn:        conn,
	}
}

// Close closes the gRPC connection
func (h *AgentHandler) Close() {
	if h.conn != nil {
		h.conn.Close()
	}
}

// RegisterAgent handles agent registration
func (h *AgentHandler) RegisterAgent(c *gin.Context) {
	var req struct {
		ID                string   `json:"id"`
		Name              string   `json:"name"`
		Description       string   `json:"description"`
		Instructions      string   `json:"instructions"`
		PromptTemplateKey string   `json:"prompt_template_key"`
		Tools             []string `json:"tools"`
		Handoffs          []string `json:"handoffs"`
		Skills            []string `json:"skills"`
		Model             string   `json:"model"`
		MaxTokens         int      `json:"max_tokens"`
		Temperature       float64  `json:"temperature"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "error": "invalid request"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.agentClient.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Agent: &pb.Agent{
			Id:                req.ID,
			Name:              req.Name,
			Description:       req.Description,
			Instructions:      req.Instructions,
			PromptTemplateKey: req.PromptTemplateKey,
			Tools:             req.Tools,
			Handoffs:          req.Handoffs,
			Skills:            req.Skills,
			Model:             req.Model,
			MaxTokens:         int32(req.MaxTokens),
			Temperature:       req.Temperature,
		},
	})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"agent": resp.Agent,
		},
	})
}

// UnregisterAgent handles agent unregistration
func (h *AgentHandler) UnregisterAgent(c *gin.Context) {
	agentID := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.agentClient.UnregisterAgent(ctx, &pb.UnregisterAgentRequest{
		AgentId: agentID,
	})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code":    0,
		"success": resp.Success,
	})
}

// GetAgent handles getting an agent
func (h *AgentHandler) GetAgent(c *gin.Context) {
	agentID := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.agentClient.GetAgent(ctx, &pb.GetAgentRequest{
		AgentId: agentID,
	})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"agent": resp.Agent,
		},
	})
}

// ListAgents handles listing agents
func (h *AgentHandler) ListAgents(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.agentClient.ListAgents(ctx, &pb.ListAgentsRequest{})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"agents":     resp.Agents,
			"pagination": resp.Pagination,
		},
	})
}

// Execute handles multi-agent execution
func (h *AgentHandler) Execute(c *gin.Context) {
	var req struct {
		SessionID       string            `json:"session_id"`
		TenantID        string            `json:"tenant_id"`
		UserID          string            `json:"user_id"`
		Message         string            `json:"message"`
		EntryAgent      string            `json:"entry_agent"`
		ContextVars     map[string]string `json:"context_vars"`
		Goal            string            `json:"goal"`
		SuccessCriteria string            `json:"success_criteria"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "error": "invalid request"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	resp, err := h.agentClient.Execute(ctx, &pb.ExecuteRequest{
		SessionId:       req.SessionID,
		TenantId:        req.TenantID,
		UserId:          req.UserID,
		Message:         req.Message,
		EntryAgent:      req.EntryAgent,
		ContextVars:     req.ContextVars,
		Goal:            req.Goal,
		SuccessCriteria: req.SuccessCriteria,
	})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"context_id":    resp.ContextId,
			"session_id":    resp.SessionId,
			"response":      resp.Response,
			"agent_history": resp.AgentHistory,
			"total_tokens":  resp.TotalTokens,
			"total_cost":    resp.TotalCost,
			"status":        resp.Status,
		},
	})
}

// ExecuteStream handles streaming execution
func (h *AgentHandler) ExecuteStream(c *gin.Context) {
	var req struct {
		SessionID       string            `json:"session_id"`
		TenantID        string            `json:"tenant_id"`
		UserID          string            `json:"user_id"`
		Message         string            `json:"message"`
		EntryAgent      string            `json:"entry_agent"`
		ContextVars     map[string]string `json:"context_vars"`
		Goal            string            `json:"goal"`
		SuccessCriteria string            `json:"success_criteria"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "error": "invalid request"})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	stream, err := h.agentClient.ExecuteStream(ctx, &pb.ExecuteStreamRequest{
		SessionId:       req.SessionID,
		TenantId:        req.TenantID,
		UserId:          req.UserID,
		Message:         req.Message,
		EntryAgent:      req.EntryAgent,
		ContextVars:     req.ContextVars,
		Goal:            req.Goal,
		SuccessCriteria: req.SuccessCriteria,
	})
	if err != nil {
		c.SSEvent("error", gin.H{"error": err.Error()})
		return
	}

	for {
		chunk, err := stream.Recv()
		if err != nil {
			break
		}

		c.SSEvent(chunk.Type, gin.H{
			"content":       chunk.Content,
			"step":          chunk.Step,
			"current_agent": chunk.CurrentAgent,
		})
	}
}

// GetContext handles getting execution context
func (h *AgentHandler) GetContext(c *gin.Context) {
	contextID := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.agentClient.GetContext(ctx, &pb.GetContextRequest{
		ContextId: contextID,
	})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"context": resp.Context,
		},
	})
}
