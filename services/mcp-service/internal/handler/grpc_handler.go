// Package handler provides gRPC handlers for MCP service
package handler

import (
	"context"

	pb "agent-platform/pkg/pb/mcp"
	commonpb "agent-platform/pkg/pb/common"
	"agent-platform/services/mcp-service/internal/service"
)

// MCPHandler implements MCPServiceServer
type MCPHandler struct {
	pb.UnimplementedMCPServiceServer
	service *service.MCPService
}

// NewMCPHandler creates a new MCP handler
func NewMCPHandler(service *service.MCPService) *MCPHandler {
	return &MCPHandler{
		service: service,
	}
}

// Connect connects to an MCP server
func (h *MCPHandler) Connect(ctx context.Context, req *pb.ConnectRequest) (*pb.ConnectResponse, error) {
	return h.service.Connect(ctx, req)
}

// Disconnect disconnects from an MCP server
func (h *MCPHandler) Disconnect(ctx context.Context, req *pb.DisconnectRequest) (*commonpb.Empty, error) {
	return h.service.Disconnect(ctx, req)
}

// ListConnections lists connections
func (h *MCPHandler) ListConnections(ctx context.Context, req *pb.ListConnectionsRequest) (*pb.ListConnectionsResponse, error) {
	return h.service.ListConnections(ctx, req)
}

// ListTools lists available tools
func (h *MCPHandler) ListTools(ctx context.Context, req *pb.ListToolsRequest) (*pb.ListToolsResponse, error) {
	return h.service.ListTools(ctx, req)
}

// CallTool calls a tool
func (h *MCPHandler) CallTool(ctx context.Context, req *pb.CallToolRequest) (*pb.CallToolResponse, error) {
	return h.service.CallTool(ctx, req)
}

// ListResources lists resources
func (h *MCPHandler) ListResources(ctx context.Context, req *pb.ListResourcesRequest) (*pb.ListResourcesResponse, error) {
	return h.service.ListResources(ctx, req)
}

// ReadResource reads a resource
func (h *MCPHandler) ReadResource(ctx context.Context, req *pb.ReadResourceRequest) (*pb.ReadResourceResponse, error) {
	return h.service.ReadResource(ctx, req)
}

// ListPrompts lists prompts
func (h *MCPHandler) ListPrompts(ctx context.Context, req *pb.ListPromptsRequest) (*pb.ListPromptsResponse, error) {
	return h.service.ListPrompts(ctx, req)
}

// GetPrompt gets a prompt
func (h *MCPHandler) GetPrompt(ctx context.Context, req *pb.GetPromptRequest) (*pb.GetPromptResponse, error) {
	return h.service.GetPrompt(ctx, req)
}