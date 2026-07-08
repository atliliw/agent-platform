// Package service provides memory management functionality with forgetting
package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"agent-platform/pkg/llm"
	"agent-platform/pkg/qdrant"
	pb "agent-platform/pkg/pb/memory"
	commonpb "agent-platform/pkg/pb/common"
	"agent-platform/services/memory-service/internal/repository"
)

// ForgettingConfig holds configuration for memory forgetting
type ForgettingConfig struct {
	// TimeDecayRate controls how quickly memories fade (0-1)
	TimeDecayRate float64
	// ImportanceThreshold is the minimum score to keep a memory
	ImportanceThreshold float64
	// MaxAge is the maximum age of a memory in hours (0 = no limit)
	MaxAgeHours int
	// CleanupInterval is how often to run cleanup (in hours)
	CleanupIntervalHours int
}

// DefaultForgettingConfig returns default configuration
func DefaultForgettingConfig() ForgettingConfig {
	return ForgettingConfig{
		TimeDecayRate:        0.1,
		ImportanceThreshold:  0.3,
		MaxAgeHours:          720, // 30 days
		CleanupIntervalHours: 24,
	}
}

// MemoryServiceWithForgetting provides memory management with forgetting
type MemoryServiceWithForgetting struct {
	pb.UnimplementedMemoryServiceServer
	llmClient llm.Client
	qdrant   *qdrant.Client
	repo     *repository.MemoryRepository
	config   ForgettingConfig
}

// NewMemoryServiceWithForgetting creates a new memory service
func NewMemoryServiceWithForgetting(llmClient llm.Client, qdrant *qdrant.Client, repo *repository.MemoryRepository, config ForgettingConfig) *MemoryServiceWithForgetting {
	return &MemoryServiceWithForgetting{
		llmClient: llmClient,
		qdrant:    qdrant,
		repo:      repo,
		config:    config,
	}
}

