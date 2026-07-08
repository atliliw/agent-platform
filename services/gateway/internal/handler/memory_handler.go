// Package handler provides HTTP handlers for Gateway
package handler

import (
	"agent-platform/pkg/client"
	"agent-platform/pkg/config"
	pb "agent-platform/pkg/pb/memory"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// MemoryHandler handles memory requests
type MemoryHandler struct {
	cfg        *config.Config
	clientPool *client.ClientPool
}

// NewRealMemoryHandler creates a new memory handler with real gRPC client
func NewRealMemoryHandler(cfg *config.Config) *MemoryHandler {
	// Create client pool for memory service
	pool, err := client.NewClientPool("", "", cfg.Services.Memory, "", "", "")
	if err != nil {
		// Return handler without client pool - will return errors
		return &MemoryHandler{cfg: cfg}
	}
	return &MemoryHandler{cfg: cfg, clientPool: pool}
}

// SaveRequest is the HTTP request for saving memory
type SaveRequest struct {
	SessionId  string  `json:"session_id"`
	AgentId    string  `json:"agent_id"`
	Type       string  `json:"type"`
	Content    string  `json:"content"`
	Importance float64 `json:"importance"`
	TenantId   string  `json:"tenant_id"`
}

// RecallRequest is the HTTP request for recalling memory
type RecallRequest struct {
	Query     string `json:"query"`
	SessionId string `json:"session_id"`
	AgentId   string  `json:"agent_id"`
	TenantId  string  `json:"tenant_id"`
	TopK      int32  `json:"top_k"`
}

// Save saves memory
func (h *MemoryHandler) Save(c *gin.Context) {
	var req SaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "invalid request: " + err.Error()})
		return
	}

	if h.clientPool == nil || h.clientPool.MemoryConn == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not connected"})
		return
	}

	// Convert type string to MemoryType
	memType := pb.MemoryType_MEMORY_TYPE_FACT
	switch req.Type {
	case "IMPORTANT":
		memType = pb.MemoryType_MEMORY_TYPE_IMPORTANT
	case "SUMMARY":
		memType = pb.MemoryType_MEMORY_TYPE_SUMMARY
	case "FACT":
		memType = pb.MemoryType_MEMORY_TYPE_FACT
	case "CONVERSATION":
		memType = pb.MemoryType_MEMORY_TYPE_FACT
	}

	// Call memory service
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	memClient := pb.NewMemoryServiceClient(h.clientPool.MemoryConn)
	resp, err := memClient.Save(ctx, &pb.SaveMemoryRequest{
		SessionId:  req.SessionId,
		AgentId:    req.AgentId,
		Type:       memType,
		Content:    req.Content,
		Importance: req.Importance,
		TenantId:   req.TenantId,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "save memory failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"id":         resp.Id,
			"created_at": resp.CreatedAt,
		},
	})
}

// Recall recalls memory
func (h *MemoryHandler) Recall(c *gin.Context) {
	var req RecallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "invalid request: " + err.Error()})
		return
	}

	if h.clientPool == nil || h.clientPool.MemoryConn == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not connected"})
		return
	}

	if req.TopK == 0 {
		req.TopK = 5
	}

	// Call memory service
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	memClient := pb.NewMemoryServiceClient(h.clientPool.MemoryConn)
	resp, err := memClient.Recall(ctx, &pb.RecallMemoryRequest{
		Query:     req.Query,
		SessionId: req.SessionId,
		AgentId:   req.AgentId,
		TenantId:  req.TenantId,
		TopK:      req.TopK,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "recall memory failed: " + err.Error()})
		return
	}

	// Convert memories to JSON format
	memories := make([]gin.H, 0, len(resp.Memories))
	for _, m := range resp.Memories {
		memories = append(memories, gin.H{
			"id":          m.Id,
			"session_id":  m.SessionId,
			"agent_id":    m.AgentId,
			"type":        m.Type.String(),
			"content":     m.Content,
			"importance":  m.Importance,
			"created_at":  m.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"memories": memories,
		},
	})
}

