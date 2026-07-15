// Package handler provides gRPC handlers for memory service
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pb "agent-platform/pkg/pb/memory"
	commonpb "agent-platform/pkg/pb/common"
	"agent-platform/services/memory-service/internal/cases"
	"agent-platform/services/memory-service/internal/episodic"
	"agent-platform/services/memory-service/internal/semantic"
	"agent-platform/services/memory-service/internal/service"
	"agent-platform/services/memory-service/internal/working"
)

// MemoryHandler implements MemoryServiceServer with layered memory subsystems
type MemoryHandler struct {
	pb.UnimplementedMemoryServiceServer
	service      *service.MemoryServiceWithForgetting
	episodic     *episodic.EpisodicMemory
	semantic     *semantic.SemanticMemory
	working      *working.WorkingMemory
	caseLibrary  *cases.CaseLibrary
	caseRetriever *cases.CaseRetriever
}

// NewMemoryHandler creates a new memory handler with all subsystems
func NewMemoryHandler(
	svc *service.MemoryServiceWithForgetting,
	ep *episodic.EpisodicMemory,
	sm *semantic.SemanticMemory,
	wm *working.WorkingMemory,
	cl *cases.CaseLibrary,
	cr *cases.CaseRetriever,
) *MemoryHandler {
	return &MemoryHandler{
		service:       svc,
		episodic:      ep,
		semantic:      sm,
		working:       wm,
		caseLibrary:   cl,
		caseRetriever: cr,
	}
}

// ============================================================
// Basic Memory RPCs (delegated to MemoryServiceWithForgetting)
// ============================================================

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

// ============================================================
// Episodic Memory Handlers
// ============================================================

