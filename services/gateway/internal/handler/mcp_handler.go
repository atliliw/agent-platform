// Package handler provides HTTP handlers for Gateway
package handler

import (
	"context"
	"encoding/json"
	"time"

	"agent-platform/pkg/config"
	pb "agent-platform/pkg/pb/mcp"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// MCPHandler handles MCP requests
type MCPHandler struct {
	cfg       *config.Config
	mcpClient pb.MCPServiceClient
	conn      *grpc.ClientConn
}

// NewMCPHandler creates a new MCP handler
func NewMCPHandler(cfg *config.Config) *MCPHandler {
	// Create gRPC connection to MCP service
	if cfg.Services.MCP == "" {
		return &MCPHandler{cfg: cfg}
	}

	// 创建带超时的 gRPC 连接
	conn, err := grpc.Dial(cfg.Services.MCP,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(10*1024*1024)), // 10MB
	)
	if err != nil {
		return &MCPHandler{cfg: cfg}
	}

	return &MCPHandler{
		cfg:       cfg,
		mcpClient: pb.NewMCPServiceClient(conn),
		conn:      conn,
	}
}

// Close closes the gRPC connection
func (h *MCPHandler) Close() {
	if h.conn != nil {
		h.conn.Close()
	}
}

// ListTools lists tools from MCP service
func (h *MCPHandler) ListTools(c *gin.Context) {
	if h.mcpClient == nil {
		c.JSON(200, gin.H{"code": 0, "data": gin.H{"tools": []interface{}{}}})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.mcpClient.ListTools(ctx, &pb.ListToolsRequest{})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"tools": resp.Tools,
		},
	})
}

// CallTool calls a tool via MCP service
func (h *MCPHandler) CallTool(c *gin.Context) {
	var req struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "error": "invalid request"})
		return
	}

	if h.mcpClient == nil {
		c.JSON(200, gin.H{"code": 0, "data": gin.H{"is_error": true, "content": "MCP service not connected"}})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second) // 5 分钟，支持 Browser Agent
		defer cancel()

		argsJSON, _ := json.Marshal(req.Arguments)

	resp, err := h.mcpClient.CallTool(ctx, &pb.CallToolRequest{
		Name:      req.Name,
		Arguments: string(argsJSON),
	})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"is_error": resp.IsError,
			"content":  resp.Content,
		},
	})
}

// Connect connects to an MCP server
func (h *MCPHandler) Connect(c *gin.Context) {
	var req struct {
		Name    string            `json:"name"`
		Type    string            `json:"type"`
		Command string            `json:"command"`
		URL     string            `json:"url"`
		Env     map[string]string `json:"env"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "error": "invalid request"})
		return
	}

	if h.mcpClient == nil {
		c.JSON(200, gin.H{"code": 0, "data": gin.H{"connection": nil}})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.mcpClient.Connect(ctx, &pb.ConnectRequest{
		Name:    req.Name,
		Type:    req.Type,
		Command: req.Command,
		Url:     req.URL,
		Env:     req.Env,
	})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"connection": resp.Connection,
		},
	})
}

// ListConnections lists MCP connections
func (h *MCPHandler) ListConnections(c *gin.Context) {
	if h.mcpClient == nil {
		c.JSON(200, gin.H{"code": 0, "data": gin.H{"connections": []interface{}{}}})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.mcpClient.ListConnections(ctx, &pb.ListConnectionsRequest{})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"connections": resp.Connections,
		},
	})
}