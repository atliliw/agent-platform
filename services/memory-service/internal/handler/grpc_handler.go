// Package handler provides gRPC handlers for memory service
package handler

import (
	"context"

	pb "agent-platform/pkg/pb/memory"
	commonpb "agent-platform/pkg/pb/common"
	"agent-platform/services/memory-service/internal/service"
)

// MemoryHandler implements MemoryServiceServer
type MemoryHandler struct {
	pb.UnimplementedMemoryServiceServer
	service *service.MemoryService
}

// NewMemoryHandler creates a new memory handler
func NewMemoryHandler(service *service.MemoryService) *MemoryHandler {
	return &MemoryHandler{
		service: service,
	}
}

// Save saves a memory
func (h *MemoryHandler) Save(ctx context.Context, req *pb.SaveMemoryRequest) (*pb.SaveMemoryResponse, error) {
	return h.service.Save(ctx, req)
}

// SaveBatch saves multiple memories
func (h *MemoryHandler) SaveBatch(ctx context.Context, req *pb.SaveMemoryBatchRequest) (*pb.SaveMemoryBatchResponse, error) {
	return h.service.SaveBatch(ctx, req)
}

// Recall recalls memories
func (h *MemoryHandler) Recall(ctx context.Context, req *pb.RecallMemoryRequest) (*pb.RecallMemoryResponse, error) {
	return h.service.Recall(ctx, req)
}

// GetSessionMemory gets session memories
func (h *MemoryHandler) GetSessionMemory(ctx context.Context, req *pb.GetSessionMemoryRequest) (*pb.RecallMemoryResponse, error) {
	return h.service.GetSessionMemory(ctx, req)
}

// DeleteSessionMemory deletes session memories
func (h *MemoryHandler) DeleteSessionMemory(ctx context.Context, req *pb.DeleteSessionMemoryRequest) (*commonpb.Empty, error) {
	return h.service.DeleteSessionMemory(ctx, req)
}

// GetAllMemories gets all memories for a tenant
func (h *MemoryHandler) GetAllMemories(ctx context.Context, req *pb.GetAllMemoriesRequest) (*pb.RecallMemoryResponse, error) {
	return h.service.GetAllMemories(ctx, req)
}

// DeleteMemory deletes a single memory by ID
func (h *MemoryHandler) DeleteMemory(ctx context.Context, req *pb.DeleteMemoryRequest) (*commonpb.Empty, error) {
	return h.service.DeleteMemory(ctx, req)
}