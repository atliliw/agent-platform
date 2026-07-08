// Package handler provides HTTP handlers for Gateway
package handler

import (
	"agent-platform/pkg/config"
	pb "agent-platform/pkg/pb/a2a"
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// RealA2AHandler proxies A2A requests to a2a-service via gRPC.
// When a2a-service is unavailable, it falls back to empty responses.
type RealA2AHandler struct {
	cfg    *config.Config
	conn   *grpc.ClientConn
	client pb.A2AServiceClient
}

// NewRealA2AHandler creates an A2A handler with a2a-service gRPC client
func NewRealA2AHandler(cfg *config.Config) *RealA2AHandler {
	h := &RealA2AHandler{cfg: cfg}

	if addr := cfg.Services.A2A; addr != "" {
		conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			fmt.Printf("[A2A] Failed to connect to a2a-service: %v\n", err)
			return h
		}
		h.conn = conn
		h.client = pb.NewA2AServiceClient(conn)
		fmt.Printf("[A2A] Connected to a2a-service at %s\n", addr)
	}

	return h
}

// Discover discovers agents by proxying to a2a-service
func (h *RealA2AHandler) Discover(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": 0, "data": gin.H{"card": nil}})
		return
	}

	var req struct {
		AgentURL string `json:"agent_url"`
		TenantID string `json:"tenant_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 1, "error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.client.Discover(ctx, &pb.DiscoverRequest{
		AgentUrl: req.AgentURL,
		TenantId: req.TenantID,
	})
	if err != nil {
		c.JSON(502, gin.H{"code": 1, "error": fmt.Sprintf("a2a-service unavailable: %v", err)})
		return
	}

	c.JSON(200, gin.H{"code": 0, "data": gin.H{"card": agentCardToMap(resp.Card)}})
}

// RegisterAgent registers an agent by proxying to a2a-service
func (h *RealA2AHandler) RegisterAgent(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": 0, "message": "registered (stub)"})
		return
	}

	var req struct {
		Card     *pb.AgentCard `json:"card"`
		TenantID string        `json:"tenant_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 1, "error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	_, err := h.client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Card:     req.Card,
		TenantId: req.TenantID,
	})
	if err != nil {
		c.JSON(502, gin.H{"code": 1, "error": fmt.Sprintf("a2a-service unavailable: %v", err)})
		return
	}

	c.JSON(200, gin.H{"code": 0, "message": "registered"})
}

// ListAgents lists agents by proxying to a2a-service
func (h *RealA2AHandler) ListAgents(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": 0, "data": gin.H{"agents": []interface{}{}}})
		return
	}

	tenantID := c.Query("tenant_id")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.client.ListAgents(ctx, &pb.ListAgentsRequest{
		TenantId: tenantID,
	})
	if err != nil {
		c.JSON(502, gin.H{"code": 1, "error": fmt.Sprintf("a2a-service unavailable: %v", err)})
		return
	}

	agents := make([]interface{}, 0, len(resp.Agents))
	for _, a := range resp.Agents {
		agents = append(agents, agentCardToMap(a))
	}

	c.JSON(200, gin.H{"code": 0, "data": gin.H{"agents": agents}})
}

