// Package service provides business logic for agent service
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"agent-platform/pkg/agent"
	"agent-platform/pkg/agent/approval"
	"agent-platform/pkg/agent/checkpoint"
	"agent-platform/pkg/agent/intervention"
	"agent-platform/pkg/config"
	"agent-platform/pkg/llm"
	pb "agent-platform/pkg/pb/agent"
	commonpb "agent-platform/pkg/pb/common"
	mcppb "agent-platform/pkg/pb/mcp"
	memorypb "agent-platform/pkg/pb/memory"
	harnesspb "agent-platform/pkg/pb/harness"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AgentService provides multi-agent orchestration
type AgentService struct {
	pb.UnimplementedAgentServiceServer
	registry      *agent.Registry
	llmClient     llm.Client
	mcpClient     mcppb.MCPServiceClient
	store         agent.ContextStore
	cfg           *config.Config
	engine        *agent.Engine
	harnessClient harnesspb.HarnessServiceClient
	harnessConn   *grpc.ClientConn
}

// NewAgentService creates a new agent service
func NewAgentService(registry *agent.Registry, llmClient llm.Client, mcpClient mcppb.MCPServiceClient, memoryClient memorypb.MemoryServiceClient, store agent.ContextStore, cfg *config.Config) *AgentService {
	// Wrap LLM client with metrics collection
	metricsLLM := llm.NewMetricsClient(llmClient, defaultLLMMetricsCallback(), "engine")

	// Connect to harness service for catalog sync
	var harnessClient harnesspb.HarnessServiceClient
	var harnessConn *grpc.ClientConn
	if harnessAddr := cfg.Services.Harness; harnessAddr != "" {
		conn, err := grpc.Dial(harnessAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err == nil {
			harnessClient = harnesspb.NewHarnessServiceClient(conn)
			harnessConn = conn
			fmt.Printf("Connected to Harness Service at %s\n", harnessAddr)
		} else {
			fmt.Printf("Warning: Failed to connect to Harness Service: %v\n", err)
		}
	}

	s := &AgentService{
		registry:      registry,
		llmClient:     llmClient,
		mcpClient:     mcpClient,
		store:         store,
		cfg:           cfg,
		harnessClient: harnessClient,
		harnessConn:   harnessConn,
	}

	// Create execution engine with metrics-wrapped LLM client
	s.engine = agent.NewEngine(
		registry,
		&llmAdapter{client: metricsLLM},
		&toolAdapter{mcpClient: mcpClient},
		store,
		agent.DefaultEngineConfig(),
	)

	// Wire memory client into the engine (graceful: nil means agent runs without memory).
	// This connects the recall/write calls in the execution loop to the memory service.
	if memoryClient != nil {
		s.engine.SetMemoryClient(&memoryAdapter{client: memoryClient})
	}

	return s
}

// defaultLLMMetricsCallback returns a default metrics callback that logs LLM call metrics
// and sends them to Harness Service for SLO tracking
func defaultLLMMetricsCallback() llm.MetricsCallback {
	return func(ctx context.Context, m *llm.CallMetrics) {
		status := "success"
		if !m.Success {
			status = "error"
		}
		fmt.Printf("[LLM Metrics] caller=%s model=%s latency=%dms tokens=%d cost=%.6f status=%s\n",
			m.Caller, m.Model, m.LatencyMs, m.TotalTokens, m.Cost, status)

		// Send to Harness Service via HTTP for SLO tracking
		// This is a fire-and-forget operation, errors are ignored
		go func() {
			payload := map[string]interface{}{
				"agent_id":     "main-agent", // Default agent for metrics
				"model":        m.Model,
				"latency_ms":   m.LatencyMs,
				"total_tokens": m.TotalTokens,
				"cost":         m.Cost,
				"success":      m.Success,
				"caller":       m.Caller,
			}
			body, _ := json.Marshal(payload)
			// Post to Harness Service internal endpoint (port 50008)
			resp, err := http.Post("http://harness-service:50008/internal/metrics/llm", "application/json", bytes.NewReader(body))
			if err == nil {
				resp.Body.Close()
			}
		}()
	}
}

// RegisterAgent registers a new agent (with persistence)
func (s *AgentService) RegisterAgent(ctx context.Context, req *pb.RegisterAgentRequest) (*pb.RegisterAgentResponse, error) {
	ag := &agent.Agent{
		ID:                req.Agent.Id,
		Name:              req.Agent.Name,
		Description:       req.Agent.Description,
		Instructions:      req.Agent.Instructions,
		PromptTemplateKey: req.Agent.PromptTemplateKey,
		Tools:             req.Agent.Tools,
		Handoffs:          req.Agent.Handoffs,
		Model:             req.Agent.Model,
		MaxTokens:         int(req.Agent.MaxTokens),
		Temperature:       req.Agent.Temperature,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	// Use persistence-enabled registration
	if err := s.registry.RegisterOrUpdateWithPersistence(ctx, ag); err != nil {
		return nil, err
	}

	// Sync to harness catalog
	s.syncToCatalog(ctx, ag)

	return &pb.RegisterAgentResponse{
		Agent: s.toProtoAgent(ag),
	}, nil
}

// UnregisterAgent unregisters an agent (with persistence)
func (s *AgentService) UnregisterAgent(ctx context.Context, req *pb.UnregisterAgentRequest) (*pb.UnregisterAgentResponse, error) {
	// Use persistence-enabled unregistration
	if err := s.registry.UnregisterWithPersistence(ctx, req.AgentId); err != nil {
		return nil, err
	}

	return &pb.UnregisterAgentResponse{Success: true}, nil
}

// syncToCatalog syncs an agent to the harness catalog
func (s *AgentService) syncToCatalog(ctx context.Context, ag *agent.Agent) {
	if s.harnessClient == nil {
		return
	}

	toolsJSON, _ := json.Marshal(ag.Tools)
	_, err := s.harnessClient.RegisterCatalogAgent(ctx, &harnesspb.RegisterCatalogAgentRequest{
		AgentId:       ag.ID,
		Name:          ag.Name,
		Type:          "chat",
		Description:   ag.Description,
		Version:       "1.0",
		Configuration: ag.Instructions,
		Capabilities:  string(toolsJSON),
		Tags:          "agent",
	})
	if err != nil {
		fmt.Printf("Warning: failed to sync agent to catalog: %v\n", err)
	}
}// GetAgent gets an agent by ID
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

	// Resolve system prompt: render from Prompt Management if agent has PromptTemplateKey
	systemPromptOverride := s.renderSystemPrompt(ctx, req.EntryAgent, req.SessionId, contextVars)

	// Create execution request
	execReq := &agent.ExecutionRequest{
		SessionID:            req.SessionId,
		TenantID:             req.TenantId,
		UserID:               req.UserId,
		Message:              req.Message,
		EntryAgent:           req.EntryAgent,
		ContextVars:          contextVars,
		SystemPromptOverride: systemPromptOverride,
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

// ExecuteStream executes with real-time streaming
func (s *AgentService) ExecuteStream(req *pb.ExecuteStreamRequest, stream pb.AgentService_ExecuteStreamServer) error {
	ctx := stream.Context()

	// Convert context vars
	contextVars := make(map[string]any)
	for k, v := range req.ContextVars {
		contextVars[k] = v
	}

	// Resolve system prompt: render from Prompt Management if agent has PromptTemplateKey
	systemPromptOverride := s.renderSystemPrompt(ctx, req.EntryAgent, req.SessionId, contextVars)

	// Create execution request
	execReq := &agent.ExecutionRequest{
		SessionID:            req.SessionId,
		TenantID:             req.TenantId,
		UserID:               req.UserId,
		Message:              req.Message,
		EntryAgent:           req.EntryAgent,
		ContextVars:          contextVars,
		SystemPromptOverride: systemPromptOverride,
	}

	// Run with streaming — each event is forwarded to the gRPC stream in real time
	_, err := s.engine.RunStream(ctx, execReq, func(event agent.StreamEvent) {
		// Convert StreamEvent to ExecuteStreamChunk
		chunk := &pb.ExecuteStreamChunk{
			Type:         string(event.Type),
			Content:      event.Content,
			CurrentAgent: event.AgentID,
		}

		// Add step info for tool events
		if event.Step > 0 {
			chunk.Step = &pb.AgentExecutionRecord{
				AgentId:   event.AgentID,
				AgentName: event.AgentName,
				Action:    event.ToolName,
				Result:    event.ToolResult,
			}
		}

		// Add tool call info
		if event.ToolName != "" {
			chunk.ToolCall = &pb.ToolCall{
				Name:      event.ToolName,
				Arguments: string(event.ToolArgs),
			}
		}

		stream.Send(chunk)
	})

	if err != nil {
		// Send error event before returning
		stream.Send(&pb.ExecuteStreamChunk{
			Type:    "error",
			Content: err.Error(),
		})
		return err
	}

	return nil
}

// renderSystemPrompt resolves the system prompt from Prompt Management.
// If the entry agent has a PromptTemplateKey, it calls harness to render the template.
// Returns empty string if no template key, harness is down, or rendering fails (fallback to Instructions).
func (s *AgentService) renderSystemPrompt(ctx context.Context, entryAgentID, sessionID string, contextVars map[string]any) string {
	if s.harnessClient == nil || entryAgentID == "" {
		return ""
	}

	// Look up the agent to check for PromptTemplateKey
	ag := s.registry.Get(entryAgentID)
	if ag == nil || ag.PromptTemplateKey == "" {
		return ""
	}

	// Serialize context variables for template rendering
	varsJSON := "{}"
	if len(contextVars) > 0 {
		if b, err := json.Marshal(contextVars); err == nil {
			varsJSON = string(b)
		}
	}

	// Call harness to render the prompt template
	renderResp, err := s.harnessClient.RenderPrompt(ctx, &harnesspb.RenderPromptRequest{
		PromptKey: ag.PromptTemplateKey,
		Variables: varsJSON,
		AgentId:   ag.ID,
		SessionId: sessionID,
	})
	if err != nil {
		fmt.Printf("[AgentService] Prompt render failed for key=%s, falling back to Instructions: %v\n", ag.PromptTemplateKey, err)
		return ""
	}

	if renderResp.Content == "" {
		fmt.Printf("[AgentService] Prompt render returned empty for key=%s, falling back to Instructions\n", ag.PromptTemplateKey)
		return ""
	}

	fmt.Printf("[AgentService] Using rendered prompt from template key=%s (len=%d)\n", ag.PromptTemplateKey, len(renderResp.Content))
	return renderResp.Content
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
		Id:                ag.ID,
		Name:              ag.Name,
		Description:       ag.Description,
		Instructions:      ag.Instructions,
		PromptTemplateKey: ag.PromptTemplateKey,
		Tools:             ag.Tools,
		Handoffs:          ag.Handoffs,
		Model:             ag.Model,
		MaxTokens:         int32(ag.MaxTokens),
		Temperature:       ag.Temperature,
		CreatedAt:         ag.CreatedAt.Unix(),
		UpdatedAt:         ag.UpdatedAt.Unix(),
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

// GetApprovalManager returns the engine's approval manager for HTTP API access
func (s *AgentService) GetApprovalManager() *approval.ApprovalFlowManager {
	if s.engine == nil {
		return nil
	}
	return s.engine.GetApprovalManager()
}

// GetRuleEngine returns the engine's rule engine for HTTP API access
func (s *AgentService) GetRuleEngine() *approval.RuleEngine {
	if s.engine == nil {
		return nil
	}
	return s.engine.GetRuleEngine()
}

// GetInterventionManager returns the engine's intervention manager for HTTP API access
func (s *AgentService) GetInterventionManager() *intervention.InterventionManager {
	if s.engine == nil {
		return nil
	}
	return s.engine.GetInterventionManager()
}

// SetCheckpointStore wires a checkpoint store into the engine for crash recovery.
// The store is fully implemented in pkg/agent/checkpoint; this connects it so that
// each step is persisted and ResumeFromCheckpoint becomes usable.
func (s *AgentService) SetCheckpointStore(store checkpoint.CheckpointStore) {
	if s.engine != nil {
		s.engine.SetCheckpointStore(store)
	}
}

// llmAdapter adapts llm.Client to agent.LLMClient
// It also implements agent.LLMStreamingClient when the underlying client supports streaming
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
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: json.RawMessage(tc.Function.Arguments),
		})
	}

	return result, nil
}

