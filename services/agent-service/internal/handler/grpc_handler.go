// Package handler provides gRPC handlers for agent service
package handler

import (
	"context"

	pb "agent-platform/pkg/pb/agent"
	"agent-platform/services/agent-service/internal/service"
)

// AgentHandler implements AgentServiceServer
type AgentHandler struct {
	pb.UnimplementedAgentServiceServer
	service *service.AgentService
}

// NewAgentHandler creates a new agent handler
func NewAgentHandler(service *service.AgentService) *AgentHandler {
	return &AgentHandler{
		service: service,
	}
}

// RegisterAgent registers a new agent
func (h *AgentHandler) RegisterAgent(ctx context.Context, req *pb.RegisterAgentRequest) (*pb.RegisterAgentResponse, error) {
	return h.service.RegisterAgent(ctx, req)
}

// UnregisterAgent unregisters an agent
func (h *AgentHandler) UnregisterAgent(ctx context.Context, req *pb.UnregisterAgentRequest) (*pb.UnregisterAgentResponse, error) {
	return h.service.UnregisterAgent(ctx, req)
}

// GetAgent gets an agent by ID
func (h *AgentHandler) GetAgent(ctx context.Context, req *pb.GetAgentRequest) (*pb.GetAgentResponse, error) {
	return h.service.GetAgent(ctx, req)
}

// ListAgents lists all agents
func (h *AgentHandler) ListAgents(ctx context.Context, req *pb.ListAgentsRequest) (*pb.ListAgentsResponse, error) {
	return h.service.ListAgents(ctx, req)
}

// Execute executes a multi-agent workflow
func (h *AgentHandler) Execute(ctx context.Context, req *pb.ExecuteRequest) (*pb.ExecuteResponse, error) {
	return h.service.Execute(ctx, req)
}

// ExecuteStream executes with streaming
func (h *AgentHandler) ExecuteStream(req *pb.ExecuteStreamRequest, stream pb.AgentService_ExecuteStreamServer) error {
	return h.service.ExecuteStream(req, stream)
}

// GetContext gets an execution context
func (h *AgentHandler) GetContext(ctx context.Context, req *pb.GetContextRequest) (*pb.GetContextResponse, error) {
	return h.service.GetContext(ctx, req)
}
