// Package handler provides gRPC handlers for A2A service
package handler

import (
	"context"

	pb "agent-platform/pkg/pb/a2a"
	commonpb "agent-platform/pkg/pb/common"
	"agent-platform/services/a2a-service/internal/service"
)

// A2AHandler implements A2AServiceServer
type A2AHandler struct {
	pb.UnimplementedA2AServiceServer
	service *service.A2AService
}

// NewA2AHandler creates a new A2A handler
func NewA2AHandler(service *service.A2AService) *A2AHandler {
	return &A2AHandler{
		service: service,
	}
}

// Discover discovers a remote agent
func (h *A2AHandler) Discover(ctx context.Context, req *pb.DiscoverRequest) (*pb.DiscoverResponse, error) {
	return h.service.Discover(ctx, req)
}

// RegisterAgent registers an agent
func (h *A2AHandler) RegisterAgent(ctx context.Context, req *pb.RegisterAgentRequest) (*commonpb.Empty, error) {
	return h.service.RegisterAgent(ctx, req)
}

// UnregisterAgent unregisters an agent
func (h *A2AHandler) UnregisterAgent(ctx context.Context, req *pb.UnregisterAgentRequest) (*commonpb.Empty, error) {
	return h.service.UnregisterAgent(ctx, req)
}

// ListAgents lists agents
func (h *A2AHandler) ListAgents(ctx context.Context, req *pb.ListAgentsRequest) (*pb.ListAgentsResponse, error) {
	return h.service.ListAgents(ctx, req)
}

// SendTask sends a task
func (h *A2AHandler) SendTask(ctx context.Context, req *pb.SendTaskRequest) (*pb.SendTaskResponse, error) {
	return h.service.SendTask(ctx, req)
}

// GetTask gets a task
func (h *A2AHandler) GetTask(ctx context.Context, req *pb.GetTaskRequest) (*pb.GetTaskResponse, error) {
	return h.service.GetTask(ctx, req)
}

// CancelTask cancels a task
func (h *A2AHandler) CancelTask(ctx context.Context, req *pb.CancelTaskRequest) (*commonpb.Empty, error) {
	return h.service.CancelTask(ctx, req)
}

// ListTasks lists tasks
func (h *A2AHandler) ListTasks(ctx context.Context, req *pb.ListTasksRequest) (*pb.ListTasksResponse, error) {
	return h.service.ListTasks(ctx, req)
}