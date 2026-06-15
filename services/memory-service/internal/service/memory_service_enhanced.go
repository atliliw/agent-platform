// Package service provides business logic for memory service
package service

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"agent-platform/pkg/llm"
	pb "agent-platform/pkg/pb/memory"
	commonpb "agent-platform/pkg/pb/common"
	"agent-platform/services/memory-service/internal/episodic"
	"agent-platform/services/memory-service/internal/repository"
	"agent-platform/services/memory-service/internal/semantic"
	"agent-platform/services/memory-service/internal/working"
)

// MemoryService provides memory management functionality
type MemoryService struct {
	pb.UnimplementedMemoryServiceServer
	llmClient llm.Client
	repo      *repository.MemoryRepository

	// Enhanced memory systems
	workingMemory  *working.WorkingMemory
	episodicMemory *episodic.EpisodicMemory
	semanticMemory *semantic.SemanticMemory

	// Consolidation and forgetting
	consolidator *MemoryConsolidator
	forgetter    *ForgettingManager

	mu sync.RWMutex
}

// NewMemoryService creates a new memory service
func NewMemoryService(llmClient llm.Client, repo *repository.MemoryRepository) *MemoryService {
	service := &MemoryService{
		llmClient:      llmClient,
		repo:           repo,
		workingMemory:  working.NewWorkingMemory(8000, 100),
		episodicMemory: episodic.NewEpisodicMemory(10000),
		semanticMemory: semantic.NewSemanticMemory(5000),
		consolidator:   NewMemoryConsolidator(),
		forgetter:      NewForgettingManager(),
	}

	// Start background processes
	go service.runConsolidation()
	go service.runForgetting()

	return service
}

// Save saves a memory
func (s *MemoryService) Save(ctx context.Context, req *pb.SaveMemoryRequest) (*pb.SaveMemoryResponse, error) {
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
		Importance: req.Importance,
		Vector:     embedding,
		TenantID:   req.TenantId,
		CreatedAt:  time.Now(),
	}

	if err := s.repo.Save(ctx, memory); err != nil {
		return nil, err
	}

	// Also store in appropriate memory system
	switch req.Type {
	case pb.MemoryType_WORKING:
		s.addToWorkingMemory(ctx, req, memory.ID)
	case pb.MemoryType_EPISODIC:
		s.addToEpisodicMemory(ctx, req, memory.ID, embedding)
	case pb.MemoryType_SEMANTIC:
		s.addToSemanticMemory(ctx, req, memory.ID, embedding)
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

	// Search in main repository
	memories, err := s.repo.Search(ctx, queryEmbedding, req.SessionId, req.AgentId, req.TenantId, topK)
	if err != nil {
		return nil, err
	}

	// Also search in semantic memory for concepts
	if req.Type == pb.MemoryType_SEMANTIC || req.Type == pb.MemoryType_UNKNOWN {
		concepts, err := s.semanticMemory.RecallByEmbedding(ctx, queryEmbedding, topK/2)
		if err == nil {
			for _, concept := range concepts {
				memories = append(memories, &repository.Memory{
					ID:         concept.ID,
					Type:       "SEMANTIC",
					Content:    fmt.Sprintf("%s: %s", concept.Name, concept.Description),
					Importance: concept.Importance,
				})
			}
		}
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

// ============================================================
// Enhanced Memory Operations
// ============================================================

// addToWorkingMemory adds to working memory
func (s *MemoryService) addToWorkingMemory(ctx context.Context, req *pb.SaveMemoryRequest, id string) {
	msg := working.Message{
		ID:         id,
		Type:       working.MessageTypeUser,
		Content:    req.Content,
		Importance: req.Importance,
	}
	s.workingMemory.Add(ctx, req.SessionId, msg)
}

// addToEpisodicMemory adds to episodic memory
func (s *MemoryService) addToEpisodicMemory(ctx context.Context, req *pb.SaveMemoryRequest, id string, embedding []float64) {
	episode := &episodic.Episode{
		ID:          id,
		SessionID:   req.SessionId,
		AgentID:     req.AgentId,
		Type:        episodic.EpisodeTypeConversation,
		Description: req.Content,
		StartTime:   time.Now(),
		EndTime:     time.Now(),
		Importance:  req.Importance,
		Vector:      embedding,
	}
	s.episodicMemory.Store(ctx, episode)
}

// addToSemanticMemory adds to semantic memory
func (s *MemoryService) addToSemanticMemory(ctx context.Context, req *pb.SaveMemoryRequest, id string, embedding []float64) {
	concept := &semantic.Concept{
		ID:          id,
		Type:        semantic.ConceptTypeFact,
		Description: req.Content,
		Importance:  req.Importance,
		Vector:      embedding,
	}
	s.semanticMemory.Store(ctx, concept)
}

// GetTimeline gets the timeline for a session
func (s *MemoryService) GetTimeline(ctx context.Context, sessionID string) (*episodic.Timeline, error) {
	return s.episodicMemory.GetTimeline(ctx, sessionID)
}

// GetKnowledgeGraph gets the knowledge graph
func (s *MemoryService) GetKnowledgeGraph() *semantic.KnowledgeGraph {
	return s.semanticMemory.GetGraph()
}

// Consolidate triggers memory consolidation
func (s *MemoryService) Consolidate(ctx context.Context, sessionID string) error {
	return s.consolidator.Consolidate(ctx, s, sessionID)
}

// ReinforceMemory reinforces a memory (increases its strength)
func (s *MemoryService) ReinforceMemory(ctx context.Context, memoryID string) error {
	// Get memories by session and find the one
	// Simplified implementation
	return nil
}

// ============================================================
// Memory Consolidation
// ============================================================

// MemoryConsolidator handles memory consolidation
type MemoryConsolidator struct {
	consolidationInterval time.Duration
	lastConsolidation     map[string]time.Time
	mu                    sync.RWMutex
}

// NewMemoryConsolidator creates a new consolidator
func NewMemoryConsolidator() *MemoryConsolidator {
	return &MemoryConsolidator{
		consolidationInterval: 5 * time.Minute,
		lastConsolidation:     make(map[string]time.Time),
	}
}

// Consolidate performs memory consolidation for a session
func (c *MemoryConsolidator) Consolidate(ctx context.Context, svc *MemoryService, sessionID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if consolidation needed
	last, ok := c.lastConsolidation[sessionID]
	if ok && time.Since(last) < c.consolidationInterval {
		return nil
	}

	// Get working memory
	workingCtx, err := svc.workingMemory.Get(ctx, sessionID)
	if err != nil {
		return nil // No working memory to consolidate
	}

	// Extract key information from working memory
	for _, msg := range workingCtx.Messages {
		if msg.Importance >= 0.7 {
			// Promote to long-term memory
			embedding, err := svc.llmClient.Embed(ctx, msg.Content)
			if err != nil {
				continue
			}

			// Store as semantic memory if it contains knowledge
			concept := &semantic.Concept{
				Type:        semantic.ConceptTypeFact,
				Description: msg.Content,
				Importance:  msg.Importance,
				Vector:      embedding,
				Source:      sessionID,
			}
			svc.semanticMemory.Store(ctx, concept)
		}
	}

	c.lastConsolidation[sessionID] = time.Now()
	return nil
}

// runConsolidation runs periodic consolidation
func (s *MemoryService) runConsolidation() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		ctx := context.Background()
		// Consolidate all active sessions
		stats := s.workingMemory.GetStats()
		if contexts, ok := stats["contexts"].(map[string]interface{}); ok {
			for sessionID := range contexts {
				s.consolidator.Consolidate(ctx, s, sessionID)
			}
		}
	}
}

