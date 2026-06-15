// Package handler provides HTTP handlers for Gateway
package handler

import (
	"time"

	"github.com/gin-gonic/gin"
)

// CostHandler handles cost monitoring requests
type CostHandler struct{}

// NewCostHandler creates a new cost handler
func NewCostHandler() *CostHandler {
	return &CostHandler{}
}

// GetSummary returns cost summary (mock)
func (h *CostHandler) GetSummary(c *gin.Context) {
	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"total_cost":    0.089,
			"total_calls":   156,
			"total_tokens":  45000,
			"input_tokens":  30000,
			"output_tokens": 15000,
			"avg_latency_ms": 2500,
			"cost_trend":     12.5,
			"by_model": []gin.H{
				{"model": "qwen-plus", "calls": 80, "tokens": 25000, "cost": 0.05, "avg_latency_ms": 2000},
				{"model": "qwen-turbo", "calls": 50, "tokens": 15000, "cost": 0.03, "avg_latency_ms": 1000},
				{"model": "qwen-max", "calls": 26, "tokens": 5000, "cost": 0.009, "avg_latency_ms": 3000},
			},
			"by_agent": []gin.H{
				{"agent_id": "browser-agent", "calls": 45, "tokens": 20000, "cost": 0.04, "success_rate": 95},
				{"agent_id": "chat-agent", "calls": 80, "tokens": 15000, "cost": 0.03, "success_rate": 100},
				{"agent_id": "task-agent", "calls": 31, "tokens": 10000, "cost": 0.019, "success_rate": 100},
			},
			"by_tool": []gin.H{
				{"tool": "browser_execute", "calls": 30, "tokens": 12000, "cost": 0.025, "success_rate": 93},
				{"tool": "web_search", "calls": 25, "tokens": 8000, "cost": 0.018, "success_rate": 100},
				{"tool": "calculator", "calls": 15, "tokens": 2000, "cost": 0.005, "success_rate": 100},
			},
			"by_date": []gin.H{
				{"date": "2026-06-13", "calls": 50, "tokens": 15000, "cost": 0.03},
				{"date": "2026-06-14", "calls": 23, "tokens": 8000, "cost": 0.015},
			},
		},
	})
}

// GetTrend returns cost trend data (mock)
func (h *CostHandler) GetTrend(c *gin.Context) {
	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"timestamps": []string{
				time.Now().Add(-6 * time.Hour).Format("15:04"),
				time.Now().Add(-5 * time.Hour).Format("15:04"),
				time.Now().Add(-4 * time.Hour).Format("15:04"),
				time.Now().Add(-3 * time.Hour).Format("15:04"),
				time.Now().Add(-2 * time.Hour).Format("15:04"),
				time.Now().Add(-1 * time.Hour).Format("15:04"),
				time.Now().Format("15:04"),
			},
			"costs":   []float64{0.008, 0.012, 0.015, 0.010, 0.018, 0.015, 0.011},
			"tokens":  []int{2500, 4000, 5000, 3200, 6000, 5000, 3600},
			"calls":   []int{8, 12, 15, 10, 18, 15, 11},
		},
	})
}

// GetBudgets returns budget list (mock)
func (h *CostHandler) GetBudgets(c *gin.Context) {
	c.JSON(200, gin.H{
		"code": 0,
		"data": []gin.H{
			{
				"id":           "budget-001",
				"name":         "日预算",
				"limit":        0.1,
				"used":         0.015,
				"percent_used": 15,
				"period":       "daily",
				"alert_threshold": 80,
				"exceeded":     false,
				"created_at":   time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
				"updated_at":   time.Now().Format(time.RFC3339),
			},
			{
				"id":           "budget-002",
				"name":         "周预算",
				"limit":        0.5,
				"used":         0.089,
				"percent_used": 17.8,
				"period":       "weekly",
				"alert_threshold": 70,
				"exceeded":     false,
				"created_at":   time.Now().Add(-7 * 24 * time.Hour).Format(time.RFC3339),
				"updated_at":   time.Now().Format(time.RFC3339),
			},
		},
	})
}

// CreateBudget creates a new budget (mock)
func (h *CostHandler) CreateBudget(c *gin.Context) {
	var req struct {
		Name           string  `json:"name"`
		Limit          float64 `json:"limit"`
		Period         string  `json:"period"`
		AlertThreshold int     `json:"alert_threshold"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "error": "invalid request"})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"id":             "budget-" + time.Now().Format("20060102150405"),
			"name":           req.Name,
			"limit":          req.Limit,
			"used":           0,
			"percent_used":   0,
			"period":         req.Period,
			"alert_threshold": req.AlertThreshold,
			"exceeded":       false,
			"created_at":     time.Now().Format(time.RFC3339),
			"updated_at":     time.Now().Format(time.RFC3339),
		},
	})
}

// GetDetails returns cost details (mock)
func (h *CostHandler) GetDetails(c *gin.Context) {
	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"details": []gin.H{
				{
					"id":             "detail-001",
					"timestamp":      time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
					"session_id":     "session-001",
					"agent_id":       "browser-agent",
					"model":          "qwen-plus",
					"input_tokens":   800,
					"output_tokens":  400,
					"total_tokens":   1200,
					"cost":           0.0024,
					"latency_ms":     50000,
					"success":        true,
					"operation":      "browser_navigate",
				},
				{
					"id":             "detail-002",
					"timestamp":      time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
					"session_id":     "session-002",
					"agent_id":       "chat-agent",
					"model":          "qwen-turbo",
					"input_tokens":   500,
					"output_tokens":  300,
					"total_tokens":   800,
					"cost":           0.0016,
					"latency_ms":     1500,
					"success":        true,
					"operation":      "chat_response",
				},
			},
			"total": 2,
			"page":  1,
			"size":  20,
		},
	})
}