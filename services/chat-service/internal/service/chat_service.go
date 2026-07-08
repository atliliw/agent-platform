// Package service provides business logic for chat service
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"agent-platform/pkg/config"
	"agent-platform/pkg/llm"
	"agent-platform/pkg/qdrant"
	pb "agent-platform/pkg/pb/chat"
	commonpb "agent-platform/pkg/pb/common"
	mcppb "agent-platform/pkg/pb/mcp"
	agentpb "agent-platform/pkg/pb/agent"
	memorypb "agent-platform/pkg/pb/memory"
	harnesspb "agent-platform/pkg/pb/harness"
	"agent-platform/pkg/client"
	"agent-platform/services/chat-service/internal/repository"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ChatService provides chat functionality with Agent capabilities
type ChatService struct {
	pb.UnimplementedChatServiceServer
	llmClient     llm.Client
	qdrant        *qdrant.Client
	sessionRepo   *repository.SessionRepository
	mcpClient     client.MCPClient
	agentClient   agentpb.AgentServiceClient
	agentConn     *grpc.ClientConn
	memoryClient  memorypb.MemoryServiceClient // 长期记忆客户端
	memoryConn    *grpc.ClientConn
	harnessClient harnesspb.HarnessServiceClient // 规则引擎客户端
	harnessConn   *grpc.ClientConn
	cfg           *config.Config
	maxSteps      int
	useMultiAgent bool
	enableMemory  bool // 是否启用长期记忆
	enableRules   bool // 是否启用规则检查
}

