// Package service provides business logic for agent service
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"agent-platform/pkg/agent"
	"agent-platform/pkg/config"
	"agent-platform/pkg/llm"
	pb "agent-platform/pkg/pb/agent"
	commonpb "agent-platform/pkg/pb/common"
	mcppb "agent-platform/pkg/pb/mcp"
)

// AgentService provides multi-agent orchestration
type AgentService struct {
	pb.UnimplementedAgentServiceServer
	registry  *agent.Registry
	llmClient llm.Client
	mcpClient mcppb.MCPServiceClient
	store     agent.ContextStore
	cfg       *config.Config
	engine    *agent.Engine
}

// NewAgentService creates a new agent service
func NewAgentService(registry *agent.Registry, llmClient llm.Client, mcpClient mcppb.MCPServiceClient, store agent.ContextStore, cfg *config.Config) *AgentService {
	s := &AgentService{
		registry:  registry,
		llmClient: llmClient,
		mcpClient: mcpClient,
		store:     store,
		cfg:       cfg,
	}

	// Create execution engine
	s.engine = agent.NewEngine(
		registry,
		&llmAdapter{client: llmClient},
		&toolAdapter{mcpClient: mcpClient},
		store,
		agent.DefaultEngineConfig(),
	)

	return s
}

// RegisterAgent registers a new agent
func (s *AgentService) RegisterAgent(ctx context.Context, req *pb.RegisterAgentRequest) (*pb.RegisterAgentResponse, error) {
	ag := &agent.Agent{
		ID:           req.Agent.Id,
		Name:         req.Agent.Name,
		Description:  req.Agent.Description,
		Instructions: req.Agent.Instructions,
		Tools:        req.Agent.Tools,
		Handoffs:     req.Agent.Handoffs,
		Model:        req.Agent.Model,
		MaxTokens:    int(req.Agent.MaxTokens),
		Temperature:  req.Agent.Temperature,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.registry.Register(ag); err != nil {
		return nil, err
	}

	return &pb.RegisterAgentResponse{
		Agent: s.toProtoAgent(ag),
	}, nil
}

// UnregisterAgent unregisters an agent
func (s *AgentService) UnregisterAgent(ctx context.Context, req *pb.UnregisterAgentRequest) (*pb.UnregisterAgentResponse, error) {
	if err := s.registry.Unregister(req.AgentId); err != nil {
		return nil, err
	}

	return &pb.UnregisterAgentResponse{Success: true}, nil
}

// GetAgent gets an agent by ID
func (s *AgentService) GetAgent(ctx context.Context, req *pb.GetAgentRequest) (*pb.GetAgentResponse, error) {
	ag := s.registry.Get(req.AgentId)
	if ag == nil {
		return nil, agent.ErrAgentNotFound
	}

	return &pb.GetAgentResponse{
		Agent: s.toProtoAgent(ag),
	}, nil
}

// ListAgents lists all registered agents
func (s *AgentService) ListAgents(ctx context.Context, req *pb.ListAgentsRequest) (*pb.ListAgentsResponse, error) {
	agents := s.registry.List()

	resp := &pb.ListAgentsResponse{
		Pagination: &commonpb.PaginationResponse{
			Total:    int32(len(agents)),
			Page:     1,
			PageSize: int32(len(agents)),
		},
	}

	for _, ag := range agents {
		resp.Agents = append(resp.Agents, s.toProtoAgent(ag))
	}

	return resp, nil
}

// Execute executes a multi-agent workflow
func (s *AgentService) Execute(ctx context.Context, req *pb.ExecuteRequest) (*pb.ExecuteResponse, error) {
	// Convert context vars
	contextVars := make(map[string]any)
	for k, v := range req.ContextVars {
		contextVars[k] = v
	}

	// Create execution request
	execReq := &agent.ExecutionRequest{
		SessionID:   req.SessionId,
		TenantID:    req.TenantId,
		UserID:      req.UserId,
		Message:     req.Message,
		EntryAgent:  req.EntryAgent,
		ContextVars: contextVars,
	}

	// Execute
	result, err := s.engine.Run(ctx, execReq)
	if err != nil {
		return &pb.ExecuteResponse{
			Status: string(agent.AgentStatusError),
			Error:  err.Error(),
		}, nil
	}

	// Convert result
	resp := &pb.ExecuteResponse{
		ContextId:    result.ContextID,
		SessionId:    result.SessionID,
		Response:     result.Response,
		TotalTokens:  int32(result.TotalTokens),
		TotalCost:    result.TotalCost,
		Status:       string(result.Status),
		Error:        result.Error,
	}

	for _, r := range result.AgentHistory {
		resp.AgentHistory = append(resp.AgentHistory, &pb.AgentExecutionRecord{
			AgentId:     r.AgentID,
			AgentName:   r.AgentName,
			Thought:     r.Thought,
			Action:      r.Action,
			Arguments:   r.Arguments,
			Result:      r.Result,
			HandoffTo:   r.HandoffTo,
			TokensUsed:  int32(r.TokensUsed),
			StartedAt:   r.StartedAt.Unix(),
			CompletedAt: r.CompletedAt.Unix(),
			DurationMs:  r.Duration,
		})
	}

	return resp, nil
}

// ExecuteStream executes with streaming
func (s *AgentService) ExecuteStream(req *pb.ExecuteStreamRequest, stream pb.AgentService_ExecuteStreamServer) error {
	ctx := stream.Context()

	// For now, execute and send results
	execReq := &pb.ExecuteRequest{
		SessionId:   req.SessionId,
		TenantId:    req.TenantId,
		UserId:      req.UserId,
		Message:     req.Message,
		EntryAgent:  req.EntryAgent,
		ContextVars: req.ContextVars,
	}

	result, err := s.Execute(ctx, execReq)
	if err != nil {
		return err
	}

	// Send agent steps
	for _, step := range result.AgentHistory {
		stream.Send(&pb.ExecuteStreamChunk{
			Type:         "agent_step",
			Content:      fmt.Sprintf("[%s] %s", step.AgentName, step.Action),
			Step:         step,
			CurrentAgent: step.AgentId,
		})
	}

	// Send final response
	stream.Send(&pb.ExecuteStreamChunk{
		Type:    "response",
		Content: result.Response,
	})

	return nil
}

// GetContext gets an execution context
func (s *AgentService) GetContext(ctx context.Context, req *pb.GetContextRequest) (*pb.GetContextResponse, error) {
	execCtx, err := s.store.Get(ctx, req.ContextId)
	if err != nil {
		return nil, err
	}

	return &pb.GetContextResponse{
		Context: s.toProtoContext(execCtx),
	}, nil
}

// toProtoAgent converts agent to proto
func (s *AgentService) toProtoAgent(ag *agent.Agent) *pb.Agent {
	return &pb.Agent{
		Id:           ag.ID,
		Name:         ag.Name,
		Description:  ag.Description,
		Instructions: ag.Instructions,
		Tools:        ag.Tools,
		Handoffs:     ag.Handoffs,
		Model:        ag.Model,
		MaxTokens:    int32(ag.MaxTokens),
		Temperature:  ag.Temperature,
		CreatedAt:    ag.CreatedAt.Unix(),
		UpdatedAt:    ag.UpdatedAt.Unix(),
	}
}

// toProtoContext converts context to proto
func (s *AgentService) toProtoContext(execCtx *agent.ExecutionContext) *pb.ExecutionContext {
	ctx := &pb.ExecutionContext{
		Id:           execCtx.ID,
		SessionId:    execCtx.SessionID,
		TenantId:     execCtx.TenantID,
		UserId:       execCtx.UserID,
		CurrentAgent: execCtx.CurrentAgent,
		EntryAgent:   execCtx.EntryAgent,
		Status:       string(execCtx.Status),
		TotalTokens:  int32(execCtx.TotalTokens),
		TotalCost:    execCtx.TotalCost,
		Error:        execCtx.Error,
		StartedAt:    execCtx.StartedAt.Unix(),
		StepCount:    int32(execCtx.StepCount),
	}

	if execCtx.CompletedAt != nil {
		ctx.CompletedAt = execCtx.CompletedAt.Unix()
	}

	// Convert variables
	for k, v := range execCtx.Variables {
		if s, ok := v.(string); ok {
			ctx.Variables[k] = s
		}
	}

	// Convert messages
	for _, m := range execCtx.Messages {
		ctx.Messages = append(ctx.Messages, &pb.Message{
			Role:    m.Role,
			Content: m.Content,
			Name:    m.Name,
			ToolId:  m.ToolID,
		})
	}

	// Convert agent history
	for _, r := range execCtx.AgentHistory {
		ctx.AgentHistory = append(ctx.AgentHistory, &pb.AgentExecutionRecord{
			AgentId:     r.AgentID,
			AgentName:   r.AgentName,
			Thought:     r.Thought,
			Action:      r.Action,
			Arguments:   r.Arguments,
			Result:      r.Result,
			HandoffTo:   r.HandoffTo,
			TokensUsed:  int32(r.TokensUsed),
			StartedAt:   r.StartedAt.Unix(),
			CompletedAt: r.CompletedAt.Unix(),
			DurationMs:  r.Duration,
		})
	}

	return ctx
}

// llmAdapter adapts llm.Client to agent.LLMClient
type llmAdapter struct {
	client llm.Client
}

func (a *llmAdapter) Chat(ctx context.Context, req *agent.LLMRequest) (*agent.LLMResponse, error) {
	// Convert messages
	messages := make([]llm.Message, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, llm.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	// Convert tools
	tools := make([]llm.ToolDefinition, 0, len(req.Tools))
	for _, t := range req.Tools {
		if fn, ok := t["function"].(map[string]any); ok {
			td := llm.ToolDefinition{Type: "function"}
			if name, ok := fn["name"].(string); ok {
				td.Function.Name = name
			}
			if desc, ok := fn["description"].(string); ok {
				td.Function.Description = desc
			}
			if params, ok := fn["parameters"].(map[string]any); ok {
				td.Function.Parameters = params
			}
			tools = append(tools, td)
		}
	}

	// Call LLM
	resp, err := a.client.Chat(ctx, &llm.ChatRequest{
		Messages:    messages,
		Tools:       tools,
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	})
	if err != nil {
		return nil, err
	}

	// Convert response
	result := &agent.LLMResponse{
		Content:     resp.Content,
		TotalTokens: resp.TotalTokens,
		Cost:        resp.Cost,
		StopReason:  resp.FinishReason,
	}

	for _, tc := range resp.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, agent.ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Arguments: json.RawMessage(tc.Function.Arguments),
		})
	}

	return result, nil
}