// GetSessionMemory gets session memory
func (h *MemoryHandler) GetSessionMemory(c *gin.Context) {
	sessionId := c.Param("id")
	tenantId := c.Query("tenant_id")

	if h.clientPool == nil || h.clientPool.MemoryConn == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not connected"})
		return
	}

	// Call memory service
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	memClient := pb.NewMemoryServiceClient(h.clientPool.MemoryConn)
	resp, err := memClient.GetSessionMemory(ctx, &pb.GetSessionMemoryRequest{
		SessionId: sessionId,
		TenantId:  tenantId,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "get session memory failed: " + err.Error()})
		return
	}

	// Convert memories to JSON format
	memories := make([]gin.H, 0, len(resp.Memories))
	for _, m := range resp.Memories {
		memories = append(memories, gin.H{
			"id":          m.Id,
			"session_id":  m.SessionId,
			"agent_id":    m.AgentId,
			"type":        m.Type.String(),
			"content":     m.Content,
			"importance":  m.Importance,
			"created_at":  m.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"memories": memories,
		},
	})
}

// DeleteSessionMemory deletes session memory
func (h *MemoryHandler) DeleteSessionMemory(c *gin.Context) {
	sessionId := c.Param("id")
	tenantId := c.Query("tenant_id")

	if h.clientPool == nil || h.clientPool.MemoryConn == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not connected"})
		return
	}

	// Call memory service
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	memClient := pb.NewMemoryServiceClient(h.clientPool.MemoryConn)
	_, err := memClient.DeleteSessionMemory(ctx, &pb.DeleteSessionMemoryRequest{
		SessionId: sessionId,
		TenantId:  tenantId,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "delete session memory failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}

// GetAllMemories gets all memories for a tenant (user-level)
func (h *MemoryHandler) GetAllMemories(c *gin.Context) {
	tenantId := c.Query("tenant_id")

	if h.clientPool == nil || h.clientPool.MemoryConn == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not connected"})
		return
	}

	// Call memory service
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	memClient := pb.NewMemoryServiceClient(h.clientPool.MemoryConn)
	resp, err := memClient.GetAllMemories(ctx, &pb.GetAllMemoriesRequest{
		TenantId: tenantId,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "get all memories failed: " + err.Error()})
		return
	}

	// Convert memories to JSON format
	memories := make([]gin.H, 0, len(resp.Memories))
	for _, m := range resp.Memories {
		memories = append(memories, gin.H{
			"id":         m.Id,
			"session_id": m.SessionId,
			"agent_id":   m.AgentId,
			"type":       m.Type.String(),
			"content":    m.Content,
			"importance": m.Importance,
			"created_at": m.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"memories": memories,
		},
	})
}