// NewChatService creates a new chat service
func NewChatService(llmClient llm.Client, qdrant *qdrant.Client, sessionRepo *repository.SessionRepository, mcpClient client.MCPClient, cfg *config.Config) *ChatService {
	// 尝试连接 Agent Service
	agentAddr := cfg.Services.Agent
	var agentClient agentpb.AgentServiceClient
	var agentConn *grpc.ClientConn

	if agentAddr != "" {
		conn, err := grpc.Dial(agentAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(10*1024*1024)), // 10MB
		)
		if err == nil {
			agentClient = agentpb.NewAgentServiceClient(conn)
			agentConn = conn
			fmt.Printf("Connected to Agent Service at %s\n", agentAddr)
		} else {
			fmt.Printf("Warning: Failed to connect to Agent Service: %v\n", err)
		}
	}

	// ★ 尝试连接 Memory Service
	memoryAddr := cfg.Services.Memory
	var memoryClient memorypb.MemoryServiceClient
	var memoryConn *grpc.ClientConn

	if memoryAddr != "" {
		conn, err := grpc.Dial(memoryAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err == nil {
			memoryClient = memorypb.NewMemoryServiceClient(conn)
			memoryConn = conn
			fmt.Printf("Connected to Memory Service at %s\n", memoryAddr)
		} else {
			fmt.Printf("Warning: Failed to connect to Memory Service: %v\n", err)
		}
	}

	// ★ 尝试连接 Harness Service（规则引擎）
	harnessAddr := cfg.Services.Harness
	var harnessClient harnesspb.HarnessServiceClient
	var harnessConn *grpc.ClientConn

	if harnessAddr != "" {
		conn, err := grpc.Dial(harnessAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err == nil {
			harnessClient = harnesspb.NewHarnessServiceClient(conn)
			harnessConn = conn
			fmt.Printf("Connected to Harness Service at %s\n", harnessAddr)
		} else {
			fmt.Printf("Warning: Failed to connect to Harness Service: %v\n", err)
		}
	}

	return &ChatService{
		llmClient:     llm.NewMetricsClient(llmClient, chatLLMMetricsCallback(harnessClient), "chat"),
		qdrant:        qdrant,
		sessionRepo:   sessionRepo,
		mcpClient:     mcpClient,
		agentClient:   agentClient,
		agentConn:     agentConn,
		memoryClient:  memoryClient,
		memoryConn:    memoryConn,
		harnessClient: harnessClient,
		harnessConn:   harnessConn,
		cfg:           cfg,
		maxSteps:      5,
		useMultiAgent: agentClient != nil,
		enableMemory:  memoryClient != nil,
		enableRules:   harnessClient != nil,
	}
}

// Close closes connections
func (s *ChatService) Close() {
	if s.agentConn != nil {
		s.agentConn.Close()
	}
	if s.memoryConn != nil {
		s.memoryConn.Close()
	}
	if s.harnessConn != nil {
		s.harnessConn.Close()
	}
}

// Chat handles a chat request - 使用多 Agent 架构
func (s *ChatService) Chat(ctx context.Context, req *pb.ChatRequest) (*pb.ChatResponse, error) {
	startTime := time.Now()

	// ★ 先检查规则（安全护栏）
	if s.enableRules && s.harnessClient != nil {
		blocked, reason := s.checkRules(ctx, req.Message, req.TenantId)
		if blocked {
			return &pb.ChatResponse{
				Content: "您的请求被安全护栏拦截：" + reason,
			}, nil
		}
	}

	// ★ Prompt 模板渲染：如果请求指定了模板key，通过 harness 服务渲染
	if req.PromptTemplateKey != "" && s.harnessClient != nil {
		renderResp, err := s.harnessClient.RenderPrompt(ctx, &harnesspb.RenderPromptRequest{
			PromptKey: req.PromptTemplateKey,
			Variables: req.PromptVariables,
		})
		if err == nil && renderResp.Content != "" {
			req.SystemPrompt = renderResp.Content
		}
	}

		// ★ A/B 实验：Prompt 对比测试
	var abTestID string
	var abIsVariant bool
	var abPromptOverride string

	if s.harnessClient != nil {
		abTestID, abIsVariant, abPromptOverride = s.getABTestPrompt(ctx, req.SessionId, req.TenantId)
		if abPromptOverride != "" {
			// 追加到原有 system prompt，不覆盖
			if req.SystemPrompt != "" {
				req.SystemPrompt = req.SystemPrompt + "\n\n" + abPromptOverride
			} else {
				req.SystemPrompt = abPromptOverride
			}
		}
	}

	// 如果有 Agent Service，使用多 Agent 架构
	var resp *pb.ChatResponse
	var execErr error
	if s.useMultiAgent && s.agentClient != nil {
		resp, execErr = s.chatWithMultiAgent(ctx, req)
	} else {
		// 否则使用原有的单 Agent 逻辑
		resp, execErr = s.chatWithSingleAgent(ctx, req)
	}

	// ★ 记录 A/B 实验结果
	if abTestID != "" && resp != nil {
		latency := float64(time.Since(startTime).Milliseconds())
		score := s.calculateResponseScore(resp)
		fmt.Printf("[AB] Recording: testId=%s isVariant=%v score=%.2f latency=%dms success=%v\n", abTestID, abIsVariant, score, int(latency), execErr == nil)
	}

	if execErr != nil {
		return nil, execErr
	}

	// ★ Record catalog usage (use independent context)
	if s.harnessClient != nil && resp != nil {
		catalogCtx, catalogCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer catalogCancel()
		if s.useMultiAgent && s.agentClient != nil {
			s.recordCatalogUsage(catalogCtx, "main-agent")  // multi-agent mode
		} else {
			s.recordCatalogUsage(catalogCtx, "chat-agent")  // single-agent mode
		}
	}

	return resp, nil
}

// checkRules 检查规则，返回是否被拦截和拦截原因
func (s *ChatService) checkRules(ctx context.Context, content, tenantId string) (bool, string) {
	if s.harnessClient == nil {
		return false, ""
	}

	// 调用 Harness Service 检查护栏
	resp, err := s.harnessClient.CheckGuardrail(ctx, &harnesspb.GuardrailCheckRequest{
		Content:  content,
		Type:     "input", // 必须指定类型，否则不会检测 prompt 注入
		TenantId: tenantId,
	})
	if err != nil {
		fmt.Printf("Warning: failed to check guardrail: %v\n", err)
		return false, "" // 检查失败时不拦截
	}

	if !resp.Passed && len(resp.Violations) > 0 {
		return true, strings.Join(resp.Violations, "; ")
	}
	return false, ""
}

// chatWithMultiAgent 使用多 Agent 架构处理对话
func (s *ChatService) chatWithMultiAgent(ctx context.Context, req *pb.ChatRequest) (*pb.ChatResponse, error) {
	// ★ 先创建或获取 session（与 chatWithSingleAgent 保持一致）
	session, err := s.getOrCreateSession(ctx, req.SessionId, req.TenantId, req.UserId)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	// ★ 检索长期记忆
	memoryCtx, memoryCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer memoryCancel()
	var memoryContext string
	if s.enableMemory && s.memoryClient != nil {
		memories, err := s.recallMemories(memoryCtx, req.Message, session.ID, req.TenantId)
		if err == nil && len(memories) > 0 {
			memoryContext = s.formatMemories(memories)
		}
	}

	// 构建系统提示
	systemPrompt := req.SystemPrompt
	if memoryContext != "" {
		systemPrompt = fmt.Sprintf("%s\n\n%s", systemPrompt, memoryContext)
	}

	// 调用 Agent Service（使用 session.ID）
	agentReq := &agentpb.ExecuteRequest{
		SessionId:   session.ID, // 使用创建好的 session ID
		TenantId:    req.TenantId,
		UserId:      req.UserId,
		Message:     req.Message,
		EntryAgent:  "main-agent", // 默认入口 Agent
	}

	// 转换 context vars
	if systemPrompt != "" {
		agentReq.ContextVars = map[string]string{
			"system_prompt": systemPrompt,
		}
	}

	// 使用更长的超时调用 Agent Service（Browser Agent 需要时间）
	agentCtx, agentCancel := context.WithTimeout(context.Background(), 900*time.Second) // 15 分钟
	defer agentCancel()
	agentStart := time.Now()
	resp, err := s.agentClient.Execute(agentCtx, agentReq)
	agentLatency := time.Since(agentStart).Milliseconds()
	// Use independent context for harness metrics to avoid parent ctx timeout
	metricsCtx, metricsCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer metricsCancel()
	if err != nil {
		if s.harnessClient != nil {
			s.harnessClient.RecordLLMMetrics(metricsCtx, &harnesspb.RecordLLMMetricsRequest{
				AgentId:   "main-agent",
				Model:     s.cfg.LLM.Model,
				LatencyMs: agentLatency,
				Success:   false,
			})
		}
		fmt.Printf("Multi-agent execution failed: %v\n", err)
		return nil, fmt.Errorf("multi-agent execution failed: %w", err)
	}

	if s.harnessClient != nil {
		_, merr := s.harnessClient.RecordLLMMetrics(metricsCtx, &harnesspb.RecordLLMMetricsRequest{
			AgentId:      "main-agent",
			Model:        s.cfg.LLM.Model,
			LatencyMs:    agentLatency,
			InputTokens:  int64(resp.TotalTokens * 6 / 10),
			OutputTokens: int64(resp.TotalTokens * 4 / 10),
			Cost:         resp.TotalCost,
			Success:      true,
		})
		if merr != nil {
			fmt.Printf("[LLM Metrics] Failed to send to harness: %v\n", merr)
		} else {
			fmt.Printf("[LLM Metrics] Sent to harness: agent=main-agent latency=%dms tokens=%d cost=%.6f\n", agentLatency, resp.TotalTokens, resp.TotalCost)
		}
	}

	// ★ 保存用户消息到 session
	userMsg := &repository.Message{
		Role:      "user",
		Content:   req.Message,
		CreatedAt: time.Now(),
	}
	session.Messages = append(session.Messages, userMsg)

	// ★ 如果是第一条消息，根据内容生成标题
	if len(session.Messages) == 1 {
		session.Title = s.generateSessionTitle(req.Message)
	}

	// ★ 将 AgentExecutionRecord 转为 repository.AgentState 和 repository.ToolCall 并持久化
	var agentStates []repository.AgentState
	var toolCalls []repository.ToolCall

	for i, h := range resp.AgentHistory {
		var args map[string]interface{}
		if h.Arguments != "" {
			_ = json.Unmarshal([]byte(h.Arguments), &args)
		}
		if args == nil {
			args = map[string]interface{}{}
		}

		agentStates = append(agentStates, repository.AgentState{
			Thought:   h.Thought,
			Action:    h.Action,
			Arguments: args,
			Result:    h.Result,
			Step:      i + 1,
		})

		if h.Action != "" && h.Action != "handoff" {
			toolCalls = append(toolCalls, repository.ToolCall{
				ID:        fmt.Sprintf("tc-%s-%d", h.AgentId, i+1),
				Name:      h.Action,
				Arguments: args,
				Result:    h.Result,
				Status:    "completed",
				CreatedAt: time.Now(),
			})
		}
	}

	// ★ 保存助手消息到 session（包含 agent_trace 和 tool_calls）
	assistantMsg := &repository.Message{
		Role:       "assistant",
		Content:    resp.Response,
		AgentTrace: agentStates,
		ToolCalls:  toolCalls,
		CreatedAt:  time.Now(),
	}
	session.Messages = append(session.Messages, assistantMsg)

	// ★ 保存 session（使用独立 context 避免父 ctx 已过期导致保存失败）
	saveCtx, saveCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer saveCancel()
	if err := s.sessionRepo.Save(saveCtx, session); err != nil {
		fmt.Printf("Warning: Failed to save session: %v\n", err)
	}

	// ★ 保存重要信息到长期记忆
	if s.enableMemory && resp.Status == "completed" {
		go s.saveConversationMemory(context.Background(), session.ID, req.TenantId, req.Message, resp.Response)
	}

	// ★ 保存情景记忆（记录本次对话作为一段经历）
	if s.enableMemory && s.memoryClient != nil {
		go s.saveEpisodeMemory(context.Background(), session.ID, req.TenantId, req.Message, resp.Response, resp.Status)
	}

	// 转换响应（使用 session.ID，而不是 resp.SessionId）
	result := &pb.ChatResponse{
		SessionId:   session.ID, // ★ 使用本地创建的 session ID
		Content:     resp.Response,
		TotalTokens: resp.TotalTokens,
		Cost:        resp.TotalCost,
	}

	// 转换 agent history（复用已构建的 agentStates）
	for _, state := range agentStates {
		argsJSON, _ := json.Marshal(state.Arguments)
		result.AgentStates = append(result.AgentStates, &pb.AgentState{
			Thought:   state.Thought,
			Action:    state.Action,
			Arguments: string(argsJSON),
			Result:    state.Result,
			Step:      int32(state.Step),
		})
	}

	return result, nil
}

// chatWithSingleAgent 使用原有单 Agent 逻辑
func (s *ChatService) chatWithSingleAgent(ctx context.Context, req *pb.ChatRequest) (*pb.ChatResponse, error) {
	// Get or create session
	session, err := s.getOrCreateSession(ctx, req.SessionId, req.TenantId, req.UserId)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	// Get available tools
	tools, err := s.getAvailableTools(ctx)
	if err != nil {
		tools = []llm.ToolDefinition{}
	}

	// ★ Build system prompt with memory
	systemPrompt := s.buildAgentSystemPrompt(req.SystemPrompt, tools)

	// ★ 检索长期记忆
	singleMemoryCtx, singleMemoryCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer singleMemoryCancel()
	if s.enableMemory && s.memoryClient != nil {
		memories, err := s.recallMemories(singleMemoryCtx, req.Message, session.ID, req.TenantId)
		if err == nil && len(memories) > 0 {
			systemPrompt = fmt.Sprintf("%s\n\n%s", systemPrompt, s.formatMemories(memories))
		}
	}

	// Build messages
	messages := s.buildMessages(session, req.Message, systemPrompt)

	// Save user message
	userMsg := &repository.Message{
		Role:      "user",
		Content:   req.Message,
		CreatedAt: time.Now(),
	}
	session.Messages = append(session.Messages, userMsg)

	// 如果是第一条消息，根据内容生成标题
	if len(session.Messages) == 1 {
		session.Title = s.generateSessionTitle(req.Message)
	}

	// Execute Agent loop
	agentStates, finalContent, toolCalls, totalTokens, cost, err := s.executeAgentLoop(ctx, messages, tools, req.Model)
	if err != nil {
		return nil, fmt.Errorf("agent loop: %w", err)
	}

	// Save agent trace and assistant message
	assistantMsg := &repository.Message{
		Role:       "assistant",
		Content:    finalContent,
		AgentTrace: agentStates,
		ToolCalls:  toolCalls,
		CreatedAt:  time.Now(),
	}
	session.Messages = append(session.Messages, assistantMsg)

	// Save session with independent context to avoid parent ctx timeout
	saveCtx2, saveCancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer saveCancel2()
	if err := s.sessionRepo.Save(saveCtx2, session); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}

	// ★ 异步保存重要信息到长期记忆
	if s.enableMemory {
		go s.saveConversationMemory(context.Background(), session.ID, req.TenantId, req.Message, finalContent)
	}

	// ★ 保存情景记忆（单 Agent 模式）
	if s.enableMemory && s.memoryClient != nil {
		go s.saveEpisodeMemory(context.Background(), session.ID, req.TenantId, req.Message, finalContent, "completed")
	}

	// Build response
	resp := &pb.ChatResponse{
		SessionId:   session.ID,
		Content:     finalContent,
		TotalTokens: int32(totalTokens),
		Cost:        cost,
	}

	// Convert agent states
	for _, state := range agentStates {
		argsJSON, _ := json.Marshal(state.Arguments)
		resp.AgentStates = append(resp.AgentStates, &pb.AgentState{
			Thought:   state.Thought,
			Action:    state.Action,
			Arguments: string(argsJSON),
			Result:    state.Result,
			Step:      int32(state.Step),
		})
	}

	// Convert tool calls for response
	for _, tc := range toolCalls {
		argsJSON, _ := json.Marshal(tc.Arguments)
		resp.ToolCalls = append(resp.ToolCalls, &pb.ToolCall{
			Id:        tc.ID,
			Name:      tc.Name,
			Arguments: string(argsJSON),
			Status:    tc.Status,
			Result:    tc.Result,
		})
	}

	return resp, nil
}

