// Package middleware provides HTTP middleware
package middleware

import (
	"fmt"
	"strconv"
	"time"

	"agent-platform/pkg/redis"
	"github.com/gin-gonic/gin"
)

// Logger returns a logger middleware
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		gin.DefaultWriter.Write([]byte(
			time.Now().Format("2006/01/02 - 15:04:05") +
				" | " + c.Request.Method +
				" | " + path +
				" | " + time.Duration(latency).String() +
				" | " + strconv.Itoa(status) + "\n",
		))
	}
}

// CORS returns a CORS middleware
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Tenant-ID")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// Tenant extracts tenant ID from header
func Tenant() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetHeader("X-Tenant-ID")
		if tenantID == "" {
			tenantID = "default"
		}
		c.Set("tenant_id", tenantID)
		c.Next()
	}
}

// RateLimit returns a rate limiting middleware
func RateLimit(redisClient *redis.Client, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, exists := c.Get("tenant_id")
		if !exists {
			tenantID = "default"
		}

		tenantIDStr, ok := tenantID.(string)
		if !ok {
			tenantIDStr = "default"
		}

		key := "ratelimit:" + tenantIDStr + ":" + c.FullPath()

		allowed, remaining, err := redisClient.RateLimit(c.Request.Context(), key, limit, window)
		if err != nil {
			c.Next()
			return
		}

		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

		if !allowed {
			c.JSON(429, gin.H{
				"code":    20004,
				"message": "rate limit exceeded",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Auth returns an authentication middleware
func Auth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(401, gin.H{
				"code":    10002,
				"message": "unauthorized",
			})
			c.Abort()
			return
		}

		// TODO: Validate JWT token
		// For now, just pass through
		c.Next()
	}
}