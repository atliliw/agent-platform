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
	harnessHandler := handler.NewRealHarnessHandler(cfg) // Use real harness handler
	agentHandler := handler.NewAgentHandler(cfg)
	cookieHandler := handler.NewCookieHandler(cfg) // NEW: Cookie handler
	userHandler := handler.NewUserHandler(cfg)     // NEW: User handler

	// NEW: Observability handlers (mock)
	observabilityHandler := handler.NewObservabilityHandler()
	costHandler := handler.NewCostHandler()
	memoryEnhancedHandler := handler.NewMemoryEnhancedHandler()
	evalHandler := handler.NewEvalHandler()

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
	api.GET("/memory/all", memoryHandler.GetAllMemories)
	api.GET("/memory/session/:id", memoryHandler.GetSessionMemory)
	api.DELETE("/memory/session/:id", memoryHandler.DeleteSessionMemory)
	api.DELETE("/memory/session/clear", memoryHandler.DeleteAllSessionMemories)
	api.DELETE("/memory/:id", memoryHandler.DeleteMemory)

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
	api.POST("/harness/abtest/list", harnessHandler.ListABTests)
	api.DELETE("/harness/abtest/:id", harnessHandler.DeleteABTest)
	api.GET("/harness/abtest/:id/result", harnessHandler.GetABTestResult)
	api.GET("/harness/slo/status", harnessHandler.GetSLOStatus)
	api.POST("/harness/slo", harnessHandler.CreateSLO)
	api.GET("/harness/llm/metrics", harnessHandler.GetLLMMetrics)
	api.POST("/harness/chat", harnessHandler.Chat)

	// Feature Flag routes (NEW!)
	api.POST("/harness/flags", harnessHandler.CreateFeatureFlag)
	api.GET("/harness/flags", harnessHandler.ListFeatureFlags)
	api.PUT("/harness/flags/toggle", harnessHandler.ToggleFeatureFlag)
	api.POST("/harness/flags/evaluate", harnessHandler.EvaluateFeatureFlag)

	// Chaos routes (NEW!)
	api.POST("/harness/chaos", harnessHandler.CreateChaosExperiment)
	api.GET("/harness/chaos", harnessHandler.ListChaosExperiments)
	api.POST("/harness/chaos/:id/start", harnessHandler.StartChaosExperiment)
	api.POST("/harness/chaos/:id/stop", harnessHandler.StopChaosExperiment)

	// Cost routes (NEW!)
	api.GET("/harness/cost/report", harnessHandler.GetCostReport)
	api.POST("/harness/cost/pricing", harnessHandler.SetModelPricing)
	api.GET("/harness/cost/pricing", harnessHandler.ListModelPricing)
	api.GET("/harness/cost/recommendations", harnessHandler.GetCostRecommendations)
	api.POST("/harness/cost/usage", harnessHandler.RecordCostUsage)

	// Proposal routes (NEW!)
	api.POST("/harness/proposals", harnessHandler.CreateProposal)
	api.GET("/harness/proposals", harnessHandler.ListProposals)
	api.POST("/harness/proposals/:id/approve", harnessHandler.ApproveProposal)
	api.POST("/harness/proposals/:id/reject", harnessHandler.RejectProposal)
	api.POST("/harness/proposals/:id/execute", harnessHandler.ExecuteProposal)
	api.POST("/harness/proposals/analyze", harnessHandler.AnalyzeAndPropose)

	// Catalog routes (NEW!)
	api.GET("/harness/catalog/:id", harnessHandler.GetCatalogAgent)
	api.POST("/harness/catalog", harnessHandler.RegisterCatalogAgent)
	api.POST("/harness/catalog/usage", harnessHandler.RecordCatalogUsage)
	api.POST("/harness/catalog/rate", harnessHandler.RateCatalogAgent)

	// Rollback routes (NEW!)
	api.POST("/harness/rollback/configs", harnessHandler.CreateRollbackConfig)
	api.GET("/harness/rollback/configs/:id", harnessHandler.GetRollbackConfig)
	api.POST("/harness/rollback/snapshots", harnessHandler.TakeSnapshot)
	api.GET("/harness/rollback/configs/:id/snapshots", harnessHandler.ListSnapshots)
	api.POST("/harness/rollback/execute", harnessHandler.ExecuteRollback)

	// Golden Path routes (NEW!)
	api.POST("/harness/goldenpath/templates", harnessHandler.CreateGoldenPathTemplate)
	api.GET("/harness/goldenpath/templates", harnessHandler.ListGoldenPathTemplates)
	api.POST("/harness/goldenpath/instantiate", harnessHandler.InstantiateTemplate)

	// Optimizer routes (NEW!)
	api.POST("/harness/evolve/optimize", harnessHandler.RunOptimizer)
	api.GET("/harness/catalog", harnessHandler.ListCatalogAgents)

	// RCA routes (NEW!)
	api.POST("/harness/rca/changes", harnessHandler.RecordChange)
	api.GET("/harness/rca/incidents/:id/analyze", harnessHandler.AnalyzeIncident)

	// Cookie routes (NEW!)
	api.POST("/cookies", cookieHandler.SaveCookies)
	api.GET("/cookies", cookieHandler.GetCookies)
	api.GET("/cookies/all", cookieHandler.GetAllCookies)
	api.DELETE("/cookies", cookieHandler.DeleteCookies)

	// Observability routes (NEW!)
	api.GET("/observability/traces", observabilityHandler.GetTraces)
	api.GET("/observability/traces/:id", observabilityHandler.GetTrace)
	api.GET("/observability/metrics", observabilityHandler.GetMetrics)
	api.GET("/observability/profile/:id", observabilityHandler.GetProfile)
	api.GET("/observability/stats", observabilityHandler.GetStats)

	// Cost routes (NEW!)
	api.GET("/cost/summary", costHandler.GetSummary)
	api.GET("/cost/trend", costHandler.GetTrend)
	api.GET("/cost/budgets", costHandler.GetBudgets)
	api.POST("/cost/budgets", costHandler.CreateBudget)
	api.GET("/cost/details", costHandler.GetDetails)

	// Memory Enhanced routes (NEW!)
	api.GET("/memory-enhanced/stats", memoryEnhancedHandler.GetStats)
	api.GET("/memory-enhanced/timeline", memoryEnhancedHandler.GetTimeline)
	api.GET("/memory-enhanced/graph", memoryEnhancedHandler.GetGraph)
	api.GET("/memory-enhanced/episodic", memoryEnhancedHandler.GetEpisodicMemories)
	api.GET("/memory-enhanced/semantic", memoryEnhancedHandler.GetSemanticMemories)
	api.GET("/memory-enhanced/procedural", memoryEnhancedHandler.GetProceduralMemories)
	api.POST("/memory-enhanced/consolidate", memoryEnhancedHandler.Consolidate)
	api.POST("/memory-enhanced/search", memoryEnhancedHandler.Search)

	// Eval routes (NEW!)
	api.GET("/eval/suites", evalHandler.GetSuites)
	api.GET("/eval/suites/:id/results", evalHandler.GetResults)
	api.POST("/eval/suites/:id/run", evalHandler.RunEval)

	// User routes (NEW!)
	api.GET("/user/info", userHandler.GetUserInfo)
	api.GET("/user/settings", userHandler.GetUserSettings)
	api.PUT("/user/settings", userHandler.UpdateUserSettings)
	// Scheduler routes (NEW!)
	api.POST("/harness/scheduler/schedules", harnessHandler.SetEvalSchedule)
	api.GET("/harness/scheduler/schedules", harnessHandler.ListEvalSchedules)
	api.GET("/harness/scheduler/schedules/:id", harnessHandler.GetEvalSchedule)
	api.PUT("/harness/scheduler/schedules/:id/pause", harnessHandler.PauseEvalSchedule)
	api.PUT("/harness/scheduler/schedules/:id/resume", harnessHandler.ResumeEvalSchedule)
	api.DELETE("/harness/scheduler/schedules/:id", harnessHandler.DeleteEvalSchedule)
	api.POST("/harness/scheduler/schedules/:id/run", harnessHandler.RunEvalScheduleNow)
	api.GET("/harness/scheduler/schedules/:id/results", harnessHandler.GetEvalScheduleResults)
	api.GET("/harness/scheduler/status", harnessHandler.GetSchedulerStatus)
	api.POST("/harness/scheduler/control", harnessHandler.SchedulerControl)
	api.GET("/harness/scheduler/stats", harnessHandler.GetSchedulerStats)

	// Session Replay routes (NEW!)
	api.POST("/harness/session", harnessHandler.CreateSession)
	api.GET("/harness/session/list", harnessHandler.ListSessionReplays)
	api.GET("/harness/session/stats", harnessHandler.GetSessionStats)
	api.GET("/harness/session/:id", harnessHandler.GetSessionReplay)
	api.GET("/harness/session/:id/graph", harnessHandler.GetSessionGraph)
	api.POST("/harness/session/:id/replay", harnessHandler.ReplaySession)
	api.GET("/harness/session/:id/export", harnessHandler.ExportSession)
	api.DELETE("/harness/session/:id", harnessHandler.DeleteSessionReplay)

	// Prompt Management routes (NEW!)
	api.POST("/harness/prompt", harnessHandler.CreatePrompt)
	api.GET("/harness/prompt/list", harnessHandler.ListPrompts)
	api.GET("/harness/prompt/:key", harnessHandler.GetPrompt)
	api.DELETE("/harness/prompt/:key", harnessHandler.DeletePrompt)
	api.POST("/harness/prompt/:key/versions", harnessHandler.CreatePromptVersion)
	api.GET("/harness/prompt/:key/versions", harnessHandler.ListPromptVersions)
	api.GET("/harness/prompt/:key/active", harnessHandler.GetActivePromptVersion)
	api.POST("/harness/prompt/version/activate", harnessHandler.ActivatePromptVersion)
	api.POST("/harness/prompt/compare", harnessHandler.ComparePromptVersions)
	api.POST("/harness/prompt/render", harnessHandler.RenderPrompt)
	api.GET("/harness/prompt/performance/:versionId", harnessHandler.GetPromptPerformance)

	// Red Team Testing routes (NEW!)
	api.POST("/harness/redteam", harnessHandler.CreateRedTeamTest)
	api.GET("/harness/redteam/list", harnessHandler.ListRedTeamTests)
	api.GET("/harness/redteam/payloads", harnessHandler.GetAttackPayloads)
	api.GET("/harness/redteam/:id", harnessHandler.GetRedTeamTest)
	api.POST("/harness/redteam/:id/run", harnessHandler.RunRedTeamTest)
	api.GET("/harness/redteam/:id/report", harnessHandler.GetRedTeamReport)
	api.GET("/harness/redteam/:id/attacks", harnessHandler.ListRedTeamAttacks)
	api.DELETE("/harness/redteam/:id", harnessHandler.DeleteRedTeamTest)

	// RAG Metrics routes (NEW!)
	api.POST("/harness/rag/evaluate", harnessHandler.EvaluateRAG)
	api.POST("/harness/rag/batch-evaluate", harnessHandler.BatchEvaluateRAG)
	api.GET("/harness/rag/metrics", harnessHandler.ListRAGMetrics)
	api.GET("/harness/rag/metrics/:id", harnessHandler.GetRAGMetrics)
	api.POST("/harness/rag/evaluation", harnessHandler.CreateRAGEvaluation)
	api.GET("/harness/rag/evaluations", harnessHandler.ListRAGEvaluations)
	api.POST("/harness/rag/evaluation/:id/run", harnessHandler.RunRAGEvaluation)

	// LLM Gateway routes (NEW!)
	api.POST("/harness/gateway/chat", harnessHandler.GatewayChat)
	api.POST("/harness/gateway/config", harnessHandler.CreateGatewayConfig)
	api.GET("/harness/gateway/config", harnessHandler.ListGatewayConfigs)
	api.GET("/harness/gateway/config/:id", harnessHandler.GetGatewayConfig)
	api.PUT("/harness/gateway/config/:id", harnessHandler.UpdateGatewayConfig)
	api.DELETE("/harness/gateway/config/:id", harnessHandler.DeleteGatewayConfig)
	api.GET("/harness/gateway/stats", harnessHandler.GetGatewayStats)
	api.POST("/harness/gateway/route", harnessHandler.CreateGatewayRoute)
	api.GET("/harness/gateway/routes", harnessHandler.ListGatewayRoutes)
	api.DELETE("/harness/gateway/route/:id", harnessHandler.DeleteGatewayRoute)
	api.POST("/harness/gateway/strategy", harnessHandler.SetLoadBalanceStrategy)

	// Playground routes (NEW!)
	api.POST("/harness/playground/execute", harnessHandler.ExecutePlayground)
	api.POST("/harness/playground/compare", harnessHandler.CompareModels)
	api.GET("/harness/playground/history", harnessHandler.GetPlaygroundHistory)
	api.GET("/harness/playground/stats", harnessHandler.GetPlaygroundStats)

	// Health check
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "healthy",
			"services": gin.H{
				"chat":      "healthy",
				"knowledge": "healthy",
				"memory":    "healthy",
				"a2a":       "healthy",
				"mcp":       "healthy",
				"harness":   "healthy",
				"agent":     "healthy",
			},
		})
	})

	engine.GET("/ready", func(c *gin.Context) {
		c.JSON(200, gin.H{"ready": true})
	})
}