// ============================================================
// 记忆相关方法
// ============================================================

// recallMemories 检索相关长期记忆（用户级别，跨 session）
func (s *ChatService) recallMemories(ctx context.Context, query, sessionID, tenantID string) ([]*memorypb.MemoryEntry, error) {
	if s.memoryClient == nil {
		return nil, fmt.Errorf("memory service not available")
	}

	// 不传入 session_id，只按 tenant_id 查询，实现用户级记忆
	resp, err := s.memoryClient.Recall(ctx, &memorypb.RecallMemoryRequest{
		Query:     query,
		SessionId: "", // 空：跨 session 查询用户级记忆
		TenantId:  tenantID,
		TopK:      5, // 检索最相关的 5 条记忆
	})
	if err != nil {
		fmt.Printf("Failed to recall memories: %v\n", err)
		return nil, err
	}

	return resp.Memories, nil
}

// formatMemories 格式化记忆为文本
func (s *ChatService) formatMemories(memories []*memorypb.MemoryEntry) string {
	if len(memories) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("【用户相关记忆】\n")
	for _, m := range memories {
		// 格式化时间
		timeStr := ""
		if m.CreatedAt > 0 {
			t := time.Unix(m.CreatedAt, 0)
			timeStr = t.Format("2006-01-02 15:04")
		}
		if timeStr != "" {
			sb.WriteString(fmt.Sprintf("- %s (记录于 %s)\n", m.Content, timeStr))
		} else {
			sb.WriteString(fmt.Sprintf("- %s\n", m.Content))
		}
	}
	return sb.String()
}

