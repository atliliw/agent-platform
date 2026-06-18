// Package handler provides HTTP handlers
package handler

import (
	"agent-platform/pkg/config"

	"github.com/gin-gonic/gin"
)

// UserHandler handles user-related requests
type UserHandler struct {
	cfg *config.Config
}

// NewUserHandler creates a new user handler
func NewUserHandler(cfg *config.Config) *UserHandler {
	return &UserHandler{cfg: cfg}
}

// GetUserInfo returns mock user info
// GET /api/v2/user/info
func (h *UserHandler) GetUserInfo(c *gin.Context) {
	// 返回一个默认用户信息
	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"id":        "default-user",
			"name":      "默认用户",
			"email":     "user@example.com",
			"avatar":    "",
			"role":      "admin",
			"tenant_id": c.GetString("tenant_id"),
		},
	})
}

// GetUserSettings returns user settings
// GET /api/v2/user/settings
func (h *UserHandler) GetUserSettings(c *gin.Context) {
	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"theme":    "light",
			"language": "zh-CN",
			"timezone": "Asia/Shanghai",
			"preferences": gin.H{
				"auto_save":     true,
				"notifications": true,
			},
		},
	})
}

// UpdateUserSettings updates user settings
// PUT /api/v2/user/settings
func (h *UserHandler) UpdateUserSettings(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"code":    400,
			"message": "Invalid request",
		})
		return
	}

	c.JSON(200, gin.H{
		"code":    0,
		"message": "Settings updated",
		"data":    req,
	})
}
