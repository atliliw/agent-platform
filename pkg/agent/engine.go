package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// EngineConfig holds engine configuration
type EngineConfig struct {
	// MaxSteps is the maximum number of agent loop iterations
	MaxSteps int

	// MaxHistoryLength is the maximum message history length
	MaxHistoryLength int
}

// DefaultEngineConfig returns default engine configuration
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		MaxSteps:         10,
		MaxHistoryLength: 50,
	}
}

// ToolExecutor is the interface for executing tools
type ToolExecutor interface {
	// Execute executes a tool and returns the result
	Execute(ctx context.Context, toolName string, arguments json.RawMessage, toolConfig *ToolSpecificConfig) (string, error)

	// ListTools returns all available tools
	ListTools(ctx context.Context) (map[string]any, error)
}

// LLMClient is the interface for LLM calls
type LLMClient interface {
	// Chat calls the LLM with messages and tools
	Chat(ctx context.Context, req *LLMRequest) (*LLMResponse, error)
}

// LLMRequest represents an LLM request
type LLMRequest struct {
	Messages    []Message         `json:"messages"`
	Tools       []map[string]any  `json:"tools,omitempty"`
	Model       string            `json:"model,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Temperature float64           `json:"temperature,omitempty"`
}

// LLMResponse represents an LLM response
type LLMResponse struct {
	Content     string     `json:"content"`
	ToolCalls   []ToolCall `json:"tool_calls,omitempty"`
	TotalTokens int        `json:"total_tokens"`
	Cost        float64    `json:"cost"`
	StopReason  string     `json:"stop_reason"`
}

// Engine is the multi-agent execution engine
type Engine struct {
	registry  *Registry
	llmClient LLMClient
	tools     ToolExecutor
	config    EngineConfig
	store     ContextStore
}

// NewEngine creates a new execution engine
func NewEngine(registry *Registry, llmClient LLMClient, tools ToolExecutor, store ContextStore, config EngineConfig) *Engine {
	return &Engine{
		registry:  registry,
		llmClient: llmClient,
		tools:     tools,
		config:    config,
		store:     store,
	}
}

// ExecutionRequest represents an execution request
type ExecutionRequest struct {
	SessionID     string         `json:"session_id"`
	TenantID      string         `json:"tenant_id,omitempty"`
	UserID        string         `json:"user_id,omitempty"`
	Message       string         `json:"message"`
	EntryAgent    string         `json:"entry_agent,omitempty"`
	ContextVars   map[string]any `json:"context_vars,omitempty"`
}

// ExecutionResult represents an execution result
type ExecutionResult struct {
	ContextID     string                `json:"context_id"`
	SessionID     string                `json:"session_id"`
	Response      string                `json:"response"`
	AgentHistory  []AgentExecutionRecord `json:"agent_history"`
	TotalTokens   int                   `json:"total_tokens"`
	TotalCost     float64               `json:"total_cost"`
	Status        AgentStatus           `json:"status"`
	Error         string                `json:"error,omitempty"`
}

// Run executes the multi-agent workflow
func (e *Engine) Run(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error) {
	// Create execution context
	execCtx := NewExecutionContext(req.SessionID)
	execCtx.TenantID = req.TenantID
	execCtx.UserID = req.UserID
	execCtx.EntryAgent = req.EntryAgent
	execCtx.MarkRunning()

	// Set initial variables
	for k, v := range req.ContextVars {
		execCtx.SetVariable(k, v)
	}

	// Add user message
	execCtx.AddMessage("user", req.Message)

	// Get entry agent
	entryAgentID := req.EntryAgent
	if entryAgentID == "" {
		defaultAgent := e.registry.GetDefault()
		if defaultAgent == nil {
			return nil, ErrNoDefaultAgent
		}
		entryAgentID = defaultAgent.ID
	}

	// Start with entry agent
	currentAgent := e.registry.Get(entryAgentID)
	if currentAgent == nil {
		return nil, fmt.Errorf("entry agent %s not found", entryAgentID)
	}

	execCtx.CurrentAgent = currentAgent.ID

	// Save context
	if e.store != nil {
		if err := e.store.Save(ctx, execCtx); err != nil {
			return nil, fmt.Errorf("save context: %w", err)
		}
	}

	// Execute agent loop
	result, err := e.executeLoop(ctx, execCtx, currentAgent, req.Message)

	// Save final context
	if e.store != nil {
		e.store.Save(ctx, execCtx)
	}

	return result, err
}

// executeLoop runs the agent execution loop
func (e *Engine) executeLoop(ctx context.Context, execCtx *ExecutionContext, startAgent *Agent, initialMessage string) (*ExecutionResult, error) {
	currentAgent := startAgent
	fmt.Printf("[AgentEngine] Starting executeLoop with agent: %s\n", currentAgent.ID)

	for step := 0; step < e.config.MaxSteps; step++ {
		// Record step start
		stepStart := time.Now()
		execCtx.StepCount = step + 1
		fmt.Printf("[AgentEngine] Step %d: Agent=%s\n", step+1, currentAgent.ID)

		// Build messages for this agent
		messages := e.buildAgentMessages(currentAgent, execCtx)
		fmt.Printf("[AgentEngine] Built %d messages\n", len(messages))

		// Build tools for this agent
		tools, err := e.buildAgentTools(ctx, currentAgent)
		if err != nil {
			fmt.Printf("[AgentEngine] ERROR build tools: %v\n", err)
			return nil, fmt.Errorf("build tools: %w", err)
		}
		fmt.Printf("[AgentEngine] Built %d tools\n", len(tools))

		// Build LLM request
		llmReq := &LLMRequest{
			Messages:    messages,
			Tools:       tools,
			Model:       currentAgent.Model,
			MaxTokens:   currentAgent.MaxTokens,
			Temperature: currentAgent.Temperature,
		}

		// Call LLM
		fmt.Printf("[AgentEngine] Calling LLM...\n")
		llmResp, err := e.llmClient.Chat(ctx, llmReq)
		if err != nil {
			fmt.Printf("[AgentEngine] ERROR LLM call: %v\n", err)
			execCtx.MarkError(err.Error())
			return e.buildResult(execCtx, ""), fmt.Errorf("llm call: %w", err)
		}
		fmt.Printf("[AgentEngine] LLM response: tokens=%d, tool_calls=%d\n", llmResp.TotalTokens, len(llmResp.ToolCalls))

		// Update token counts
		execCtx.AddTokens(llmResp.TotalTokens)
		execCtx.AddCost(llmResp.Cost)

		// Check for tool calls
		if len(llmResp.ToolCalls) == 0 {
			// No tool calls - we're done
			fmt.Printf("[AgentEngine] DONE! ContentLen=%d, Preview: %.200s\n", len(llmResp.Content), llmResp.Content)
			execCtx.AddMessage("assistant", llmResp.Content)
			execCtx.MarkCompleted()

			return e.buildResult(execCtx, llmResp.Content), nil
		}

		// Process tool calls
		for _, tc := range llmResp.ToolCalls {
			// Check if it's a handoff
			if IsHandoffTool(tc.Name) {
				targetID := ParseHandoffTarget(tc.Name)

				// Validate handoff
				if err := ValidateHandoff(currentAgent, targetID, e.registry); err != nil {
					execCtx.MarkError(err.Error())
					return e.buildResult(execCtx, ""), err
				}

				// Record handoff
				record := AgentExecutionRecord{
					AgentID:     currentAgent.ID,
					AgentName:   currentAgent.Name,
					Thought:     llmResp.Content,
					Action:      "handoff",
					Result:      fmt.Sprintf("Transferred to %s", targetID),
					HandoffTo:   targetID,
					TokensUsed:  llmResp.TotalTokens,
					StartedAt:   stepStart,
					CompletedAt: time.Now(),
					Duration:    time.Since(stepStart).Milliseconds(),
				}
				execCtx.AddAgentRecord(record)

				// Switch to target agent
				currentAgent = e.registry.Get(targetID)
				execCtx.SetCurrentAgent(targetID)

				// Save context
				if e.store != nil {
					e.store.Save(ctx, execCtx)
				}

				// Continue with new agent
				break
			}

			// Execute regular tool
			toolStart := time.Now()

			// Get tool config for current agent
			var toolCfg *ToolSpecificConfig
			if currentAgent.ToolConfig != nil {
				if cfg, ok := currentAgent.ToolConfig[tc.Name]; ok {
					toolCfg = &cfg
				}
			}

			result, err := e.tools.Execute(ctx, tc.Name, tc.Arguments, toolCfg)
			toolDuration := time.Since(toolStart).Milliseconds()

			var recordResult string
			var status string
			if err != nil {
				recordResult = fmt.Sprintf("Error: %v", err)
				status = "error"
			} else {
				recordResult = result
				status = "completed"
			}

			// Record tool execution
			tc.Status = status
			tc.Result = result
			execCtx.RecordToolCall(tc)

			// Add to message history
			execCtx.AddToolMessage(tc.Name, tc.ID, result)

			// Record in agent history
			record := AgentExecutionRecord{
				AgentID:     currentAgent.ID,
				AgentName:   currentAgent.Name,
				Thought:     llmResp.Content,
				Action:      tc.Name,
				Arguments:   string(tc.Arguments),
				Result:      recordResult,
				TokensUsed:  llmResp.TotalTokens,
				StartedAt:   stepStart,
				CompletedAt: time.Now(),
				Duration:    toolDuration,
			}
			execCtx.AddAgentRecord(record)
		}

		// Save context after each step
		if e.store != nil {
			e.store.Save(ctx, execCtx)
		}
	}

	// Max steps reached
	execCtx.MarkError("maximum steps reached")
	return e.buildResult(execCtx, "Maximum execution steps reached. Please try a simpler request."), ErrMaxStepsReached
}

// buildAgentMessages builds messages for an agent
func (e *Engine) buildAgentMessages(agent *Agent, execCtx *ExecutionContext) []Message {
	messages := make([]Message, 0)

	// Add system prompt
	systemPrompt := agent.Instructions

	// Add context variables to system prompt
	if len(execCtx.Variables) > 0 {
		varsJSON, _ := json.Marshal(execCtx.Variables)
		systemPrompt += fmt.Sprintf("\n\nCurrent context variables:\n%s", string(varsJSON))
	}

	// Add agent history summary
	if len(execCtx.AgentHistory) > 0 {
		systemPrompt += "\n\nPrevious agent actions:\n"
		for _, r := range execCtx.AgentHistory {
			systemPrompt += fmt.Sprintf("- %s (%s): %s\n", r.AgentName, r.Action, r.Result)
		}
	}

	messages = append(messages, Message{
		Role:    "system",
		Content: systemPrompt,
	})

	// Add recent message history
	start := 0
	if len(execCtx.Messages) > e.config.MaxHistoryLength {
		start = len(execCtx.Messages) - e.config.MaxHistoryLength
	}

	for i := start; i < len(execCtx.Messages); i++ {
		messages = append(messages, execCtx.Messages[i])
	}

	return messages
}

// buildAgentTools builds tools available to an agent
func (e *Engine) buildAgentTools(ctx context.Context, agent *Agent) ([]map[string]any, error) {
	tools := make([]map[string]any, 0)

	// Get all available tools
	allTools, err := e.tools.ListTools(ctx)
	if err == nil {
		// Add agent's regular tools
		for _, toolName := range agent.Tools {
			if def, ok := allTools[toolName]; ok {
				tools = append(tools, def.(map[string]any))
			}
		}
	}

	// Add handoff tools
	for _, targetID := range agent.Handoffs {
		targetAgent := e.registry.Get(targetID)
		if targetAgent != nil {
			ht := NewHandoffTool(targetAgent)
			tools = append(tools, ht.ToToolDefinition())
		}
	}

	return tools, nil
}

// buildResult builds the execution result
func (e *Engine) buildResult(execCtx *ExecutionContext, response string) *ExecutionResult {
	return &ExecutionResult{
		ContextID:    execCtx.ID,
		SessionID:    execCtx.SessionID,
		Response:     response,
		AgentHistory: execCtx.AgentHistory,
		TotalTokens:  execCtx.TotalTokens,
		TotalCost:    execCtx.TotalCost,
		Status:       execCtx.Status,
		Error:        execCtx.Error,
	}
}

// Continue resumes an execution from a previous context
func (e *Engine) Continue(ctx context.Context, contextID string, message string) (*ExecutionResult, error) {
	// Load existing context
	execCtx, err := e.store.Get(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("load context: %w", err)
	}

	if execCtx.Status == AgentStatusCompleted || execCtx.Status == AgentStatusError {
		return nil, fmt.Errorf("execution already %s", execCtx.Status)
	}

	// Add new user message
	execCtx.AddMessage("user", message)

	// Get current agent
	currentAgent := e.registry.Get(execCtx.CurrentAgent)
	if currentAgent == nil {
		return nil, fmt.Errorf("agent %s not found", execCtx.CurrentAgent)
	}

	// Continue execution
	return e.executeLoop(ctx, execCtx, currentAgent, message)
}

// GetContext retrieves an execution context
func (e *Engine) GetContext(ctx context.Context, contextID string) (*ExecutionContext, error) {
	return e.store.Get(ctx, contextID)
}

// ExtractThought extracts thought content from LLM response
func ExtractThought(content string) string {
	// Look for thought markers
	if strings.Contains(content, "<thought>") {
		start := strings.Index(content, "<thought>")
		end := strings.Index(content, "</thought>")
		if end > start {
			return strings.TrimSpace(content[start+9 : end])
		}
	}

	// Use first 200 chars as thought
	if len(content) > 200 {
		return content[:200] + "..."
	}
	return content
}