// saveConversationMemory 保存对话中的重要信息到长期记忆
func (s *ChatService) saveConversationMemory(ctx context.Context, sessionID, tenantID, userMessage, assistantMessage string) {
	if s.memoryClient == nil {
		return
	}

	// 提取用户消息中的重要信息
	if importantFacts := s.extractImportantInfo(userMessage); len(importantFacts) > 0 {
		for _, fact := range importantFacts {
			_, err := s.memoryClient.Save(ctx, &memorypb.SaveMemoryRequest{
				SessionId:  sessionID,
				TenantId:   tenantID,
				Content:    fact,
				Type:       memorypb.MemoryType_MEMORY_TYPE_FACT,
				Importance: 0.7,
			})
			if err != nil {
				fmt.Printf("Failed to save memory: %v\n", err)
			}
		}
	}

	// 如果助手回复包含重要信息，也保存
	if s.isImportantResponse(assistantMessage) {
		// 简化：保存摘要而非全文
		summary := s.summarizeContent(assistantMessage)
		if summary != "" {
			_, err := s.memoryClient.Save(ctx, &memorypb.SaveMemoryRequest{
				SessionId:  sessionID,
				TenantId:   tenantID,
				Content:    summary,
				Type:       memorypb.MemoryType_MEMORY_TYPE_SUMMARY,
				Importance: 0.5,
			})
			if err != nil {
				fmt.Printf("Failed to save summary: %v\n", err)
			}
		}
	}
}

	// saveEpisodeMemory saves an episodic memory record for the conversation
	func (s *ChatService) saveEpisodeMemory(ctx context.Context, sessionID, tenantID, userMessage, assistantMessage, status string) {
		if s.memoryClient == nil {
			return
		}

		// Store the episode as an IMPORTANT memory with structured content
		episodeContent := fmt.Sprintf("用户: %s | 助手: %s", userMessage, assistantMessage)
		if len(episodeContent) > 500 {
			episodeContent = episodeContent[:500] + "..."
		}

		_, err := s.memoryClient.Save(ctx, &memorypb.SaveMemoryRequest{
			SessionId:  sessionID,
			TenantId:   tenantID,
			Content:    episodeContent,
			Type:       memorypb.MemoryType_MEMORY_TYPE_IMPORTANT,
			Importance: 0.6,
		})
		if err != nil {
			log.Printf("Failed to save episode memory: %v", err)
		}
	}

	// extractImportantInfo 提取用户消息中的重要信息
func (s *ChatService) extractImportantInfo(message string) []string {
	var facts []string

	// 关键词模式匹配
	patterns := []struct {
		keyword string
		extract func(string, string) string
	}{
		{"我叫", extractName},
		{"我的名字是", extractName},
		{"我是", extractName},
		{"我喜欢", extractPreference},
		{"我讨厌", extractPreference},
		{"记住", extractInstruction},
		{"别忘了", extractInstruction},
		{"重要", extractImportant},
	}

	for _, p := range patterns {
		if strings.Contains(message, p.keyword) {
			if fact := p.extract(message, p.keyword); fact != "" {
				facts = append(facts, fact)
			}
		}
	}

	return facts
}

// 辅助提取函数
func extractName(message, keyword string) string {
	idx := strings.Index(message, keyword)
	if idx == -1 {
		return ""
	}
	after := strings.TrimSpace(message[idx+len(keyword):])
	// 提取到句号或逗号
	end := strings.IndexAny(after, "。，,！!")
	if end > 0 {
		after = after[:end]
	}
	if len(after) > 0 && len(after) < 50 {
		return fmt.Sprintf("用户的名字是%s", after)
	}
	return ""
}

func extractPreference(message, keyword string) string {
	idx := strings.Index(message, keyword)
	if idx == -1 {
		return ""
	}
	after := strings.TrimSpace(message[idx:])
	if len(after) < 100 {
		return after
	}
	return after[:100]
}

func extractInstruction(message, keyword string) string {
	idx := strings.Index(message, keyword)
	if idx == -1 {
		return ""
	}
	after := strings.TrimSpace(message[idx:])
	if len(after) < 200 {
		return after
	}
	return after[:200]
}

func extractImportant(message, keyword string) string {
	idx := strings.Index(message, keyword)
	if idx == -1 {
		return ""
	}
	// 提取包含"重要"的整句话
	start := strings.LastIndex(message[:idx], "。")
	if start == -1 {
		start = 0
	} else {
		start++
	}
	end := strings.Index(message[idx:], "。")
	if end == -1 {
		end = len(message)
	} else {
		end = idx + end + 1
	}
	return message[start:end]
}

// isImportantResponse 判断回复是否包含重要信息
func (s *ChatService) isImportantResponse(message string) bool {
	importantKeywords := []string{
		"建议", "推荐", "注意", "重要", "总结", "结论",
	}
	for _, kw := range importantKeywords {
		if strings.Contains(message, kw) {
			return true
		}
	}
	return false
}

// summarizeContent 简单摘要（截取前 200 字符）
func (s *ChatService) summarizeContent(content string) string {
	content = strings.TrimSpace(content)
	if len(content) <= 200 {
		return content
	}
	return content[:200] + "..."
}

// ============================================================
// 原有方法
// ============================================================

// executeAgentLoop runs the agent thought-action-observation loop
func (s *ChatService) executeAgentLoop(ctx context.Context, messages []llm.Message, tools []llm.ToolDefinition, model string) (
	[]repository.AgentState, string, []repository.ToolCall, int, float64, error) {

	agentStates := []repository.AgentState{}
	toolCalls := []repository.ToolCall{}
	totalTokens := 0
	totalCost := 0.0
	currentMessages := messages

	for step := 0; step < s.maxSteps; step++ {
		// Call LLM
		llmReq := &llm.ChatRequest{
			Messages:    currentMessages,
			Model:       model,
			MaxTokens:   s.cfg.LLM.MaxTokens,
			Temperature: 0.7,
			Tools:       tools,
		}

		if llmReq.Model == "" {
			llmReq.Model = s.cfg.LLM.Model
		}

		llmResp, err := s.llmClient.Chat(ctx, llmReq)
		if err != nil {
			return agentStates, "", toolCalls, totalTokens, totalCost, fmt.Errorf("llm call: %w", err)
		}

		totalTokens += llmResp.TotalTokens
		totalCost += llmResp.Cost

		// Record cost usage to harness
		s.recordCostUsage(ctx, "", model, "", llmResp.PromptTokens, llmResp.TotalTokens)

		// Check if LLM wants to use tools
		if len(llmResp.ToolCalls) == 0 {
			// No tools - LLM is done, return the content
			return agentStates, llmResp.Content, toolCalls, totalTokens, totalCost, nil
		}

		// Process each tool call
		for _, tc := range llmResp.ToolCalls {
			// Parse arguments
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				args = map[string]interface{}{}
			}

			// Record agent state (Thought + Action)
			state := repository.AgentState{
				Thought:   s.extractThought(llmResp.Content),
				Action:    tc.Function.Name,
				Arguments: args,
				Result:    "",
				Step:      step + 1,
			}

			// Execute the tool
			result, status := s.executeTool(ctx, tc.Function.Name, args)
			state.Result = result

			agentStates = append(agentStates, state)

			// Record tool call
			toolCall := repository.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: args,
				Result:    result,
				Status:    status,
				CreatedAt: time.Now(),
			}
			toolCalls = append(toolCalls, toolCall)

			// Add assistant message content
			currentMessages = append(currentMessages, llm.Message{
				Role:    "assistant",
				Content: llmResp.Content,
			})

			// Add tool result as user message (observation)
			currentMessages = append(currentMessages, llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("Tool %s result: %s", tc.Function.Name, result),
			})
		}
	}

	// Max steps reached - force final answer
	finalReq := &llm.ChatRequest{
		Messages:    currentMessages,
		Model:       model,
		MaxTokens:   s.cfg.LLM.MaxTokens,
		Temperature: 0.7,
		SystemPrompt: "You have reached the maximum number of tool calls. Please provide your final answer now based on the information gathered.",
	}

	if finalReq.Model == "" {
		finalReq.Model = s.cfg.LLM.Model
	}

	finalResp, err := s.llmClient.Chat(ctx, finalReq)
	if err != nil {
		return agentStates, "", toolCalls, totalTokens, totalCost, fmt.Errorf("final llm call: %w", err)
	}

	totalTokens += finalResp.TotalTokens
	totalCost += finalResp.Cost

	// Record cost usage to harness
	s.recordCostUsage(ctx, "", model, "", finalResp.PromptTokens, finalResp.TotalTokens)

	return agentStates, finalResp.Content, toolCalls, totalTokens, totalCost, nil
}