// Save saves a memory with automatic importance scoring
func (s *MemoryServiceWithForgetting) Save(ctx context.Context, req *pb.SaveMemoryRequest) (*pb.SaveMemoryResponse, error) {
	// Calculate importance score based on content
	importance := s.calculateImportance(req.Content, req.Importance)

	// Generate embedding
	embedding, err := s.llmClient.Embed(ctx, req.Content)
	if err != nil {
		return nil, fmt.Errorf("generate embedding: %w", err)
	}

	// Save to repository
	memory := &repository.Memory{
		SessionID:  req.SessionId,
		AgentID:    req.AgentId,
		Type:       req.Type.String(),
		Content:    req.Content,
		Importance: importance,
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

// SaveBatch saves multiple memories with importance scoring
func (s *MemoryServiceWithForgetting) SaveBatch(ctx context.Context, req *pb.SaveMemoryBatchRequest) (*pb.SaveMemoryBatchResponse, error) {
	var ids []string
	now := time.Now()

	for _, m := range req.Memories {
		importance := s.calculateImportance(m.Content, m.Importance)

		embedding, err := s.llmClient.Embed(ctx, m.Content)
		if err != nil {
			return nil, fmt.Errorf("generate embedding: %w", err)
		}

		memory := &repository.Memory{
			SessionID:  m.SessionId,
			AgentID:    m.AgentId,
			Type:       m.Type.String(),
			Content:    m.Content,
			Importance: importance,
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

// Recall recalls memories with time decay scoring
func (s *MemoryServiceWithForgetting) Recall(ctx context.Context, req *pb.RecallMemoryRequest) (*pb.RecallMemoryResponse, error) {
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
	memories, err := s.repo.Search(ctx, queryEmbedding, req.SessionId, req.AgentId, req.TenantId, topK*2)
	if err != nil {
		return nil, err
	}

	// Apply time decay scoring
	now := time.Now()
	for i := range memories {
		// Calculate decayed importance
		ageHours := now.Sub(memories[i].CreatedAt).Hours()
		decayFactor := math.Exp(-s.config.TimeDecayRate * ageHours / 24) // Daily decay
		memories[i].Importance = memories[i].Importance * decayFactor
	}

	// Sort by decayed importance
	s.sortMemoriesByImportance(memories)

	// Return top K
	if len(memories) > topK {
		memories = memories[:topK]
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

// Cleanup removes low-importance and old memories
func (s *MemoryServiceWithForgetting) Cleanup(ctx context.Context, tenantID string) (int, error) {
	// Get all memories
	memories, err := s.repo.GetAll(ctx, tenantID)
	if err != nil {
		return 0, fmt.Errorf("get memories: %w", err)
	}

	now := time.Now()
	deleted := 0

	for _, m := range memories {
		// Calculate current importance with decay
		ageHours := now.Sub(m.CreatedAt).Hours()
		decayFactor := math.Exp(-s.config.TimeDecayRate * ageHours / 24)
		currentImportance := m.Importance * decayFactor

		shouldDelete := false

		// Delete if below threshold
		if currentImportance < s.config.ImportanceThreshold {
			shouldDelete = true
		}

		// Delete if too old
		if s.config.MaxAgeHours > 0 && ageHours > float64(s.config.MaxAgeHours) {
			shouldDelete = true
		}

		if shouldDelete {
			if err := s.repo.Delete(ctx, m.ID, tenantID); err != nil {
				// Log error but continue
				continue
			}
			deleted++
		}
	}

	return deleted, nil
}

// GetSessionMemory gets session memories
func (s *MemoryServiceWithForgetting) GetSessionMemory(ctx context.Context, req *pb.GetSessionMemoryRequest) (*pb.RecallMemoryResponse, error) {
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
func (s *MemoryServiceWithForgetting) DeleteSessionMemory(ctx context.Context, req *pb.DeleteSessionMemoryRequest) (*commonpb.Empty, error) {
	if err := s.repo.DeleteBySession(ctx, req.SessionId, req.TenantId); err != nil {
		return nil, err
	}
	return &commonpb.Empty{}, nil
}

// GetAllMemories gets all memories for a tenant (user-level)
func (s *MemoryServiceWithForgetting) GetAllMemories(ctx context.Context, req *pb.GetAllMemoriesRequest) (*pb.RecallMemoryResponse, error) {
	memories, err := s.repo.GetAll(ctx, req.TenantId)
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

// DeleteMemory deletes a single memory by ID
func (s *MemoryServiceWithForgetting) DeleteMemory(ctx context.Context, req *pb.DeleteMemoryRequest) (*commonpb.Empty, error) {
	if err := s.repo.Delete(ctx, req.Id, req.TenantId); err != nil {
		return nil, err
	}
	return &commonpb.Empty{}, nil
}

// GetForgettingConfig returns the current forgetting configuration
func (s *MemoryServiceWithForgetting) GetForgettingConfig() ForgettingConfig {
	return s.config
}

// UpdateForgettingConfig updates the forgetting configuration
func (s *MemoryServiceWithForgetting) UpdateForgettingConfig(config ForgettingConfig) {
	s.config = config
}

// calculateImportance calculates importance score for a memory
func (s *MemoryServiceWithForgetting) calculateImportance(content string, baseImportance float64) float64 {
	// Start with base importance
	score := baseImportance

	// Boost importance based on content characteristics
	contentLower := content

	// Boost for questions (likely important)
	questionCount := countOccurrences(contentLower, "?") + countOccurrences(contentLower, "？")
	score += float64(questionCount) * 0.1

	// Boost for decisions or agreements
	decisionKeywords := []string{"决定", "同意", "confirm", "agreed", "decision", "important", "重要", "关键"}
	for _, kw := range decisionKeywords {
		if contains(contentLower, kw) {
			score += 0.15
		}
	}

	// Boost for action items
	actionKeywords := []string{"todo", "任务", "需要", "should", "must", "will", "将要", "需要做"}
	for _, kw := range actionKeywords {
		if contains(contentLower, kw) {
			score += 0.1
		}
	}

	// Cap at 1.0
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// sortMemoriesByImportance sorts memories by importance descending
func (s *MemoryServiceWithForgetting) sortMemoriesByImportance(memories []*repository.Memory) {
	// Simple bubble sort
	n := len(memories)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if memories[j].Importance < memories[j+1].Importance {
				memories[j], memories[j+1] = memories[j+1], memories[j]
			}
		}
	}
}

// Helper functions
func countOccurrences(s, substr string) int {
	count := 0
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			count++
		}
	}
	return count
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