// ============================================================
// Forgetting Curve
// ============================================================

// ForgettingManager manages memory forgetting based on Ebbinghaus forgetting curve
type ForgettingManager struct {
	// Ebbinghaus parameters
	retentionRate float64 // Base retention rate
	timeConstant  float64 // Time constant for decay

	forgetThreshold float64 // Below this importance, memory is forgotten
	cleanupInterval time.Duration
}

// NewForgettingManager creates a new forgetting manager
func NewForgettingManager() *ForgettingManager {
	return &ForgettingManager{
		retentionRate:  0.9,
		timeConstant:   24 * time.Hour.Seconds(), // 24 hours
		forgetThreshold: 0.2,
		cleanupInterval: 1 * time.Hour,
	}
}

// CalculateRetention calculates retention using Ebbinghaus curve
// R = e^(-t/S) where t is time since learning, S is memory strength
func (f *ForgettingManager) CalculateRetention(createdAt time.Time, importance float64) float64 {
	timeSinceLearning := time.Since(createdAt).Seconds()
	strength := f.timeConstant * importance // Higher importance = stronger memory

	retention := math.Exp(-timeSinceLearning / strength)
	return retention
}

// ApplyDecay applies forgetting curve to update memory importance
func (f *ForgettingManager) ApplyDecay(memory *repository.Memory) float64 {
	retention := f.CalculateRetention(memory.CreatedAt, memory.Importance)
	newImportance := memory.Importance * retention

	// Clamp to [0, 1]
	if newImportance < 0 {
		newImportance = 0
	}
	if newImportance > 1 {
		newImportance = 1
	}

	return newImportance
}

// ShouldForget determines if a memory should be forgotten
func (f *ForgettingManager) ShouldForget(memory *repository.Memory) bool {
	retention := f.CalculateRetention(memory.CreatedAt, memory.Importance)
	return retention < f.forgetThreshold
}

// runForgetting runs periodic forgetting
func (s *MemoryService) runForgetting() {
	ticker := time.NewTicker(s.forgetter.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		ctx := context.Background()
		s.cleanupForgottenMemories(ctx)
	}
}

// cleanupForgottenMemories removes forgotten memories
func (s *MemoryService) cleanupForgottenMemories(ctx context.Context) {
	// Get all memories
	memories, err := s.repo.GetAll(ctx, "")
	if err != nil {
		return
	}

	for _, memory := range memories {
		if s.forgetter.ShouldForget(memory) {
			s.repo.Delete(ctx, memory.ID, memory.TenantID)
		}
		// Note: Importance update would require additional repository method
	}
}

// GetMemoryStats returns memory system statistics
func (s *MemoryService) GetMemoryStats() map[string]interface{} {
	stats := map[string]interface{}{
		"working":  s.workingMemory.GetStats(),
		"episodic": s.episodicMemory.GetStats(),
		"semantic": s.semanticMemory.GetGraph().GetStats(),
		"forgetting": map[string]interface{}{
			"retention_rate":   s.forgetter.retentionRate,
			"forget_threshold": s.forgetter.forgetThreshold,
		},
	}
	return stats
}