// executeTool executes a tool via MCP service
func (s *ChatService) executeTool(ctx context.Context, name string, args map[string]interface{}) (string, string) {
	if s.mcpClient == nil {
		return "MCP service not available", "error"
	}

	// Call MCP service
	argsJSON, _ := json.Marshal(args)
	req := &mcppb.CallToolRequest{
		Name:      name,
		Arguments: string(argsJSON),
	}

	resp, err := s.mcpClient.CallTool(ctx, req)
	if err != nil {
		return fmt.Sprintf("Tool execution error: %v", err), "error"
	}

	if resp.IsError {
		return resp.Content, "error"
	}

	return resp.Content, "completed"
}

// getAvailableTools fetches tools from MCP service
func (s *ChatService) getAvailableTools(ctx context.Context) ([]llm.ToolDefinition, error) {
	if s.mcpClient == nil {
		return nil, fmt.Errorf("MCP client not initialized")
	}

	resp, err := s.mcpClient.ListTools(ctx, &mcppb.ListToolsRequest{})
	if err != nil {
		return nil, err
	}

	tools := []llm.ToolDefinition{}
	for _, t := range resp.Tools {
		// Parse input schema
		var params map[string]interface{}
		if err := json.Unmarshal([]byte(t.InputSchema), &params); err != nil {
			params = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
		}

		tools = append(tools, llm.ToolDefinition{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		})
	}

	return tools, nil
}

// buildAgentSystemPrompt builds system prompt with agent instructions
func (s *ChatService) buildAgentSystemPrompt(customPrompt string, tools []llm.ToolDefinition) string {
	basePrompt := `You are an intelligent AI agent with access to tools.

When you need to use a tool:
1. First think about what information you need
2. Choose the appropriate tool
3. Provide the tool call with proper arguments
4. Wait for the tool result
5. Use the result to continue reasoning or provide the final answer

Available tools:
`

	// Add tool descriptions
	for _, t := range tools {
		basePrompt += fmt.Sprintf("- %s: %s\n", t.Function.Name, t.Function.Description)
	}

	if customPrompt != "" {
		return customPrompt + "\n\n" + basePrompt
	}

	return basePrompt
}

// extractThought extracts thinking content from response
func (s *ChatService) extractThought(content string) string {
	// Look for thinking markers
	if strings.Contains(content, "<thought>") {
		start := strings.Index(content, "<thought>")
		end := strings.Index(content, "</thought>")
		if end > start {
			return strings.TrimSpace(content[start+9 : end])
		}
	}

	// Use first 200 chars as thought if no markers
	if len(content) > 200 {
		return content[:200] + "..."
	}
	return content
}

// ChatStream handles a streaming chat request with real-time token streaming
func (s *ChatService) ChatStream(req *pb.ChatRequest, stream pb.ChatService_ChatStreamServer) error {
	ctx := stream.Context()

	// ★ 先检查规则（安全护栏）— 与 Chat 保持一致
	if s.enableRules && s.harnessClient != nil {
		blocked, reason := s.checkRules(ctx, req.Message, req.TenantId)
		if blocked {
			stream.Send(&pb.ChatStreamChunk{
				Type:    "error",
				Content: "您的请求被安全护栏拦截：" + reason,
			})
			return nil
		}
	}

	// ★ Prompt 模板渲染
	if req.PromptTemplateKey != "" && s.harnessClient != nil {
		renderResp, err := s.harnessClient.RenderPrompt(ctx, &harnesspb.RenderPromptRequest{
			PromptKey: req.PromptTemplateKey,
			Variables: req.PromptVariables,
		})
		if err == nil && renderResp.Content != "" {
			req.SystemPrompt = renderResp.Content
		}
	}

	// ★ A/B 实验
	if s.harnessClient != nil {
		_, _, abPromptOverride := s.getABTestPrompt(ctx, req.SessionId, req.TenantId)
		if abPromptOverride != "" {
			if req.SystemPrompt != "" {
				req.SystemPrompt = req.SystemPrompt + "\n\n" + abPromptOverride
			} else {
				req.SystemPrompt = abPromptOverride
			}
		}
	}

	// Route to the appropriate streaming path
	if s.useMultiAgent && s.agentClient != nil {
		return s.chatStreamWithMultiAgent(ctx, req, stream)
	}
	return s.chatStreamWithSingleAgent(ctx, req, stream)
}

// chatStreamWithMultiAgent streams via Agent Service's ExecuteStream
func (s *ChatService) chatStreamWithMultiAgent(ctx context.Context, req *pb.ChatRequest, stream pb.ChatService_ChatStreamServer) error {
	// Create or get session
	session, err := s.getOrCreateSession(ctx, req.SessionId, req.TenantId, req.UserId)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	// Recall memories for context
	var memoryContext string
	if s.enableMemory && s.memoryClient != nil {
		memoryCtx, memoryCancel := context.WithTimeout(context.Background(), 10*time.Second)
		memories, memErr := s.recallMemories(memoryCtx, req.Message, session.ID, req.TenantId)
		memoryCancel()
		if memErr == nil && len(memories) > 0 {
			memoryContext = s.formatMemories(memories)
		}
	}

	// Build system prompt with memory
	systemPrompt := req.SystemPrompt
	if memoryContext != "" {
		systemPrompt = fmt.Sprintf("%s\n\n%s", systemPrompt, memoryContext)
	}

	// Build agent request
	agentReq := &agentpb.ExecuteStreamRequest{
		SessionId:  session.ID,
		TenantId:   req.TenantId,
		UserId:     req.UserId,
		Message:    req.Message,
		EntryAgent: "main-agent",
	}
	if systemPrompt != "" {
		agentReq.ContextVars = map[string]string{"system_prompt": systemPrompt}
	}

	// Call Agent Service streaming endpoint
	agentCtx, agentCancel := context.WithTimeout(context.Background(), 900*time.Second)
	defer agentCancel()

	agentStream, err := s.agentClient.ExecuteStream(agentCtx, agentReq)
	if err != nil {
		return fmt.Errorf("agent stream failed: %w", err)
	}

	// Collect streaming events and forward to chat stream
	var fullContent string
	var agentStates []*pb.AgentState
	var toolCalls []*pb.ToolCall

	for {
		chunk, err := agentStream.Recv()
		if err != nil {
			// Stream ended (io.EOF) or error
			break
		}

		// Forward the event to the chat stream
		chatChunk := &pb.ChatStreamChunk{
			Type:    chunk.Type,
			Content: chunk.Content,
		}

		// Track content for session persistence
		switch chunk.Type {
		case "token":
			fullContent += chunk.Content
		case "think":
			chatChunk.Content = fmt.Sprintf("[%s] 正在思考...", chunk.CurrentAgent)
		case "tool_start":
			if chunk.ToolCall != nil {
				chatChunk.Content = fmt.Sprintf("🔧 调用工具: %s", chunk.ToolCall.Name)
				// Convert agent.ToolCall to chat.ToolCall (different proto packages)
				toolCalls = append(toolCalls, &pb.ToolCall{
					Id:        chunk.ToolCall.Id,
					Name:      chunk.ToolCall.Name,
					Arguments: chunk.ToolCall.Arguments,
				})
			}
		case "tool_result":
			chatChunk.Content = fmt.Sprintf("✅ 工具结果: %s", truncateString(chunk.Content, 100))
			if chunk.Step != nil {
				agentStates = append(agentStates, &pb.AgentState{
					Action:  chunk.Step.Action,
					Result:  chunk.Step.Result,
					Step:    int32(len(agentStates) + 1),
				})
			}
		case "handoff":
			chatChunk.Content = fmt.Sprintf("🔄 转交到 %s", chunk.CurrentAgent)
		case "final":
			fullContent = chunk.Content // Use the final complete content
			chatChunk.Content = chunk.Content
		case "error":
			chatChunk.Content = "❌ " + chunk.Content
		}

		// Set the state field for agent_step events
		if chunk.Step != nil {
			chatChunk.State = &pb.AgentState{
				Thought:   chunk.Step.Thought,
				Action:    chunk.Step.Action,
				Arguments: chunk.Step.Arguments,
				Result:    chunk.Step.Result,
				Step:      int32(len(agentStates) + 1), // derive step number from accumulated states
			}
		}

		stream.Send(chatChunk)
	}

	// Save session with the collected data
	s.saveStreamedSession(ctx, session, req, fullContent, agentStates, toolCalls)

	return nil
}

