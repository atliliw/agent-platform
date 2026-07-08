// Package handler provides HTTP handlers for Gateway
package handler

import (
	"agent-platform/pkg/config"

	"github.com/gin-gonic/gin"
)

// KnowledgeHandler - moved to knowledge_handler.go

// A2AHandler - moved to a2a_handler.go (RealA2AHandler)

// HarnessHandler handles Harness requests
type HarnessHandler struct {
	cfg *config.Config
}

// NewHarnessHandler creates a new harness handler
func NewHarnessHandler(cfg *config.Config) *HarnessHandler {
	return &HarnessHandler{cfg: cfg}
}

// CreateRule creates a rule
func (h *HarnessHandler) CreateRule(c *gin.Context) {
	c.JSON(200, gin.H{"code": 0, "data": gin.H{"rule": nil}})
}

// ListRules lists rules
func (h *HarnessHandler) ListRules(c *gin.Context) {
	c.JSON(200, gin.H{"code": 0, "data": gin.H{"rules": []interface{}{}}})
}

// DeleteRule deletes a rule
func (h *HarnessHandler) DeleteRule(c *gin.Context) {
	c.JSON(200, gin.H{"code": 0, "message": "deleted"})
}

// CheckGuardrail checks guardrail
func (h *HarnessHandler) CheckGuardrail(c *gin.Context) {
	c.JSON(200, gin.H{"code": 0, "data": gin.H{"passed": true, "violations": []string{}}})
}

// RunEval runs evaluation
func (h *HarnessHandler) RunEval(c *gin.Context) {
	c.JSON(200, gin.H{"code": 0, "data": gin.H{"run_id": "run-1", "avg_score": 8.5}})
}

// CreateABTest creates A/B test
func (h *HarnessHandler) CreateABTest(c *gin.Context) {
	c.JSON(200, gin.H{"code": 0, "data": gin.H{"ab_test": nil}})
}

// GetABTestResult gets A/B test result
func (h *HarnessHandler) GetABTestResult(c *gin.Context) {
	c.JSON(200, gin.H{"code": 0, "data": gin.H{"result": nil}})
}

// GetSLOStatus gets SLO status
func (h *HarnessHandler) GetSLOStatus(c *gin.Context) {
	c.JSON(200, gin.H{"code": 0, "data": gin.H{"statuses": []interface{}{}}})
}

// Chat handles harness chat
func (h *HarnessHandler) Chat(c *gin.Context) {
	c.JSON(200, gin.H{"code": 0, "data": gin.H{"content": "harness response"}})
}