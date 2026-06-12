// Package service provides business logic for memory service
package service

import (
	"context"
	"fmt"
	"time"

	"agent-platform/pkg/llm"
	pb "agent-platform/pkg/pb/memory"
	commonpb "agent-platform/pkg/pb/common"
	"agent-platform/services/memory-service/internal/repository"
)

// MemoryService provides memory management functionality
type MemoryService struct {
	pb.UnimplementedMemoryServiceServer
	llmClient llm.Client
	repo      *repository.MemoryRepository
}

// NewMemoryService creates a new memory service
func NewMemoryService(llmClient llm.Client, repo *repository.MemoryRepository) *MemoryService {
	return &MemoryService{
		llmClient: llmClient,
		repo:      repo,
	}
}

// Save saves a memory
func (s *MemoryService) Save(ctx context.Context, req *pb.SaveMemoryRequest) (*pb.SaveMemoryResponse, error) {
	// Generate embedding
	embedding, err := s.llmClient.Embed(ctx, req.Content)
	if err != nil {
		return nil, fmt.Errorf("generate embedding: %w", err)
	}

	// Debug log
	fmt.Printf("DEBUG: Generated embedding for content '%s', len=%d\n", req.Content, len(embedding))

	// Save to repository
	memory := &repository.Memory{
		SessionID:  req.SessionId,
		AgentID:    req.AgentId,
		Type:       req.Type.String(),
		Content:    req.Content,
		Importance: req.Importance,
		Vector:     embedding,
		TenantID:   req.TenantId,
		CreatedAt:  time.Now(),
	}

	if err := s.repo.Save(ctx, memory); err != nil {
		return nil, err
	}

	return &pb.SaveMemoryResponse{
		Id:         memory.ID,
		CreatedAt:  memory.CreatedAt.Unix(),
	}, nil
}

// SaveBatch saves multiple memories
func (s *MemoryService) SaveBatch(ctx context.Context, req *pb.SaveMemoryBatchRequest) (*pb.SaveMemoryBatchResponse, error) {
	var ids []string
	now := time.Now()

	for _, m := range req.Memories {
		embedding, err := s.llmClient.Embed(ctx, m.Content)
		if err != nil {
			return nil, fmt.Errorf("generate embedding: %w", err)
		}

		memory := &repository.Memory{
			SessionID:  m.SessionId,
			AgentID:    m.AgentId,
			Type:       m.Type.String(),
			Content:    m.Content,
			Importance: m.Importance,
			Vector:     embedding,
			TenantID:   req.TenantId,
			CreatedAt:  now,
		}

		if err := s.repo.Save(ctx, memory); err != nil {
			return nil, err
		}
		ids = append(ids, memory.ID)
	}

	return &pb.SaveMemoryBatchResponse{
		Ids:        ids,
		CreatedAt:  now.Unix(),
	}, nil
}

// Recall recalls memories
func (s *MemoryService) Recall(ctx context.Context, req *pb.RecallMemoryRequest) (*pb.RecallMemoryResponse, error) {
	// Generate query embedding
	queryEmbedding, err := s.llmClient.Embed(ctx, req.Query)
	if err != nil {
		return nil, fmt.Errorf("generate query embedding: %w", err)
	}

	topK := int(req.TopK)
	if topK == 0 {
		topK = 5
	}

	// Search
	memories, err := s.repo.Search(ctx, queryEmbedding, req.SessionId, req.AgentId, req.TenantId, topK)
	if err != nil {
		return nil, err
	}

	var pbMemories []*pb.MemoryEntry
	for _, m := range memories {
		pbMemories = append(pbMemories, &pb.MemoryEntry{
			Id:         m.ID,
			SessionId:  m.SessionID,
			AgentId:    m.AgentID,
			Type:       pb.MemoryType(pb.MemoryType_value[m.Type]),
			Content:    m.Content,
			Importance: m.Importance,
			CreatedAt:  m.CreatedAt.Unix(),
		})
	}

	return &pb.RecallMemoryResponse{
		Memories: pbMemories,
	}, nil
}

// GetSessionMemory gets session memories
func (s *MemoryService) GetSessionMemory(ctx context.Context, req *pb.GetSessionMemoryRequest) (*pb.RecallMemoryResponse, error) {
	memories, err := s.repo.GetBySession(ctx, req.SessionId, req.TenantId)
	if err != nil {
		return nil, err
	}

	var pbMemories []*pb.MemoryEntry
	for _, m := range memories {
		pbMemories = append(pbMemories, &pb.MemoryEntry{
			Id:         m.ID,
			SessionId:  m.SessionID,
			AgentId:    m.AgentID,
			Type:       pb.MemoryType(pb.MemoryType_value[m.Type]),
			Content:    m.Content,
			Importance: m.Importance,
			CreatedAt:  m.CreatedAt.Unix(),
		})
	}

	return &pb.RecallMemoryResponse{
		Memories: pbMemories,
	}, nil
}

// DeleteSessionMemory deletes session memories
func (s *MemoryService) DeleteSessionMemory(ctx context.Context, req *pb.DeleteSessionMemoryRequest) (*commonpb.Empty, error) {
	if err := s.repo.DeleteBySession(ctx, req.SessionId, req.TenantId); err != nil {
		return nil, err
	}
	return &commonpb.Empty{}, nil
}