// toolAdapter adapts MCP client to ToolExecutor
type toolAdapter struct {
	mcpClient mcppb.MCPServiceClient
}

func (a *toolAdapter) Execute(ctx context.Context, toolName string, arguments json.RawMessage, toolConfig *agent.ToolSpecificConfig) (string, error) {
	if a.mcpClient == nil {
		return "", fmt.Errorf("MCP service not available")
	}

	// 使用更长的超时（Browser Agent 需要时间）
	toolCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second) // 5 分钟
	defer cancel()

	// Build request with tool config
	req := &mcppb.CallToolRequest{
		Name:      toolName,
		Arguments: string(arguments),
	}

	// Add tool config if provided
	if toolConfig != nil {
		configBytes, _ := json.Marshal(toolConfig)
		req.ToolConfig = string(configBytes)
	}

	resp, err := a.mcpClient.CallTool(toolCtx, req)
	if err != nil {
		return "", err
	}

	if resp.IsError {
		return resp.Content, fmt.Errorf("tool error: %s", resp.Content)
	}

	return resp.Content, nil
}

func (a *toolAdapter) ListTools(ctx context.Context) (map[string]any, error) {
	if a.mcpClient == nil {
		return make(map[string]any), nil
	}

	resp, err := a.mcpClient.ListTools(ctx, &mcppb.ListToolsRequest{})
	if err != nil {
		return nil, err
	}

	tools := make(map[string]any)
	for _, t := range resp.Tools {
		var params map[string]any
		json.Unmarshal([]byte(t.InputSchema), &params)

		tools[t.Name] = map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  params,
			},
		}
	}

	return tools, nil
}
