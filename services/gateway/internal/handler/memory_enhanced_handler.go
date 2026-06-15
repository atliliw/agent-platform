// Package handler provides HTTP handlers for Gateway
package handler

import (
	"time"

	"github.com/gin-gonic/gin"
)

// MemoryEnhancedHandler handles enhanced memory requests
type MemoryEnhancedHandler struct{}

// NewMemoryEnhancedHandler creates a new memory enhanced handler
func NewMemoryEnhancedHandler() *MemoryEnhancedHandler {
	return &MemoryEnhancedHandler{}
}

// GetStats returns memory statistics (mock)
func (h *MemoryEnhancedHandler) GetStats(c *gin.Context) {
	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"working_memory_count":  5,
			"episodic_memory_count": 12,
			"semantic_memory_count": 8,
			"procedural_memory_count": 3,
			"total_size_bytes":      1024000,
			"avg_importance":         0.65,
			"oldest_memory":          time.Now().Add(-7 * 24 * time.Hour).Format(time.RFC3339),
			"newest_memory":          time.Now().Format(time.RFC3339),
		},
	})
}

// GetTimeline returns memory timeline (mock)
func (h *MemoryEnhancedHandler) GetTimeline(c *gin.Context) {
	c.JSON(200, gin.H{
		"code": 0,
		"data": []gin.H{
			{
				"id":          "event-001",
				"timestamp":    time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
				"event_type":   "browser_navigate",
				"title":         "导航到CSDN",
				"description":   "用户请求导航到CSDN博客",
				"importance":    0.8,
				"related_memories": []string{"mem-001", "mem-002"},
			},
			{
				"id":          "event-002",
				"timestamp":    time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
				"event_type":   "chat_conversation",
				"title":         "对话讨论",
				"description":   "讨论了Agent架构设计",
				"importance":    0.6,
				"related_memories": []string{"mem-003"},
			},
			{
				"id":          "event-003",
				"timestamp":    time.Now().Add(-3 * time.Hour).Format(time.RFC3339),
				"event_type":   "tool_execution",
				"title":         "工具执行",
				"description":   "执行了web_search工具",
				"importance":    0.7,
				"related_memories": []string{},
			},
		},
	})
}

// GetGraph returns knowledge graph (mock)
func (h *MemoryEnhancedHandler) GetGraph(c *gin.Context) {
	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"nodes": []gin.H{
				{"id": "node-1", "type": "concept", "name": "Agent", "properties": gin.H{"category": "AI", "confidence": 0.9}},
				{"id": "node-2", "type": "concept", "name": "Browser", "properties": gin.H{"category": "Tool", "confidence": 0.85}},
				{"id": "node-3", "type": "concept", "name": "CSDN", "properties": gin.H{"category": "Platform", "confidence": 0.95}},
				{"id": "node-4", "type": "entity", "name": "用户", "properties": gin.H{"type": "Person", "confidence": 1.0}},
				{"id": "node-5", "type": "event", "name": "博客发布", "properties": gin.H{"type": "Action", "confidence": 0.8}},
			},
			"edges": []gin.H{
				{"id": "edge-1", "source_id": "node-1", "target_id": "node-2", "relation": "uses", "properties": gin.H{"strength": 0.9}},
				{"id": "edge-2", "source_id": "node-1", "target_id": "node-4", "relation": "serves", "properties": gin.H{"strength": 1.0}},
				{"id": "edge-3", "source_id": "node-2", "target_id": "node-3", "relation": "navigates", "properties": gin.H{"strength": 0.85}},
				{"id": "edge-4", "source_id": "node-3", "target_id": "node-5", "relation": "enables", "properties": gin.H{"strength": 0.8}},
			},
		},
	})
}

// GetEpisodicMemories returns episodic memories (mock)
func (h *MemoryEnhancedHandler) GetEpisodicMemories(c *gin.Context) {
	c.JSON(200, gin.H{
		"code": 0,
		"data": []gin.H{
			{
				"id":          "epi-001",
				"session_id":  "session-001",
				"agent_id":    "browser-agent",
				"event_type":  "navigation",
				"title":       "导航到CSDN博客",
				"description": "成功导航到CSDN博客页面",
				"importance":  0.8,
				"started_at":  time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
				"ended_at":    time.Now().Add(-1 * time.Hour).Add(5 * time.Minute).Format(time.RFC3339),
			},
		},
	})
}

// GetSemanticMemories returns semantic memories (mock)
func (h *MemoryEnhancedHandler) GetSemanticMemories(c *gin.Context) {
	c.JSON(200, gin.H{
		"code": 0,
		"data": []gin.H{
			{
				"id":          "sem-001",
				"concept":     "CSDN",
				"category":    "Platform",
				"description": "中国最大的开发者社区",
				"confidence":  0.95,
				"created_at":  time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
			},
		},
	})
}

// GetProceduralMemories returns procedural memories (mock)
func (h *MemoryEnhancedHandler) GetProceduralMemories(c *gin.Context) {
	c.JSON(200, gin.H{
		"code": 0,
		"data": []gin.H{
			{
				"id":           "proc-001",
				"name":         "发布CSDN博客",
				"description":  "自动化发布博客到CSDN的流程",
				"category":     "blog_publishing",
				"success_rate":  0.95,
				"usage_count":  10,
				"created_at":   time.Now().Add(-7 * 24 * time.Hour).Format(time.RFC3339),
			},
		},
	})
}

// Consolidate triggers memory consolidation (mock)
func (h *MemoryEnhancedHandler) Consolidate(c *gin.Context) {
	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"processed_count":  15,
			"created_memories": []string{"mem-new-001", "mem-new-002"},
			"updated_memories": []string{"mem-001"},
			"deleted_memories": []string{},
			"duration_ms":      150,
		},
	})
}

// Search searches memories (mock)
func (h *MemoryEnhancedHandler) Search(c *gin.Context) {
	var req struct {
		Query         string `json:"query"`
		Types         []string `json:"types"`
		Limit         int    `json:"limit"`
		MinImportance float64 `json:"min_importance"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "error": "invalid request"})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": []gin.H{
			{
				"id":              "mem-001",
				"type":            "episodic",
				"content":         "成功发布博客到CSDN",
				"relevance_score":  0.92,
				"importance":      0.8,
				"created_at":      time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
			},
			{
				"id":              "mem-002",
				"type":            "semantic",
				"content":         "CSDN是中国最大的开发者社区",
				"relevance_score":  0.88,
				"importance":      0.9,
				"created_at":      time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
			},
		},
	})
}