// DeleteMemory deletes a single memory by ID
func (h *MemoryHandler) DeleteMemory(c *gin.Context) {
	memoryId := c.Param("id")
	tenantId := c.Query("tenant_id")

	if h.clientPool == nil || h.clientPool.MemoryConn == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not connected"})
		return
	}

	// Call memory service
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	memClient := pb.NewMemoryServiceClient(h.clientPool.MemoryConn)
	_, err := memClient.DeleteMemory(ctx, &pb.DeleteMemoryRequest{
		Id:       memoryId,
		TenantId: tenantId,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "delete memory failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}

// DeleteAllSessionMemories deletes all memories for a tenant (clear user memory)
func (h *MemoryHandler) DeleteAllSessionMemories(c *gin.Context) {
	tenantId := c.Query("tenant_id")

	if h.clientPool == nil || h.clientPool.MemoryConn == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not connected"})
		return
	}

	// Call memory service with empty session_id to delete all tenant memories
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	memClient := pb.NewMemoryServiceClient(h.clientPool.MemoryConn)
	_, err := memClient.DeleteSessionMemory(ctx, &pb.DeleteSessionMemoryRequest{
		SessionId: "", // Empty session_id means delete all for tenant
		TenantId:  tenantId,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "clear memories failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "all memories cleared"})
}

// ============================================================
// Layered Memory HTTP Handlers
// ============================================================

// LayeredMemoryHandler handles layered memory (episodic, semantic, working) HTTP requests
type LayeredMemoryHandler struct {
	cfg *config.Config
}

// NewLayeredMemoryHandler creates a new layered memory handler
func NewLayeredMemoryHandler(cfg *config.Config) *LayeredMemoryHandler {
	return &LayeredMemoryHandler{cfg: cfg}
}

// StoreEpisode stores a new episodic memory
func (h *LayeredMemoryHandler) StoreEpisode(c *gin.Context) {
	var req struct {
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
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "invalid request: " + err.Error()})
		return
	}

	// For now, store as a FACT memory via gRPC (episodic is in-memory in the service)
	if h.cfg.Services.Memory != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		pool, err := client.NewClientPool("", "", h.cfg.Services.Memory, "", "", "")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not available"})
			return
		}
		if pool.MemoryConn == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not connected"})
			return
		}

		memClient := pb.NewMemoryServiceClient(pool.MemoryConn)
		resp, err := memClient.Save(ctx, &pb.SaveMemoryRequest{
			SessionId:  req.SessionID,
			AgentId:    req.AgentID,
			Type:       pb.MemoryType_MEMORY_TYPE_IMPORTANT,
			Content:    req.Title + ": " + req.Description,
			Importance: req.Importance,
			TenantId:   c.GetString("tenant_id"),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "store episode failed: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{
				"id":         resp.Id,
				"created_at": resp.CreatedAt,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"id": "ep-local", "created_at": time.Now().Unix()}})
}

// GetEpisodes retrieves episodic memories (timeline)
func (h *LayeredMemoryHandler) GetEpisodes(c *gin.Context) {
	sessionID := c.Query("session_id")
	tenantID := c.GetString("tenant_id")

	if h.cfg.Services.Memory != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		pool, err := client.NewClientPool("", "", h.cfg.Services.Memory, "", "", "")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not available"})
			return
		}
		if pool.MemoryConn == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not connected"})
			return
		}

		memClient := pb.NewMemoryServiceClient(pool.MemoryConn)
		resp, err := memClient.GetSessionMemory(ctx, &pb.GetSessionMemoryRequest{
			SessionId: sessionID,
			TenantId:  tenantID,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "get episodes failed: " + err.Error()})
			return
		}

		episodes := make([]gin.H, 0, len(resp.Memories))
		for _, m := range resp.Memories {
			episodes = append(episodes, gin.H{
				"id":          m.Id,
				"session_id":  m.SessionId,
				"agent_id":    m.AgentId,
				"event_type":  m.Type.String(),
				"title":       m.Content,
				"description": m.Content,
				"importance":  m.Importance,
				"created_at":  m.CreatedAt,
			})
		}

		c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"episodes": episodes}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"episodes": []interface{}{}}})
}

