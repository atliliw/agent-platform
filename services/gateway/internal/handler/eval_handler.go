// Package handler provides HTTP handlers for Gateway
package handler

import (
	"time"

	"github.com/gin-gonic/gin"
)

// EvalHandler handles evaluation requests
type EvalHandler struct{}

// NewEvalHandler creates a new eval handler
func NewEvalHandler() *EvalHandler {
	return &EvalHandler{}
}

// GetSuites returns eval suite list (mock)
func (h *EvalHandler) GetSuites(c *gin.Context) {
	c.JSON(200, gin.H{
		"code": 0,
		"data": []gin.H{
			{
				"id":          "suite-001",
				"name":        "Browser Agent 测试套件",
				"description": "测试 Browser Agent 的核心功能",
				"test_count":  10,
				"created_at":  time.Now().Add(-7 * 24 * time.Hour).Format(time.RFC3339),
				"updated_at":  time.Now().Format(time.RFC3339),
			},
			{
				"id":          "suite-002",
				"name":        "Chat Agent 测试套件",
				"description": "测试 Chat Agent 的对话能力",
				"test_count":  15,
				"created_at":  time.Now().Add(-3 * 24 * time.Hour).Format(time.RFC3339),
				"updated_at":  time.Now().Format(time.RFC3339),
			},
		},
	})
}

// GetResults returns eval results (mock)
func (h *EvalHandler) GetResults(c *gin.Context) {
	suiteID := c.Param("id")
	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"suite_id":     suiteID,
			"suite_name":   "Browser Agent 测试套件",
			"avg_score":    0.85,
			"total_tests":  10,
			"passed_tests": 9,
			"failed_tests": 1,
			"duration_ms":  120000,
			"tokens_used":  50000,
			"cost":         0.1,
			"score_by_category": gin.H{
				"navigation": 0.9,
				"interaction": 0.85,
				"content_extraction": 0.8,
				"form_submission": 0.85,
			},
			"results": []gin.H{
				{
					"id":            "result-001",
					"name":          "导航测试",
					"category":      "navigation",
					"score":         0.9,
					"passed":        true,
					"score_details": gin.H{"faithfulness": 0.9, "relevancy": 0.85, "precision": 0.9, "reasoning": 0.85},
					"duration_ms":   5000,
					"steps":         3,
					"tokens":        1500,
					"cost":          0.003,
				},
				{
					"id":            "result-002",
					"name":          "点击测试",
					"category":      "interaction",
					"score":         0.85,
					"passed":        true,
					"score_details": gin.H{"faithfulness": 0.85, "relevancy": 0.8, "precision": 0.9, "reasoning": 0.8},
					"duration_ms":   3000,
					"steps":         2,
					"tokens":        1000,
					"cost":          0.002,
				},
				{
					"id":            "result-003",
					"name":          "输入测试",
					"category":      "interaction",
					"score":         0.75,
					"passed":        false,
					"score_details": gin.H{"faithfulness": 0.7, "relevancy": 0.75, "precision": 0.8, "reasoning": 0.7},
					"duration_ms":   2000,
					"steps":         1,
					"tokens":        800,
					"cost":          0.0016,
					"error":         "输入超时",
				},
			},
			"regressions": []gin.H{},
			"metrics_summary": gin.H{
				"avg_steps":            2.5,
				"avg_latency_ms":       4000,
				"total_tool_calls":     15,
				"tool_success_rate":    0.9,
				"avg_tokens_per_test":  1500,
				"avg_cost_per_test":    0.01,
			},
			"created_at": time.Now().Format(time.RFC3339),
		},
	})
}

// RunEval runs evaluation (mock)
func (h *EvalHandler) RunEval(c *gin.Context) {
	suiteID := c.Param("id")
	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"suite_id":     suiteID,
			"suite_name":   "Browser Agent 测试套件",
			"avg_score":    0.85,
			"total_tests":  10,
			"passed_tests": 9,
			"failed_tests": 1,
			"duration_ms":  120000,
			"tokens_used":  50000,
			"cost":         0.1,
			"score_by_category": gin.H{
				"navigation": 0.9,
				"interaction": 0.85,
				"content_extraction": 0.8,
			},
			"results": []gin.H{
				{"id": "result-001", "name": "导航测试", "category": "navigation", "score": 0.9, "passed": true},
				{"id": "result-002", "name": "点击测试", "category": "interaction", "score": 0.85, "passed": true},
			},
			"created_at": time.Now().Format(time.RFC3339),
		},
	})
}