// chatStreamWithSingleAgent streams using local LLM streaming
func (s *ChatService) chatStreamWithSingleAgent(ctx context.Context, req *pb.ChatRequest, stream pb.ChatService_ChatStreamServer) error {
	// Get or create session
	session, err := s.getOrCreateSession(ctx, req.SessionId, req.TenantId, req.UserId)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	// Get available tools
	tools, err := s.getAvailableTools(ctx)
	if err != nil {
		return fmt.Errorf("get tools: %w", err)
	}

	// Build messages
	messages := s.buildMessages(session, req.Message, req.SystemPrompt)

	// Use LLM streaming
	stream.Send(&pb.ChatStreamChunk{
		Type:    "think",
		Content: "正在思考...",
	})

	llmCh, err := s.llmClient.ChatStream(ctx, &llm.ChatRequest{
		Messages:    messages,
		Tools:       tools,
		Model:       s.cfg.LLM.Model,
		MaxTokens:   s.cfg.LLM.MaxTokens,
		Temperature: 0.7,
	})
	if err != nil {
		stream.Send(&pb.ChatStreamChunk{Type: "error", Content: err.Error()})
		return err
	}

	var fullContent string
	for chunk := range llmCh {
		if chunk.Error != nil {
			stream.Send(&pb.ChatStreamChunk{Type: "error", Content: chunk.Error.Error()})
			return chunk.Error
		}
		if chunk.Done {
			break
		}
		if chunk.Content != "" {
			fullContent += chunk.Content
			stream.Send(&pb.ChatStreamChunk{
				Type:    "token",
				Content: chunk.Content,
			})
		}
	}

	// Send final chunk
	stream.Send(&pb.ChatStreamChunk{
		Type:    "final",
		Content: fullContent,
	})

	// Save session
	userMsg := &repository.Message{Role: "user", Content: req.Message, CreatedAt: time.Now()}
	assistantMsg := &repository.Message{Role: "assistant", Content: fullContent, CreatedAt: time.Now()}
	session.Messages = append(session.Messages, userMsg, assistantMsg)
	if len(session.Messages) == 2 {
		session.Title = s.generateSessionTitle(req.Message)
	}
	saveCtx, saveCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer saveCancel()
	s.sessionRepo.Save(saveCtx, session)

	return nil
}