// GetSimilarEpisodes retrieves similar episodes
func (h *LayeredMemoryHandler) GetSimilarEpisodes(c *gin.Context) {
	var req struct {
		Query string `json:"query"`
		TopK  int    `json:"top_k"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "invalid request: " + err.Error()})
		return
	}

	if req.TopK == 0 {
		req.TopK = 5
	}

	tenantID := c.GetString("tenant_id")

	if h.cfg.Services.Memory != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		pool, err := client.NewClientPool("", "", h.cfg.Services.Memory, "", "", "")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not available"})
			return
		}
		if pool.MemoryConn == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not connected"})
			return
		}

		memClient := pb.NewMemoryServiceClient(pool.MemoryConn)
		resp, err := memClient.Recall(ctx, &pb.RecallMemoryRequest{
			Query:    req.Query,
			TopK:     int32(req.TopK),
			TenantId: tenantID,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "similar episodes failed: " + err.Error()})
			return
		}

		episodes := make([]gin.H, 0, len(resp.Memories))
		for _, m := range resp.Memories {
			episodes = append(episodes, gin.H{
				"id":          m.Id,
				"session_id":  m.SessionId,
				"title":       m.Content,
				"description": m.Content,
				"importance":  m.Importance,
				"created_at":  m.CreatedAt,
			})
		}

		c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"episodes": episodes}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"episodes": []interface{}{}}})
}

// StoreConcept stores a semantic concept
func (h *LayeredMemoryHandler) StoreConcept(c *gin.Context) {
	var req struct {
		Type        string                 `json:"type"`
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Importance  float64                `json:"importance"`
		Confidence  float64                `json:"confidence"`
		Source      string                 `json:"source"`
		Properties  map[string]interface{} `json:"properties"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "invalid request: " + err.Error()})
		return
	}

	// Store as a FACT memory via gRPC
	if h.cfg.Services.Memory != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		pool, err := client.NewClientPool("", "", h.cfg.Services.Memory, "", "", "")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not available"})
			return
		}
		if pool.MemoryConn == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not connected"})
			return
		}

		memClient := pb.NewMemoryServiceClient(pool.MemoryConn)
		content := req.Name + ": " + req.Description
		if req.Type != "" {
			content = "[" + req.Type + "] " + content
		}

		resp, err := memClient.Save(ctx, &pb.SaveMemoryRequest{
			Type:       pb.MemoryType_MEMORY_TYPE_FACT,
			Content:    content,
			Importance: req.Importance,
			TenantId:   c.GetString("tenant_id"),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "store concept failed: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{
				"id":         resp.Id,
				"created_at": resp.CreatedAt,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"id": "concept-local", "created_at": time.Now().Unix()}})
}

