// Package handler provides HTTP handlers
package handler

import (
	"net/http"
	"sync"

	"agent-platform/pkg/config"
	"agent-platform/pkg/browseragent"

	"github.com/gin-gonic/gin"
)

// CookieHandler handles cookie management
type CookieHandler struct {
	cfg    *config.Config
	store  *browseragent.MemoryCookieStorage
	mu     sync.RWMutex
}

// NewCookieHandler creates a new cookie handler
func NewCookieHandler(cfg *config.Config) *CookieHandler {
	return &CookieHandler{
		cfg:   cfg,
		store: browseragent.NewMemoryCookieStorage(),
	}
}

// CookieRequest represents a cookie save request
type CookieRequest struct {
	UserID   string              `json:"user_id"`
	TenantID string              `json:"tenant_id"`
	Domain   string              `json:"domain"`
	Cookies  []browseragent.Cookie `json:"cookies"`
}

// SaveCookies saves cookies for a user/domain
// POST /api/v2/cookies
func (h *CookieHandler) SaveCookies(c *gin.Context) {
	var req CookieRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    10001,
			"message": "invalid request: " + err.Error(),
		})
		return
	}

	if req.UserID == "" {
		req.UserID = "default"
	}
	if req.TenantID == "" {
		req.TenantID = c.GetHeader("X-Tenant-ID")
		if req.TenantID == "" {
			req.TenantID = "default"
		}
	}

	if err := h.store.Save(c.Request.Context(), req.UserID, req.TenantID, req.Cookies); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    10005,
			"message": "failed to save cookies: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "cookies saved",
		"data": gin.H{
			"user_id":   req.UserID,
			"tenant_id": req.TenantID,
			"domain":    req.Domain,
			"count":     len(req.Cookies),
		},
	})
}

// GetCookies gets cookies for a domain
// GET /api/v2/cookies?domain=.csdn.net&user_id=xxx
func (h *CookieHandler) GetCookies(c *gin.Context) {
	domain := c.Query("domain")
	userID := c.Query("user_id")
	tenantID := c.Query("tenant_id")

	if userID == "" {
		userID = "default"
	}
	if tenantID == "" {
		tenantID = c.GetHeader("X-Tenant-ID")
		if tenantID == "" {
			tenantID = "default"
		}
	}

	cookies, err := h.store.Get(c.Request.Context(), userID, tenantID, domain)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    10005,
			"message": "failed to get cookies: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"cookies":  cookies,
			"domain":   domain,
			"user_id":  userID,
			"tenant_id": tenantID,
		},
	})
}

// GetAllCookies gets all cookies for a user
// GET /api/v2/cookies/all?user_id=xxx
func (h *CookieHandler) GetAllCookies(c *gin.Context) {
	userID := c.Query("user_id")
	tenantID := c.Query("tenant_id")

	if userID == "" {
		userID = "default"
	}
	if tenantID == "" {
		tenantID = c.GetHeader("X-Tenant-ID")
		if tenantID == "" {
			tenantID = "default"
		}
	}

	cookies, err := h.store.GetAll(c.Request.Context(), userID, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    10005,
			"message": "failed to get cookies: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"cookies":   cookies,
			"user_id":   userID,
			"tenant_id": tenantID,
		},
	})
}

// DeleteCookies deletes cookies for a domain
// DELETE /api/v2/cookies?domain=.csdn.net&user_id=xxx
func (h *CookieHandler) DeleteCookies(c *gin.Context) {
	domain := c.Query("domain")
	userID := c.Query("user_id")
	tenantID := c.Query("tenant_id")

	if userID == "" {
		userID = "default"
	}
	if tenantID == "" {
		tenantID = c.GetHeader("X-Tenant-ID")
		if tenantID == "" {
			tenantID = "default"
		}
	}

	if err := h.store.Delete(c.Request.Context(), userID, tenantID, domain); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    10005,
			"message": "failed to delete cookies: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "cookies deleted",
	})
}

// GetStore returns the underlying cookie store (for use by other handlers)
func (h *CookieHandler) GetStore() *browseragent.MemoryCookieStorage {
	return h.store
}
