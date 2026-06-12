// Package service provides business logic for chat service
package service

import (
	"context"
	"encoding/json"
	"fmt"
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
	cfg           *config.Config
	maxSteps      int
	useMultiAgent bool
	enableMemory  bool // 是否启用长期记忆
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

	return &ChatService{
		llmClient:     llmClient,
		qdrant:        qdrant,
		sessionRepo:   sessionRepo,
		mcpClient:     mcpClient,
		agentClient:   agentClient,
		agentConn:     agentConn,
		memoryClient:  memoryClient,
		memoryConn:    memoryConn,
		cfg:           cfg,
		maxSteps:      5,
		useMultiAgent: agentClient != nil,
		enableMemory:  memoryClient != nil,
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
}

// Chat handles a chat request - 使用多 Agent 架构
func (s *ChatService) Chat(ctx context.Context, req *pb.ChatRequest) (*pb.ChatResponse, error) {
	// 如果有 Agent Service，使用多 Agent 架构
	if s.useMultiAgent && s.agentClient != nil {
		return s.chatWithMultiAgent(ctx, req)
	}

	// 否则使用原有的单 Agent 逻辑
	return s.chatWithSingleAgent(ctx, req)
}

// chatWithMultiAgent 使用多 Agent 架构处理对话
func (s *ChatService) chatWithMultiAgent(ctx context.Context, req *pb.ChatRequest) (*pb.ChatResponse, error) {
	// ★ 先创建或获取 session（与 chatWithSingleAgent 保持一致）
	session, err := s.getOrCreateSession(ctx, req.SessionId, req.TenantId, req.UserId)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	// ★ 检索长期记忆
	var memoryContext string
	if s.enableMemory && s.memoryClient != nil {
		memories, err := s.recallMemories(ctx, req.Message, session.ID, req.TenantId)
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
	agentCtx, agentCancel := context.WithTimeout(context.Background(), 600*time.Second) // 10 分钟
	defer agentCancel()
	resp, err := s.agentClient.Execute(agentCtx, agentReq)
	if err != nil {
		// 记录错误但不降级，让用户知道 MultiAgent 执行失败
		fmt.Printf("Multi-agent execution failed: %v\n", err)
		return nil, fmt.Errorf("multi-agent execution failed: %w", err)
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

	// ★ 保存助手消息到 session
	assistantMsg := &repository.Message{
		Role:      "assistant",
		Content:   resp.Response,
		CreatedAt: time.Now(),
	}
	session.Messages = append(session.Messages, assistantMsg)

	// ★ 保存 session
	if err := s.sessionRepo.Save(ctx, session); err != nil {
		fmt.Printf("Warning: Failed to save session: %v\n", err)
	}

	// ★ 保存重要信息到长期记忆
	if s.enableMemory && resp.Status == "completed" {
		go s.saveConversationMemory(context.Background(), session.ID, req.TenantId, req.Message, resp.Response)
	}

	// 转换响应（使用 session.ID，而不是 resp.SessionId）
	result := &pb.ChatResponse{
		SessionId:   session.ID, // ★ 使用本地创建的 session ID
		Content:     resp.Response,
		TotalTokens: resp.TotalTokens,
		Cost:        resp.TotalCost,
	}

	// 转换 agent history
	for _, h := range resp.AgentHistory {
		argsJSON := h.Arguments
		if argsJSON == "" {
			argsJSON = "{}"
		}
		result.AgentStates = append(result.AgentStates, &pb.AgentState{
			Thought:   h.Thought,
			Action:    h.Action,
			Arguments: argsJSON,
			Result:    h.Result,
			Step:      int32(h.TokensUsed / 100), // 简化
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
	if s.enableMemory && s.memoryClient != nil {
		memories, err := s.recallMemories(ctx, req.Message, session.ID, req.TenantId)
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

	if err := s.sessionRepo.Save(ctx, session); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}

	// ★ 异步保存重要信息到长期记忆
	if s.enableMemory {
		go s.saveConversationMemory(context.Background(), session.ID, req.TenantId, req.Message, finalContent)
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

// recallMemories 检索相关长期记忆
func (s *ChatService) recallMemories(ctx context.Context, query, sessionID, tenantID string) ([]*memorypb.MemoryEntry, error) {
	if s.memoryClient == nil {
		return nil, fmt.Errorf("memory service not available")
	}

	resp, err := s.memoryClient.Recall(ctx, &memorypb.RecallMemoryRequest{
		Query:     query,
		SessionId: sessionID,
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
	sb.WriteString("【相关历史记忆】\n")
	for _, m := range memories {
		sb.WriteString(fmt.Sprintf("- %s\n", m.Content))
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

// ChatStream handles a streaming chat request
func (s *ChatService) ChatStream(req *pb.ChatRequest, stream pb.ChatService_ChatStreamServer) error {
	ctx := stream.Context()

	// For streaming, we use agent loop but send updates
	resp, err := s.Chat(ctx, req)
	if err != nil {
		return err
	}

	// Send intermediate states first
	for _, state := range resp.AgentStates {
		stream.Send(&pb.ChatStreamChunk{
			Type:    "agent_step",
			Content: fmt.Sprintf("[Step %d] Thought: %s\nAction: %s\nResult: %s", state.Step, state.Thought, state.Action, state.Result),
		})
	}

	// Send final content
	return stream.Send(&pb.ChatStreamChunk{
		Type:    "final",
		Content: resp.Content,
	})
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

	// Add user message
	messages = append(messages, llm.Message{
		Role:    "user",
		Content: userMessage,
	})

	return messages
}

func (s *ChatService) toProtoSession(session *repository.Session) *pb.Session {
	pbSession := &pb.Session{
		Id:        session.ID,
		Title:     session.Title,
		CreatedAt: session.CreatedAt.Unix(),
		UpdatedAt: session.UpdatedAt.Unix(),
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