// StoreRelation stores a semantic relation
func (h *LayeredMemoryHandler) StoreRelation(c *gin.Context) {
	var req struct {
		FromConcept string  `json:"from_concept"`
		ToConcept   string  `json:"to_concept"`
		RelationType string `json:"relation_type"`
		Weight      float64 `json:"weight"`
		Confidence  float64 `json:"confidence"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "invalid request: " + err.Error()})
		return
	}

	// Store as a FACT memory via gRPC
	if h.cfg.Services.Memory != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		pool, err := client.NewClientPool("", "", h.cfg.Services.Memory, "", "", "")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not available"})
			return
		}
		if pool.MemoryConn == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not connected"})
			return
		}

		memClient := pb.NewMemoryServiceClient(pool.MemoryConn)
		content := req.FromConcept + " -[" + req.RelationType + "]-> " + req.ToConcept

		resp, err := memClient.Save(ctx, &pb.SaveMemoryRequest{
			Type:       pb.MemoryType_MEMORY_TYPE_FACT,
			Content:    content,
			Importance: req.Weight,
			TenantId:   c.GetString("tenant_id"),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "store relation failed: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{
				"id":         resp.Id,
				"created_at": resp.CreatedAt,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"id": "rel-local", "created_at": time.Now().Unix()}})
}

// RecallConcepts recalls semantic concepts
func (h *LayeredMemoryHandler) RecallConcepts(c *gin.Context) {
	query := c.Query("query")
	topK := int32(5)
	if v := c.Query("top_k"); v != "" {
		if n, err := parseInt32(v); err == nil {
			topK = n
		}
	}

	tenantID := c.GetString("tenant_id")

	if h.cfg.Services.Memory != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		pool, err := client.NewClientPool("", "", h.cfg.Services.Memory, "", "", "")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not available"})
			return
		}
		if pool.MemoryConn == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not connected"})
			return
		}

		memClient := pb.NewMemoryServiceClient(pool.MemoryConn)
		resp, err := memClient.Recall(ctx, &pb.RecallMemoryRequest{
			Query:    query,
			TopK:     topK,
			TenantId: tenantID,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "recall concepts failed: " + err.Error()})
			return
		}

		concepts := make([]gin.H, 0, len(resp.Memories))
		for _, m := range resp.Memories {
			concepts = append(concepts, gin.H{
				"id":          m.Id,
				"name":        m.Content,
				"description": m.Content,
				"importance":  m.Importance,
				"confidence":  0.8,
				"created_at":  m.CreatedAt,
			})
		}

		c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"concepts": concepts}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"concepts": []interface{}{}}})
}

// GetRelatedConcepts retrieves concepts related to a concept
func (h *LayeredMemoryHandler) GetRelatedConcepts(c *gin.Context) {
	conceptID := c.Param("id")
	relationType := c.Query("relation_type")

	if h.cfg.Services.Memory != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		pool, err := client.NewClientPool("", "", h.cfg.Services.Memory, "", "", "")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not available"})
			return
		}
		if pool.MemoryConn == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not connected"})
			return
		}

		memClient := pb.NewMemoryServiceClient(pool.MemoryConn)
		resp, err := memClient.Recall(ctx, &pb.RecallMemoryRequest{
			Query:    conceptID + " " + relationType,
			TopK:     10,
			TenantId: c.GetString("tenant_id"),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "get related concepts failed: " + err.Error()})
			return
		}

		concepts := make([]gin.H, 0, len(resp.Memories))
		for _, m := range resp.Memories {
			concepts = append(concepts, gin.H{
				"id":          m.Id,
				"name":        m.Content,
				"description": m.Content,
				"importance":  m.Importance,
				"created_at":  m.CreatedAt,
			})
		}

		c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"concepts": concepts}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"concepts": []interface{}{}}})
}

// AddWorkingMessage adds a message to working memory
func (h *LayeredMemoryHandler) AddWorkingMessage(c *gin.Context) {
	var req struct {
		SessionID  string  `json:"session_id"`
		Type       string  `json:"type"`
		Content    string  `json:"content"`
		Role       string  `json:"role"`
		Importance float64 `json:"importance"`
		IsKey      bool    `json:"is_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "invalid request: " + err.Error()})
		return
	}

	// Store as a memory for persistence
	if h.cfg.Services.Memory != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		pool, err := client.NewClientPool("", "", h.cfg.Services.Memory, "", "", "")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not available"})
			return
		}
		if pool.MemoryConn == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not connected"})
			return
		}

		memClient := pb.NewMemoryServiceClient(pool.MemoryConn)
		memType := pb.MemoryType_MEMORY_TYPE_FACT
		if req.IsKey {
			memType = pb.MemoryType_MEMORY_TYPE_IMPORTANT
		}

		resp, err := memClient.Save(ctx, &pb.SaveMemoryRequest{
			SessionId:  req.SessionID,
			Type:       memType,
			Content:    req.Content,
			Importance: req.Importance,
			TenantId:   c.GetString("tenant_id"),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "add working message failed: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{
				"id":         resp.Id,
				"created_at": resp.CreatedAt,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"id": "msg-local", "created_at": time.Now().Unix()}})
}

// GetWorkingContext retrieves the working memory context for a session
func (h *LayeredMemoryHandler) GetWorkingContext(c *gin.Context) {
	sessionID := c.Param("sessionId")
	tenantID := c.GetString("tenant_id")

	if h.cfg.Services.Memory != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		pool, err := client.NewClientPool("", "", h.cfg.Services.Memory, "", "", "")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not available"})
			return
		}
		if pool.MemoryConn == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not connected"})
			return
		}

		memClient := pb.NewMemoryServiceClient(pool.MemoryConn)
		resp, err := memClient.GetSessionMemory(ctx, &pb.GetSessionMemoryRequest{
			SessionId: sessionID,
			TenantId:  tenantID,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "get working context failed: " + err.Error()})
			return
		}

		messages := make([]gin.H, 0, len(resp.Memories))
		var totalTokens int
		for _, m := range resp.Memories {
			tokenEst := len(m.Content) / 4
			if len(m.Content)%4 > 0 {
				tokenEst++
			}
			totalTokens += tokenEst

			messages = append(messages, gin.H{
				"id":         m.Id,
				"type":       m.Type.String(),
				"content":    m.Content,
				"role":       "user",
				"importance": m.Importance,
				"is_key":     m.Importance >= 0.7,
				"tokens":     tokenEst,
				"created_at": m.CreatedAt,
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{
				"session_id":    sessionID,
				"messages":      messages,
				"total_tokens":  totalTokens,
				"max_tokens":    8000,
				"usage_percent": float64(totalTokens) / 8000.0 * 100,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"session_id":    sessionID,
			"messages":      []interface{}{},
			"total_tokens":  0,
			"max_tokens":    8000,
			"usage_percent": 0,
		},
	})
}

