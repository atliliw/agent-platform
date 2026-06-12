// Package router provides HTTP routing
package router

import (
	"agent-platform/pkg/config"
	"agent-platform/services/gateway/internal/handler"
	"agent-platform/services/gateway/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Setup sets up routes
func Setup(engine *gin.Engine, cfg *config.Config) {
	// Create handlers
	chatHandler := handler.NewChatHandler(cfg)
	knowledgeHandler := handler.NewKnowledgeHandler(cfg)
	memoryHandler := handler.NewRealMemoryHandler(cfg)
	a2aHandler := handler.NewA2AHandler(cfg)
	mcpHandler := handler.NewMCPHandler(cfg)
	harnessHandler := handler.NewRealHarnessHandler(cfg)  // Use real harness handler
	agentHandler := handler.NewAgentHandler(cfg)
	cookieHandler := handler.NewCookieHandler(cfg)  // NEW: Cookie handler

	// API v2 group
	api := engine.Group("/api/v2")
	api.Use(middleware.Tenant())

	// Chat routes
	api.POST("/chat", chatHandler.Chat)
	api.POST("/chat/stream", chatHandler.ChatStream)
	api.GET("/sessions", chatHandler.ListSessions)
	api.GET("/sessions/:id", chatHandler.GetSession)
	api.DELETE("/sessions/:id", chatHandler.DeleteSession)
	api.POST("/multi-agent/chat", chatHandler.MultiAgentChat)

	// Agent routes (NEW!)
	api.POST("/agents", agentHandler.RegisterAgent)
	api.GET("/agents", agentHandler.ListAgents)
	api.GET("/agents/:id", agentHandler.GetAgent)
	api.DELETE("/agents/:id", agentHandler.UnregisterAgent)
	api.POST("/agents/execute", agentHandler.Execute)
	api.POST("/agents/execute/stream", agentHandler.ExecuteStream)
	api.GET("/agents/context/:id", agentHandler.GetContext)

	// Knowledge routes
	api.POST("/knowledge/upload", knowledgeHandler.Upload)
	api.GET("/knowledge/documents", knowledgeHandler.ListDocuments)
	api.GET("/knowledge/documents/:id", knowledgeHandler.GetDocument)
	api.DELETE("/knowledge/documents/:id", knowledgeHandler.DeleteDocument)
	api.POST("/knowledge/search", knowledgeHandler.Search)

	// Memory routes
	api.POST("/memory", memoryHandler.Save)
	api.POST("/memory/recall", memoryHandler.Recall)
	api.GET("/memory/session/:id", memoryHandler.GetSessionMemory)
	api.DELETE("/memory/session/:id", memoryHandler.DeleteSessionMemory)

	// A2A routes
	api.POST("/a2a/discover", a2aHandler.Discover)
	api.POST("/a2a/agents", a2aHandler.RegisterAgent)
	api.GET("/a2a/agents", a2aHandler.ListAgents)
	api.POST("/a2a/tasks/send", a2aHandler.SendTask)
	api.GET("/a2a/tasks/:id", a2aHandler.GetTask)

	// MCP routes
	api.GET("/mcp/tools", mcpHandler.ListTools)
	api.POST("/mcp/call", mcpHandler.CallTool)
	api.POST("/mcp/connect", mcpHandler.Connect)
	api.GET("/mcp/connections", mcpHandler.ListConnections)

	// Harness routes
	api.POST("/harness/rules", harnessHandler.CreateRule)
	api.GET("/harness/rules", harnessHandler.ListRules)
	api.DELETE("/harness/rules/:id", harnessHandler.DeleteRule)
	api.POST("/harness/guardrail/check", harnessHandler.CheckGuardrail)
	api.POST("/harness/eval/run", harnessHandler.RunEval)
	api.POST("/harness/abtest", harnessHandler.CreateABTest)
	api.GET("/harness/abtest/:id/result", harnessHandler.GetABTestResult)
	api.GET("/harness/slo/status", harnessHandler.GetSLOStatus)
	api.POST("/harness/chat", harnessHandler.Chat)

	// Cookie routes (NEW!)
	api.POST("/cookies", cookieHandler.SaveCookies)
	api.GET("/cookies", cookieHandler.GetCookies)
	api.GET("/cookies/all", cookieHandler.GetAllCookies)
	api.DELETE("/cookies", cookieHandler.DeleteCookies)

	// Health check
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "healthy",
			"services": gin.H{
				"chat":     "healthy",
				"knowledge": "healthy",
				"memory":   "healthy",
				"a2a":      "healthy",
				"mcp":      "healthy",
				"harness":  "healthy",
				"agent":    "healthy",
			},
		})
	})

	engine.GET("/ready", func(c *gin.Context) {
		c.JSON(200, gin.H{"ready": true})
	})
}