// StoreEpisodeRequest is the internal request for storing an episode
type StoreEpisodeRequest struct {
	SessionID    string                 `json:"session_id"`
	AgentID      string                 `json:"agent_id"`
	Type         string                 `json:"type"`
	Title        string                 `json:"title"`
	Description  string                 `json:"description"`
	Outcome      string                 `json:"outcome"`
	Importance   float64                `json:"importance"`
	Participants []string               `json:"participants"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// StoreEpisode stores a new episode in episodic memory
func (h *MemoryHandler) StoreEpisode(ctx context.Context, req *StoreEpisodeRequest) (*episodic.Episode, error) {
	episode := &episodic.Episode{
		SessionID:    req.SessionID,
		AgentID:      req.AgentID,
		Type:         episodic.EpisodeType(req.Type),
		Title:        req.Title,
		Description:  req.Description,
		Outcome:      req.Outcome,
		Importance:   req.Importance,
		Participants: req.Participants,
		Metadata:     req.Metadata,
		StartTime:    time.Now(),
		EndTime:      time.Now(),
	}

	if err := h.episodic.Store(ctx, episode); err != nil {
		return nil, err
	}

	return episode, nil
}

// GetTimeline returns the timeline for a session
func (h *MemoryHandler) GetTimeline(ctx context.Context, sessionID string) (*episodic.Timeline, error) {
	return h.episodic.GetTimeline(ctx, sessionID)
}

// GetSimilarEpisodes returns episodes similar to the given embedding
func (h *MemoryHandler) GetSimilarEpisodes(ctx context.Context, embedding []float64, topK int) ([]*episodic.Episode, error) {
	return h.episodic.GetSimilarEpisodes(ctx, embedding, topK)
}

// SearchEpisodes searches episodes by content query
func (h *MemoryHandler) SearchEpisodes(ctx context.Context, query string) ([]*episodic.Episode, error) {
	return h.episodic.Search(ctx, query)
}

// GetEpisodesBySession retrieves episodes for a session
func (h *MemoryHandler) GetEpisodesBySession(ctx context.Context, sessionID string) ([]*episodic.Episode, error) {
	return h.episodic.GetBySession(ctx, sessionID)
}

// ============================================================
// Semantic Memory Handlers
// ============================================================

// StoreConcept stores a concept in semantic memory
func (h *MemoryHandler) StoreConcept(ctx context.Context, concept *semantic.Concept) error {
	return h.semantic.Store(ctx, concept)
}

// StoreRelation stores a relation between concepts
func (h *MemoryHandler) StoreRelation(ctx context.Context, relation *semantic.Relation) error {
	return h.semantic.StoreRelation(ctx, relation)
}

// RecallConcepts recalls concepts matching a query
func (h *MemoryHandler) RecallConcepts(ctx context.Context, query string, topK int) ([]*semantic.Concept, error) {
	return h.semantic.Recall(ctx, query, topK)
}

// GetRelatedConcepts returns concepts related to a given concept
func (h *MemoryHandler) GetRelatedConcepts(ctx context.Context, conceptID string, relationType string) ([]*semantic.Concept, error) {
	return h.semantic.GetRelated(ctx, conceptID, semantic.RelationType(relationType))
}

// GetSemanticGraph returns the full knowledge graph data
func (h *MemoryHandler) GetSemanticGraph() *semantic.KnowledgeGraph {
	return h.semantic.GetGraph()
}

// ============================================================
// Working Memory Handlers
// ============================================================

// AddWorkingMessage adds a message to working memory
func (h *MemoryHandler) AddWorkingMessage(ctx context.Context, sessionID string, msg working.Message) error {
	return h.working.Add(ctx, sessionID, msg)
}

// GetWorkingContext retrieves the working memory context for a session
func (h *MemoryHandler) GetWorkingContext(ctx context.Context, sessionID string) (*working.WorkingMemoryContext, error) {
	return h.working.Get(ctx, sessionID)
}

// GetWorkingMessagesForLLM retrieves messages formatted for LLM consumption
func (h *MemoryHandler) GetWorkingMessagesForLLM(ctx context.Context, sessionID string) []working.LLMMessage {
	return h.working.GetMessagesForLLM(ctx, sessionID)
}

// ClearWorkingContext clears the working memory for a session
func (h *MemoryHandler) ClearWorkingContext(ctx context.Context, sessionID string) error {
	return h.working.Clear(ctx, sessionID)
}

// GetWorkingTokenUsage returns token usage for a session
func (h *MemoryHandler) GetWorkingTokenUsage(ctx context.Context, sessionID string) (int, int, float64, error) {
	return h.working.GetTokenUsage(ctx, sessionID)
}

// ============================================================
// Forgetting Handlers
// ============================================================

// GetForgettingConfig returns the current forgetting configuration
func (h *MemoryHandler) GetForgettingConfig() service.ForgettingConfig {
	return h.service.GetForgettingConfig()
}

// UpdateForgettingConfig updates the forgetting configuration
func (h *MemoryHandler) UpdateForgettingConfig(config service.ForgettingConfig) {
	h.service.UpdateForgettingConfig(config)
}

// TriggerCleanup manually triggers memory cleanup for a tenant
func (h *MemoryHandler) TriggerCleanup(ctx context.Context, tenantID string) (int, error) {
	return h.service.Cleanup(ctx, tenantID)
}

// ============================================================
// Stats Handlers
// ============================================================

// GetEpisodicStats returns episodic memory statistics
func (h *MemoryHandler) GetEpisodicStats() map[string]interface{} {
	return h.episodic.GetStats()
}

// GetSemanticStats returns semantic memory statistics
func (h *MemoryHandler) GetSemanticStats() map[string]interface{} {
	return h.semantic.GetGraph().GetStats()
}

// GetWorkingStats returns working memory statistics
func (h *MemoryHandler) GetWorkingStats() map[string]interface{} {
	return h.working.GetStats()
}

// jsonBytes is a helper to marshal data for HTTP responses
func jsonBytes(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

// ============================================================
// Case-Based Reasoning Handlers
// ============================================================

// StoreCase stores a new case in the case library
func (h *MemoryHandler) StoreCase(ctx context.Context, case_ *cases.Case) error {
	if h.caseLibrary == nil {
		return fmt.Errorf("case library not initialized")
	}
	return h.caseLibrary.Store(ctx, case_)
}

// GetCase retrieves a case by ID
func (h *MemoryHandler) GetCase(ctx context.Context, id string) (*cases.Case, error) {
	if h.caseLibrary == nil {
		return nil, fmt.Errorf("case library not initialized")
	}
	return h.caseLibrary.Get(ctx, id)
}

// GetCasesByAgent retrieves cases for a specific agent
func (h *MemoryHandler) GetCasesByAgent(ctx context.Context, agentID string) []*cases.Case {
	if h.caseLibrary == nil {
		return nil
	}
	return h.caseLibrary.GetByAgent(ctx, agentID)
}

// GetSuccessCases retrieves all successful cases
func (h *MemoryHandler) GetSuccessCases(ctx context.Context) []*cases.Case {
	if h.caseLibrary == nil {
		return nil
	}
	return h.caseLibrary.GetSuccessCases(ctx)
}

// GetFailureCases retrieves all failed cases
func (h *MemoryHandler) GetFailureCases(ctx context.Context) []*cases.Case {
	if h.caseLibrary == nil {
		return nil
	}
	return h.caseLibrary.GetFailureCases(ctx)
}

// GetCasesByCategory retrieves cases by category
func (h *MemoryHandler) GetCasesByCategory(ctx context.Context, category string) []*cases.Case {
	if h.caseLibrary == nil {
		return nil
	}
	return h.caseLibrary.GetByCategory(ctx, category)
}

// DeleteCase deletes a case by ID
func (h *MemoryHandler) DeleteCase(ctx context.Context, id string) error {
	if h.caseLibrary == nil {
		return fmt.Errorf("case library not initialized")
	}
	return h.caseLibrary.Delete(ctx, id)
}

// ArchiveCase archives a case by ID
func (h *MemoryHandler) ArchiveCase(ctx context.Context, id string) error {
	if h.caseLibrary == nil {
		return fmt.Errorf("case library not initialized")
	}
	return h.caseLibrary.Archive(ctx, id)
}

// GetCaseStatistics returns case library statistics
func (h *MemoryHandler) GetCaseStatistics() map[string]interface{} {
	if h.caseLibrary == nil {
		return map[string]interface{}{"total_cases": 0, "enabled": false}
	}
	stats := h.caseLibrary.GetStatistics()
	stats["enabled"] = true
	return stats
}

// RetrieveSimilarCases retrieves similar cases based on a query
func (h *MemoryHandler) RetrieveSimilarCases(ctx context.Context, query string, topK int) ([]*cases.SimilarCaseResult, error) {
	if h.caseRetriever == nil {
		return nil, fmt.Errorf("case retriever not initialized")
	}
	return h.caseRetriever.Retrieve(ctx, query, topK)
}

// RetrieveCasesByTask retrieves cases similar to a task
func (h *MemoryHandler) RetrieveCasesByTask(ctx context.Context, task string, topK int, preferSuccess bool) ([]*cases.SimilarCaseResult, error) {
	if h.caseRetriever == nil {
		return nil, fmt.Errorf("case retriever not initialized")
	}
	return h.caseRetriever.RetrieveByTask(ctx, task, topK, preferSuccess)
}

// RetrieveCasesByPattern retrieves cases matching a pattern
func (h *MemoryHandler) RetrieveCasesByPattern(ctx context.Context, pattern string, topK int) ([]*cases.SimilarCaseResult, error) {
	if h.caseRetriever == nil {
		return nil, fmt.Errorf("case retriever not initialized")
	}
	return h.caseRetriever.RetrieveByPattern(ctx, pattern, topK)
}
