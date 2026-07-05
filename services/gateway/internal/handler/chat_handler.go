// Package handler provides HTTP handlers for Gateway
package handler

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"agent-platform/pkg/config"
	pb "agent-platform/pkg/pb/chat"
	commonpb "agent-platform/pkg/pb/common"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ChatHandler handles chat requests
type ChatHandler struct {
	cfg        *config.Config
	chatClient pb.ChatServiceClient
	conn       *grpc.ClientConn
}

// NewChatHandler creates a new chat handler
func NewChatHandler(cfg *config.Config) *ChatHandler {
	// Create gRPC connection to chat service
	conn, err := grpc.Dial(cfg.Services.Chat, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	return &ChatHandler{
		cfg:        cfg,
		chatClient: pb.NewChatServiceClient(conn),
		conn:       conn,
	}
}

// Close closes the gRPC connection
func (h *ChatHandler) Close() {
	if h.conn != nil {
		h.conn.Close()
	}
}

// Chat handles chat request
func (h *ChatHandler) Chat(c *gin.Context) {
	var req struct {
		SessionID    string `json:"session_id"`
		Message      string `json:"message"`
		Model        string `json:"model"`
		SystemPrompt string `json:"system_prompt"`
		TenantID     string `json:"tenant_id"`
		UserID       string `json:"user_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "error": "invalid request"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second) // 10 分钟超时，支持 MultiAgent 浏览器操作
	defer cancel()

	resp, err := h.chatClient.Chat(ctx, &pb.ChatRequest{
		SessionId:    req.SessionID,
		Message:      req.Message,
		Model:        req.Model,
		SystemPrompt: req.SystemPrompt,
		TenantId:     req.TenantID,
		UserId:       req.UserID,
	})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"session_id":    resp.SessionId,
			"content":       resp.Content,
			"total_tokens":  resp.TotalTokens,
			"cost":          resp.Cost,
			"agent_states":  resp.AgentStates,
			"tool_calls":    resp.ToolCalls,
		},
	})
}

// ChatStream handles streaming chat
func (h *ChatHandler) ChatStream(c *gin.Context) {
	var req struct {
		SessionID    string `json:"session_id"`
		Message      string `json:"message"`
		Model        string `json:"model"`
		SystemPrompt string `json:"system_prompt"`
		TenantID     string `json:"tenant_id"`
		UserID       string `json:"user_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "error": "invalid request"})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	stream, err := h.chatClient.ChatStream(ctx, &pb.ChatRequest{
		SessionId:    req.SessionID,
		Message:      req.Message,
		Model:        req.Model,
		SystemPrompt: req.SystemPrompt,
		TenantId:     req.TenantID,
		UserId:       req.UserID,
		Stream:       true,
	})
	if err != nil {
		c.SSEvent("error", gin.H{"error": err.Error()})
		return
	}

	for {
		resp, err := stream.Recv()
		if err != nil {
			break
		}
		c.SSEvent("message", gin.H{
			"type":    resp.Type,
			"content": resp.Content,
		})
	}
}

// ListSessions lists sessions
func (h *ChatHandler) ListSessions(c *gin.Context) {
	tenantID := c.Query("tenant_id")
	userID := c.Query("user_id")

	// Parse pagination from query params
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.chatClient.ListSessions(ctx, &pb.ListSessionsRequest{
		TenantId: tenantID,
		UserId:   userID,
		Pagination: &commonpb.PaginationRequest{
			Page:     int32(page),
			PageSize: int32(pageSize),
		},
	})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "error": err.Error()})
		return
	}

	// Convert protobuf sessions to maps and inject message_count
	sessionMaps := make([]map[string]interface{}, 0, len(resp.Sessions))
	for _, s := range resp.Sessions {
		// Marshal to JSON then unmarshal to map for manipulation
		raw, err := json.Marshal(s)
		if err != nil {
			continue
		}
		var m map[string]interface{}
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		// Compute message_count from messages array length
		msgs, _ := m["messages"].([]interface{})
		msgCount := 0
		if msgs != nil {
			msgCount = len(msgs)
		}
		m["message_count"] = msgCount
		sessionMaps = append(sessionMaps, m)
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"sessions":   sessionMaps,
			"pagination": resp.Pagination,
		},
	})
}

// GetSession gets a session
func (h *ChatHandler) GetSession(c *gin.Context) {
	sessionID := c.Param("id")
	tenantID := c.Query("tenant_id")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.chatClient.GetSession(ctx, &pb.GetSessionRequest{
		Id:       sessionID,
		TenantId: tenantID,
	})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"code": 0, "data": gin.H{"session": resp}})
}

// DeleteSession deletes a session
func (h *ChatHandler) DeleteSession(c *gin.Context) {
	sessionID := c.Param("id")
	tenantID := c.Query("tenant_id")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := h.chatClient.DeleteSession(ctx, &pb.DeleteSessionRequest{
		Id:       sessionID,
		TenantId: tenantID,
	})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"code": 0, "message": "deleted"})
}

// DeleteEmptySessions deletes all sessions that have no messages
func (h *ChatHandler) DeleteEmptySessions(c *gin.Context) {
	tenantID := c.Query("tenant_id")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Fetch all sessions (paginate through all pages)
	var emptySessionIDs []string
	page := int32(1)
	pageSize := int32(100)

	for {
		resp, err := h.chatClient.ListSessions(ctx, &pb.ListSessionsRequest{
			TenantId: tenantID,
			Pagination: &commonpb.PaginationRequest{
				Page:     page,
				PageSize: pageSize,
			},
		})
		if err != nil {
			c.JSON(500, gin.H{"code": 500, "error": err.Error()})
			return
		}

		// Check each session for messages by fetching full session
		for _, s := range resp.Sessions {
			detail, err := h.chatClient.GetSession(ctx, &pb.GetSessionRequest{
				Id:       s.Id,
				TenantId: tenantID,
			})
			if err != nil {
				continue
			}
			if len(detail.Messages) == 0 {
				emptySessionIDs = append(emptySessionIDs, s.Id)
			}
		}

		if resp.Pagination == nil || page*pageSize >= resp.Pagination.Total {
			break
		}
		page++
	}

	// Delete each empty session
	deleted := 0
	failed := 0
	for _, id := range emptySessionIDs {
		_, err := h.chatClient.DeleteSession(ctx, &pb.DeleteSessionRequest{
			Id:       id,
			TenantId: tenantID,
		})
		if err != nil {
			failed++
		} else {
			deleted++
		}
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"deleted": deleted,
			"failed":  failed,
			"total":   len(emptySessionIDs),
		},
	})
}

// MultiAgentChat handles multi-agent chat
func (h *ChatHandler) MultiAgentChat(c *gin.Context) {
	var req struct {
		SessionID   string `json:"session_id"`
		Message     string `json:"message"`
		MasterAgent string `json:"master_agent"`
		TenantID    string `json:"tenant_id"`
		UserID      string `json:"user_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "error": "invalid request"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	resp, err := h.chatClient.MultiAgentChat(ctx, &pb.MultiAgentRequest{
		SessionId:   req.SessionID,
		Message:     req.Message,
		MasterAgent: req.MasterAgent,
		TenantId:    req.TenantID,
		UserId:      req.UserID,
	})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"session_id":   resp.SessionId,
			"final_answer": resp.FinalAnswer,
			"steps":        resp.Steps,
			"total_tokens": resp.TotalTokens,
		},
	})
}