// ChatStream implements agent.LLMStreamingClient — streams tokens from the LLM
func (a *llmAdapter) ChatStream(ctx context.Context, req *agent.LLMRequest) (<-chan agent.LLMStreamChunk, error) {
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

	// Call streaming LLM
	llmCh, err := a.client.ChatStream(ctx, &llm.ChatRequest{
		Messages:    messages,
		Tools:       tools,
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	})
	if err != nil {
		return nil, err
	}

	// Convert llm.ChatStreamChunk channel to agent.LLMStreamChunk channel
	out := make(chan agent.LLMStreamChunk, 100)
	go func() {
		defer close(out)
		for chunk := range llmCh {
			if chunk.Error != nil {
				out <- agent.LLMStreamChunk{Error: chunk.Error}
				return
			}
			if chunk.Done {
				out <- agent.LLMStreamChunk{Done: true}
				return
			}
			if chunk.Content != "" {
				out <- agent.LLMStreamChunk{Content: chunk.Content}
			}
		}
		// If the channel closed without a Done marker, send one
		out <- agent.LLMStreamChunk{Done: true}
	}()

	return out, nil
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
	toolCtx, cancel := context.WithTimeout(context.Background(), 600*time.Second) // 10 分钟
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

// memoryAdapter adapts the memory-service gRPC client to agent.MemoryClient.
// It is the bridge that lets the execution loop recall/write memories.
type memoryAdapter struct {
	client memorypb.MemoryServiceClient
}

// Recall fetches up to 5 relevant memories and formats them as bullet lines.
func (a *memoryAdapter) Recall(ctx context.Context, sessionID, tenantID, query string) (string, error) {
	if a.client == nil {
		return "", nil
	}
	resp, err := a.client.Recall(ctx, &memorypb.RecallMemoryRequest{
		Query:     query,
		SessionId: sessionID,
		TenantId:  tenantID,
		TopK:      5,
	})
	if err != nil {
		return "", err
	}
	if resp == nil || len(resp.Memories) == 0 {
		return "", nil
	}
	var sb strings.Builder
	for i, m := range resp.Memories {
		sb.WriteString(fmt.Sprintf("- %s\n", m.Content))
		if i >= 4 {
			break
		}
	}
	return sb.String(), nil
}

// Write stores a step outcome as an IMPORTANT memory (episodic-style).
func (a *memoryAdapter) Write(ctx context.Context, sessionID, tenantID, agentID, content string, importance float64) error {
	if a.client == nil {
		return nil
	}
	_, err := a.client.Save(ctx, &memorypb.SaveMemoryRequest{
		SessionId:  sessionID,
		AgentId:    agentID,
		Type:       memorypb.MemoryType_MEMORY_TYPE_IMPORTANT,
		Content:    content,
		Importance: importance,
		TenantId:   tenantID,
	})
	return err
}