// saveStreamedSession persists the session data after streaming completes
func (s *ChatService) saveStreamedSession(ctx context.Context, session *repository.Session, req *pb.ChatRequest, content string, agentStates []*pb.AgentState, toolCalls []*pb.ToolCall) {
	// Save user message
	userMsg := &repository.Message{Role: "user", Content: req.Message, CreatedAt: time.Now()}
	session.Messages = append(session.Messages, userMsg)

	// Generate title if first message
	if len(session.Messages) == 1 {
		session.Title = s.generateSessionTitle(req.Message)
	}

	// Convert proto agent states to repository format
	var repoAgentStates []repository.AgentState
	for _, state := range agentStates {
		var args map[string]interface{}
		if state.Arguments != "" {
			_ = json.Unmarshal([]byte(state.Arguments), &args)
		}
		if args == nil {
			args = map[string]interface{}{}
		}
		repoAgentStates = append(repoAgentStates, repository.AgentState{
			Thought:   state.Thought,
			Action:    state.Action,
			Arguments: args,
			Result:    state.Result,
			Step:      int(state.Step),
		})
	}

	// Convert proto tool calls to repository format
	var repoToolCalls []repository.ToolCall
	for i, tc := range toolCalls {
		var args map[string]interface{}
		if tc.Arguments != "" {
			_ = json.Unmarshal([]byte(tc.Arguments), &args)
		}
		if args == nil {
			args = map[string]interface{}{}
		}
		repoToolCalls = append(repoToolCalls, repository.ToolCall{
			ID:        fmt.Sprintf("tc-%d", i+1),
			Name:      tc.Name,
			Arguments: args,
			Status:    "completed",
			CreatedAt: time.Now(),
		})
	}

	// Save assistant message
	assistantMsg := &repository.Message{
		Role:       "assistant",
		Content:    content,
		AgentTrace: repoAgentStates,
		ToolCalls:  repoToolCalls,
		CreatedAt:  time.Now(),
	}
	session.Messages = append(session.Messages, assistantMsg)

	// Persist session
	saveCtx, saveCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer saveCancel()
	if err := s.sessionRepo.Save(saveCtx, session); err != nil {
		fmt.Printf("Warning: Failed to save streamed session: %v\n", err)
	}

	// Save to long-term memory
	if s.enableMemory && content != "" {
		go s.saveConversationMemory(context.Background(), session.ID, req.TenantId, req.Message, content)
	}
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// MultiAgentChat handles a multi-agent chat request
func (s *ChatService) MultiAgentChat(ctx context.Context, req *pb.MultiAgentRequest) (*pb.MultiAgentResponse, error) {
	// 直接调用 Agent Service
	if s.agentClient != nil {
		agentReq := &agentpb.ExecuteRequest{
			SessionId:   req.SessionId,
			TenantId:    req.TenantId,
			UserId:      req.UserId,
			Message:     req.Message,
			EntryAgent:  req.MasterAgent,
		}

		resp, err := s.agentClient.Execute(ctx, agentReq)
		if err != nil {
			return nil, err
		}

		// 转换响应
		result := &pb.MultiAgentResponse{
			SessionId:   resp.SessionId,
			FinalAnswer: resp.Response,
			TotalTokens: resp.TotalTokens,
			Cost:        resp.TotalCost,
		}

		for _, h := range resp.AgentHistory {
			result.Steps = append(result.Steps, &pb.AgentStep{
				AgentId: h.AgentId,
				Action:  h.Action,
				Result:  h.Result,
			})
		}

		return result, nil
	}

	// 降级到简单模式
	chatReq := &pb.ChatRequest{
		SessionId:    req.SessionId,
		Message:      req.Message,
		TenantId:     req.TenantId,
		UserId:       req.UserId,
		SystemPrompt: "You are coordinating multiple agents to solve complex problems. Break down tasks and delegate appropriately.",
	}

	chatResp, err := s.Chat(ctx, chatReq)
	if err != nil {
		return nil, err
	}

	// Convert agent states to steps
	steps := []*pb.AgentStep{}
	for _, state := range chatResp.AgentStates {
		steps = append(steps, &pb.AgentStep{
			AgentId: "primary-agent",
			Action:  state.Action,
			Result:  state.Result,
		})
	}

	return &pb.MultiAgentResponse{
		SessionId:   chatResp.SessionId,
		FinalAnswer: chatResp.Content,
		Steps:       steps,
		TotalTokens: chatResp.TotalTokens,
		Cost:        chatResp.Cost,
	}, nil
}

// CreateSession creates a new session
func (s *ChatService) CreateSession(ctx context.Context, req *pb.CreateSessionRequest) (*pb.Session, error) {
	session := &repository.Session{
		TenantID:  req.TenantId,
		UserID:    req.UserId,
		Title:     req.Title,
		Messages:  []*repository.Message{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.sessionRepo.Save(ctx, session); err != nil {
		return nil, err
	}

	return s.toProtoSession(session), nil
}

// GetSession gets a session
func (s *ChatService) GetSession(ctx context.Context, req *pb.GetSessionRequest) (*pb.Session, error) {
	session, err := s.sessionRepo.Get(ctx, req.Id, req.TenantId)
	if err != nil {
		return nil, err
	}

	return s.toProtoSession(session), nil
}

// ListSessions lists sessions
func (s *ChatService) ListSessions(ctx context.Context, req *pb.ListSessionsRequest) (*pb.ListSessionsResponse, error) {
	page := int(req.Pagination.GetPage())
	pageSize := int(req.Pagination.GetPageSize())
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}

	sessions, total, err := s.sessionRepo.List(ctx, req.TenantId, req.UserId, page, pageSize)
	if err != nil {
		return nil, err
	}

	resp := &pb.ListSessionsResponse{
		Pagination: &commonpb.PaginationResponse{
			Total:    int32(total),
			Page:     int32(page),
			PageSize: int32(pageSize),
		},
	}

	for _, session := range sessions {
		resp.Sessions = append(resp.Sessions, s.toProtoSession(session))
	}

	return resp, nil
}

// DeleteSession deletes a session
func (s *ChatService) DeleteSession(ctx context.Context, req *pb.DeleteSessionRequest) (*commonpb.Empty, error) {
	if err := s.sessionRepo.Delete(ctx, req.Id, req.TenantId); err != nil {
		return nil, err
	}
	return &commonpb.Empty{}, nil
}

func (s *ChatService) getOrCreateSession(ctx context.Context, sessionID, tenantID, userID string) (*repository.Session, error) {
	if sessionID != "" {
		session, err := s.sessionRepo.Get(ctx, sessionID, tenantID)
		if err == nil {
			return session, nil
		}
	}

	// Create new session
	session := &repository.Session{
		TenantID:  tenantID,
		UserID:    userID,
		Title:     "New Chat", // 默认标题，会在保存第一条消息时更新
		Messages:  []*repository.Message{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.sessionRepo.Save(ctx, session); err != nil {
		return nil, err
	}

	return session, nil
}

// generateSessionTitle 根据用户消息生成会话标题
func (s *ChatService) generateSessionTitle(userMessage string) string {
	// 截取前50个字符作为标题
	title := strings.TrimSpace(userMessage)
	if len(title) > 50 {
		title = title[:50] + "..."
	}
	// 移除换行符
	title = strings.ReplaceAll(title, "\n", " ")
	if title == "" {
		title = "New Chat"
	}
	return title
}

func (s *ChatService) buildMessages(session *repository.Session, userMessage, systemPrompt string) []llm.Message {
	messages := make([]llm.Message, 0)

	// Add system prompt
	if systemPrompt != "" {
		messages = append(messages, llm.Message{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	// Add recent messages (last 10)
	start := 0
	if len(session.Messages) > 10 {
		start = len(session.Messages) - 10
	}

	for i := start; i < len(session.Messages); i++ {
		m := session.Messages[i]
		messages = append(messages, llm.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	// Fallback to working memory for context compression when session history is long
	if len(session.Messages) > 10 && s.enableMemory && s.memoryClient != nil {
		workingCtx, workingCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer workingCancel()
		// Recall compressed summary from long-term memory as context compression
		memories, err := s.recallMemories(workingCtx, userMessage, session.ID, "")
		if err == nil && len(memories) > 0 {
			// Prepend compressed context before recent messages
			var compressedContext string
			for _, m := range memories {
				if m.Importance >= 0.6 {
					compressedContext += m.Content + "\n"
				}
			}
			if compressedContext != "" {
				compressedMsg := llm.Message{
					Role:    "system",
					Content: fmt.Sprintf("【历史对话摘要】\n%s", compressedContext),
				}
				// Insert after system prompt, before recent messages
				if len(messages) > 0 && messages[0].Role == "system" {
					messages = append([]llm.Message{messages[0], compressedMsg}, messages[1:]...)
				} else {
					messages = append([]llm.Message{compressedMsg}, messages...)
				}
			}
		}
	}

	// Add user message
	messages = append(messages, llm.Message{
		Role:    "user",
		Content: userMessage,
	})

	return messages
}

func (s *ChatService) toProtoSession(session *repository.Session) *pb.Session {
	// Use MessageCount from repository if set, otherwise compute from Messages length
	msgCount := session.MessageCount
	if msgCount == 0 && len(session.Messages) > 0 {
		msgCount = len(session.Messages)
	}

	pbSession := &pb.Session{
		Id:           session.ID,
		Title:        session.Title,
		CreatedAt:    session.CreatedAt.Unix(),
		UpdatedAt:    session.UpdatedAt.Unix(),
		MessageCount: int32(msgCount),
	}

	for _, m := range session.Messages {
		pbMsg := &pb.Message{
			Id:        m.ID,
			Role:      m.Role,
			Content:   m.Content,
			Timestamp: m.CreatedAt.Unix(),
		}

		// Add tool calls
		for _, tc := range m.ToolCalls {
			argsJSON, _ := json.Marshal(tc.Arguments)
			pbMsg.ToolCalls = append(pbMsg.ToolCalls, &pb.ToolCall{
				Id:        tc.ID,
				Name:      tc.Name,
				Arguments: string(argsJSON),
				Status:    tc.Status,
				Result:    tc.Result,
			})
		}

		// Add agent trace
		for _, state := range m.AgentTrace {
			argsJSON, _ := json.Marshal(state.Arguments)
			pbMsg.AgentTrace = append(pbMsg.AgentTrace, &pb.AgentState{
				Thought:   state.Thought,
				Action:    state.Action,
				Arguments: string(argsJSON),
				Result:    state.Result,
				Step:      int32(state.Step),
			})
		}

		pbSession.Messages = append(pbSession.Messages, pbMsg)
	}

	return pbSession
}

// ============================================================
// A/B 实验相关方法
// ============================================================

// getABTestPrompt 获取 A/B 实验的 Prompt
// 返回：实验ID、是否实验组、要使用的 Prompt
func (s *ChatService) getABTestPrompt(ctx context.Context, sessionID, tenantID string) (string, bool, string) {
	if s.harnessClient == nil {
		fmt.Printf("[AB] harnessClient is nil\n")
		return "", false, ""
	}

	// 查找正在运行的 Prompt 实验
	resp, err := s.harnessClient.ListABTests(ctx, &harnesspb.ListABTestsRequest{
		TenantId: tenantID,
		Status:   "running",
	})
	if err != nil {
		fmt.Printf("[AB] ListABTests error: %v\n", err)
		return "", false, ""
	}
	fmt.Printf("[AB] ListABTests returned %d tests\n", len(resp.Tests))

	if len(resp.Tests) == 0 {
		return "", false, ""
	}

	// 找到 Prompt 类型的实验
	for _, test := range resp.Tests {
		fmt.Printf("[AB] Test: id=%s, type=%s, control=%s, variant=%s\n", test.Id, test.Type, test.ControlConfig, test.VariantConfig)
		if test.Type == "prompt" || test.ControlConfig != "" {
			// 决定用户走哪组
			variantResp, err := s.harnessClient.ShouldUseVariant(ctx, &harnesspb.ShouldUseVariantRequest{
				ExperimentId: test.Id,
				SessionId:    sessionID,
			})
			if err != nil {
				fmt.Printf("[AB] ShouldUseVariant error: %v\n", err)
				continue
			}

			isVariant := variantResp.IsVariant
			var prompt string
			if isVariant {
				prompt = test.VariantConfig // 实验组 Prompt
			} else {
				prompt = test.ControlConfig // 对照组 Prompt
			}

			fmt.Printf("[AB] Assigned: testId=%s, isVariant=%v, prompt=%s\n", test.Id, isVariant, prompt)
			return test.Id, isVariant, prompt
		}
	}

	return "", false, ""
}

// calculateResponseScore 计算响应质量分数
func (s *ChatService) calculateResponseScore(resp *pb.ChatResponse) float64 {
	if resp == nil {
		return 0
	}

	score := 0.5 // 基础分

	// 1. 有内容 +0.2
	if len(resp.Content) > 50 {
		score += 0.2
	}

	// 2. 有 Agent 思考过程 +0.1
	if len(resp.AgentStates) > 0 {
		score += 0.1
	}

	// 3. Token 使用合理 +0.1
	if resp.TotalTokens > 100 && resp.TotalTokens < 4000 {
		score += 0.1
	}

	// 4. 内容有意义 +0.1
	if len(resp.Content) > 0 && !strings.Contains(resp.Content, "错误") && !strings.Contains(resp.Content, "error") {
		score += 0.1
	}

	return score
}

// recordABTestResult 记录 A/B 实验结果
func (s *ChatService) recordABTestResult(ctx context.Context, experimentID, sessionID string, isVariant bool, score, latencyMs float64, success bool) {
	if s.harnessClient == nil {
		return
	}

	_, err := s.harnessClient.RecordABTestResult(ctx, &harnesspb.RecordABTestResultRequest{
		ExperimentId: experimentID,
		SessionId:    sessionID,
		IsVariant:    isVariant,
		Score:        score,
		LatencyMs:    latencyMs,
		Success:      success,
	})
	if err != nil {
		fmt.Printf("Warning: failed to record A/B test result: %v\n", err)
	}
}

// recordCostUsage records cost usage to harness service
func (s *ChatService) recordCostUsage(ctx context.Context, agentID, modelID, sessionID string, promptTokens, totalTokens int) {
	if s.harnessClient == nil {
		return
	}

	inputTokens := int64(promptTokens)
	outputTokens := int64(totalTokens - promptTokens)
	if outputTokens < 0 {
		outputTokens = 0
	}

	_, err := s.harnessClient.RecordCostUsage(ctx, &harnesspb.RecordCostUsageRequest{
		AgentId:      agentID,
		ModelId:      modelID,
		SessionId:    sessionID,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	})
	if err != nil {
		fmt.Printf("Warning: failed to record cost usage: %v\n", err)
	}
}

// recordCatalogUsage records catalog usage to harness service
func (s *ChatService) recordCatalogUsage(ctx context.Context, agentID string) {
	if s.harnessClient == nil {
		return
	}

	_, err := s.harnessClient.RecordCatalogUsage(ctx, &harnesspb.RecordCatalogUsageRequest{
		AgentId: agentID,
	})
	if err != nil {
		fmt.Printf("Warning: failed to record catalog usage: %v\n", err)
	}
}

// chatLLMMetricsCallback returns a metrics callback that logs LLM call metrics and sends to harness
func chatLLMMetricsCallback(harnessClient harnesspb.HarnessServiceClient) llm.MetricsCallback {
	return func(ctx context.Context, m *llm.CallMetrics) {
		status := "success"
		if !m.Success {
			status = "error"
		}
		fmt.Printf("[LLM Metrics] caller=%s model=%s latency=%dms tokens=%d cost=%.6f status=%s\n",
			m.Caller, m.Model, m.LatencyMs, m.TotalTokens, m.Cost, status)

		// Send metrics to harness service for SLO tracking
		if harnessClient != nil {
			_, err := harnessClient.RecordLLMMetrics(ctx, &harnesspb.RecordLLMMetricsRequest{
				AgentId:     m.Caller,
				Model:       m.Model,
				LatencyMs:   int64(m.LatencyMs),
				InputTokens: int64(m.TotalTokens * 6 / 10),
				OutputTokens: int64(m.TotalTokens * 4 / 10),
				Cost:        m.Cost,
				Success:     m.Success,
			})
			if err != nil {
				fmt.Printf("[LLM Metrics] Failed to send to harness: %v\n", err)
			} else {
				fmt.Printf("[LLM Metrics] Sent to harness: agent=%s latency=%dms success=%v\n", m.Caller, m.LatencyMs, m.Success)
			}
		}
	}
}