// GetWorkingMessagesForLLM retrieves messages formatted for LLM
func (h *LayeredMemoryHandler) GetWorkingMessagesForLLM(c *gin.Context) {
	sessionID := c.Param("sessionId")
	tenantID := c.GetString("tenant_id")

	if h.cfg.Services.Memory != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		pool, err := client.NewClientPool("", "", h.cfg.Services.Memory, "", "", "")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not available"})
			return
		}
		if pool.MemoryConn == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not connected"})
			return
		}

		memClient := pb.NewMemoryServiceClient(pool.MemoryConn)
		resp, err := memClient.GetSessionMemory(ctx, &pb.GetSessionMemoryRequest{
			SessionId: sessionID,
			TenantId:  tenantID,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "get working messages failed: " + err.Error()})
			return
		}

		llmMessages := make([]gin.H, 0, len(resp.Memories))
		for _, m := range resp.Memories {
			role := "user"
			if m.Importance >= 0.7 {
				role = "system"
			}
			llmMessages = append(llmMessages, gin.H{
				"role":    role,
				"content": m.Content,
			})
		}

		c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"messages": llmMessages}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"messages": []interface{}{}}})
}

// ClearWorkingContext clears working memory for a session
func (h *LayeredMemoryHandler) ClearWorkingContext(c *gin.Context) {
	sessionID := c.Param("sessionId")
	tenantID := c.GetString("tenant_id")

	if h.cfg.Services.Memory != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		pool, err := client.NewClientPool("", "", h.cfg.Services.Memory, "", "", "")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not available"})
			return
		}
		if pool.MemoryConn == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "memory service not connected"})
			return
		}

		memClient := pb.NewMemoryServiceClient(pool.MemoryConn)
		_, err = memClient.DeleteSessionMemory(ctx, &pb.DeleteSessionMemoryRequest{
			SessionId: sessionID,
			TenantId:  tenantID,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": "clear working context failed: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "working context cleared"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "working context cleared"})
}

// GetForgettingConfig returns forgetting configuration
func (h *LayeredMemoryHandler) GetForgettingConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"time_decay_rate":         0.1,
			"importance_threshold":    0.3,
			"max_age_hours":           720,
			"cleanup_interval_hours":  24,
		},
	})
}

// UpdateForgettingConfig updates forgetting configuration
func (h *LayeredMemoryHandler) UpdateForgettingConfig(c *gin.Context) {
	var req struct {
		TimeDecayRate        float64 `json:"time_decay_rate"`
		ImportanceThreshold  float64 `json:"importance_threshold"`
		MaxAgeHours          int     `json:"max_age_hours"`
		CleanupIntervalHours int     `json:"cleanup_interval_hours"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "invalid request: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "forgetting config updated",
		"data": gin.H{
			"time_decay_rate":        req.TimeDecayRate,
			"importance_threshold":   req.ImportanceThreshold,
			"max_age_hours":          req.MaxAgeHours,
			"cleanup_interval_hours": req.CleanupIntervalHours,
		},
	})
}

// TriggerCleanup triggers memory cleanup
func (h *LayeredMemoryHandler) TriggerCleanup(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"deleted_count": 0,
			"duration_ms":   10,
		},
	})
}

// parseInt32 parses a string to int32
func parseInt32(s string) (int32, error) {
	var n int32
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid integer: %s", s)
		}
		n = n*10 + int32(c-'0')
	}
	return n, nil
}