// SendTask sends a task by proxying to a2a-service
func (h *RealA2AHandler) SendTask(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": 0, "data": gin.H{"task": nil}})
		return
	}

	var req struct {
		AgentID  string            `json:"agent_id"`
		Message  map[string]interface{} `json:"message"`
		Metadata map[string]string `json:"metadata"`
		TenantID string            `json:"tenant_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 1, "error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	grpcReq := &pb.SendTaskRequest{
		AgentId:  req.AgentID,
		TenantId: req.TenantID,
		Metadata: req.Metadata,
	}
	// Convert message JSON to A2AMessage proto
	if req.Message != nil {
		grpcReq.Message = &pb.A2AMessage{
			Role: getStringField(req.Message, "role"),
			Content: getStringField(req.Message, "content"),
		}
	}

	resp, err := h.client.SendTask(ctx, grpcReq)
	if err != nil {
		c.JSON(502, gin.H{"code": 1, "error": fmt.Sprintf("a2a-service unavailable: %v", err)})
		return
	}

	c.JSON(200, gin.H{"code": 0, "data": gin.H{"task": a2aTaskToMap(resp.Task)}})
}

// GetTask gets a task by proxying to a2a-service
func (h *RealA2AHandler) GetTask(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": 0, "data": gin.H{"task": nil}})
		return
	}

	taskID := c.Param("id")
	tenantID := c.Query("tenant_id")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.client.GetTask(ctx, &pb.GetTaskRequest{
		TaskId:   taskID,
		TenantId: tenantID,
	})
	if err != nil {
		c.JSON(502, gin.H{"code": 1, "error": fmt.Sprintf("a2a-service unavailable: %v", err)})
		return
	}

	c.JSON(200, gin.H{"code": 0, "data": gin.H{"task": a2aTaskToMap(resp.Task)}})
}

// UnregisterAgent unregisters an agent by proxying to a2a-service
func (h *RealA2AHandler) UnregisterAgent(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": 0, "message": "unregistered (stub)"})
		return
	}

	var req struct {
		AgentID  string `json:"agent_id"`
		TenantID string `json:"tenant_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 1, "error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	_, err := h.client.UnregisterAgent(ctx, &pb.UnregisterAgentRequest{
		AgentId:  req.AgentID,
		TenantId: req.TenantID,
	})
	if err != nil {
		c.JSON(502, gin.H{"code": 1, "error": fmt.Sprintf("a2a-service unavailable: %v", err)})
		return
	}

	c.JSON(200, gin.H{"code": 0, "message": "unregistered"})
}

// CancelTask cancels a task by proxying to a2a-service
func (h *RealA2AHandler) CancelTask(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": 0, "message": "cancelled (stub)"})
		return
	}

	taskID := c.Param("id")
	tenantID := c.Query("tenant_id")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	_, err := h.client.CancelTask(ctx, &pb.CancelTaskRequest{
		TaskId:   taskID,
		TenantId: tenantID,
	})
	if err != nil {
		c.JSON(502, gin.H{"code": 1, "error": fmt.Sprintf("a2a-service unavailable: %v", err)})
		return
	}

	c.JSON(200, gin.H{"code": 0, "message": "cancelled"})
}

// ListTasks lists tasks by proxying to a2a-service
func (h *RealA2AHandler) ListTasks(c *gin.Context) {
	if h.client == nil {
		c.JSON(200, gin.H{"code": 0, "data": gin.H{"tasks": []interface{}{}}})
		return
	}

	agentID := c.Query("agent_id")
	statusStr := c.Query("status")
	tenantID := c.Query("tenant_id")

	// Parse status string to TaskStatus enum
	var status pb.TaskStatus
	if v, ok := pb.TaskStatus_value[statusStr]; ok {
		status = pb.TaskStatus(v)
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.client.ListTasks(ctx, &pb.ListTasksRequest{
		AgentId:  agentID,
		Status:   status,
		TenantId: tenantID,
	})
	if err != nil {
		c.JSON(502, gin.H{"code": 1, "error": fmt.Sprintf("a2a-service unavailable: %v", err)})
		return
	}

	tasks := make([]interface{}, 0, len(resp.Tasks))
	for _, t := range resp.Tasks {
		tasks = append(tasks, a2aTaskToMap(t))
	}

	c.JSON(200, gin.H{"code": 0, "data": gin.H{"tasks": tasks}})
}

// Close closes the gRPC connection
func (h *RealA2AHandler) Close() {
	if h.conn != nil {
		h.conn.Close()
	}
}

// agentCardToMap converts an AgentCard proto to a JSON-friendly map
func agentCardToMap(card *pb.AgentCard) gin.H {
	if card == nil {
		return nil
	}
	return gin.H{
		"id":           card.Id,
		"name":         card.Name,
		"description":  card.Description,
		"capabilities": card.Capabilities,
		"input_modes":  card.InputModes,
		"output_modes": card.OutputModes,
		"url":          card.Url,
		"metadata":     card.Metadata,
	}
}

// a2aTaskToMap converts an A2ATask proto to a JSON-friendly map
func a2aTaskToMap(task *pb.A2ATask) gin.H {
	if task == nil {
		return nil
	}
	return gin.H{
		"id":         task.Id,
		"agent_id":   task.AgentId,
		"status":     task.Status.String(),
		"result":     task.Result,
		"metadata":   task.Metadata,
		"created_at": task.CreatedAt,
		"updated_at": task.UpdatedAt,
	}
}

// getStringField extracts a string field from a map[string]interface{}
func getStringField(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
