package agent

import (
	"agent-platform/pkg/agent/approval"
	"agent-platform/pkg/agent/checkpoint"
	"agent-platform/pkg/agent/intervention"
	"agent-platform/pkg/agent/reflection"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
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
	registry            *Registry
	llmClient           LLMClient
	tools               ToolExecutor
	config              EngineConfig
	store               ContextStore
	approvalManager     *approval.ApprovalFlowManager
	ruleEngine          *approval.RuleEngine
	reflectionLoop      *reflection.ReflectionLoop
	errorAnalyzer       *reflection.ErrorAnalyzer
	interventionManager *intervention.InterventionManager
	checkpointStore     checkpoint.CheckpointStore
	maxParallelWorkers  int
	toolExecTimeout     time.Duration
}

// NewEngine creates a new execution engine
func NewEngine(registry *Registry, llmClient LLMClient, tools ToolExecutor, store ContextStore, config EngineConfig) *Engine {
	e := &Engine{
		registry:           registry,
		llmClient:          llmClient,
		tools:              tools,
		config:             config,
		store:              store,
		maxParallelWorkers: 10,
		toolExecTimeout:    30 * time.Second,
	}

	// Initialize approval system with default rules
	e.approvalManager = approval.NewApprovalFlowManager()
	e.ruleEngine = approval.NewRuleEngine()

	// Initialize reflection system
	e.reflectionLoop = reflection.NewReflectionLoop()
	e.errorAnalyzer = reflection.NewErrorAnalyzer()

	// Initialize intervention system
	e.interventionManager = intervention.NewInterventionManager()

	// Default approval rules — high-risk tools require human approval
	e.ruleEngine.AddRule(&approval.ApprovalRule{
		ID:             "rule-code-execute",
		Type:           approval.ApprovalTypeToolCall,
		ToolName:       "code_execute",
		RiskThreshold:  "high",
		AutoApprove:    false,
		TimeoutSeconds: 300,
	})

	e.ruleEngine.AddRule(&approval.ApprovalRule{
		ID:             "rule-browser-navigate",
		Type:           approval.ApprovalTypeToolCall,
		ToolName:       "browser_navigate",
		RiskThreshold:  "medium",
		AutoApprove:    false,
		TimeoutSeconds: 300,
	})

	e.ruleEngine.AddRule(&approval.ApprovalRule{
		ID:             "rule-browser-click",
		Type:           approval.ApprovalTypeToolCall,
		ToolName:       "browser_click",
		RiskThreshold:  "medium",
		AutoApprove:    false,
		TimeoutSeconds: 300,
	})

	e.ruleEngine.AddRule(&approval.ApprovalRule{
		ID:             "rule-publish",
		Type:           approval.ApprovalTypePublish,
		ToolName:       "csdn_publish",
		RiskThreshold:  "high",
		AutoApprove:    false,
		TimeoutSeconds: 300,
	})

	return e
}

// SetApprovalManager allows overriding the approval manager (for custom configuration)
func (e *Engine) SetApprovalManager(manager *approval.ApprovalFlowManager) {
	e.approvalManager = manager
}

// SetRuleEngine allows overriding the rule engine (for custom rules)
func (e *Engine) SetRuleEngine(re *approval.RuleEngine) {
	e.ruleEngine = re
}

// GetApprovalManager returns the approval manager for external access (e.g., API handlers)
func (e *Engine) GetApprovalManager() *approval.ApprovalFlowManager {
	return e.approvalManager
}

// GetRuleEngine returns the rule engine for external access
func (e *Engine) GetRuleEngine() *approval.RuleEngine {
	return e.ruleEngine
}

// SetInterventionManager allows overriding the intervention manager (for custom configuration)
func (e *Engine) SetInterventionManager(manager *intervention.InterventionManager) {
	e.interventionManager = manager
}

// GetInterventionManager returns the intervention manager for external access (e.g., API handlers)
func (e *Engine) GetInterventionManager() *intervention.InterventionManager {
	return e.interventionManager
}

// SetReflectionLoop allows overriding the reflection loop (for custom configuration)
func (e *Engine) SetReflectionLoop(loop *reflection.ReflectionLoop) {
	e.reflectionLoop = loop
}

// SetErrorAnalyzer allows overriding the error analyzer (for custom configuration)
func (e *Engine) SetErrorAnalyzer(analyzer *reflection.ErrorAnalyzer) {
	e.errorAnalyzer = analyzer
}

// GetReflectionLoop returns the reflection loop for external access
func (e *Engine) GetReflectionLoop() *reflection.ReflectionLoop {
	return e.reflectionLoop
}

// GetErrorAnalyzer returns the error analyzer for external access
func (e *Engine) GetErrorAnalyzer() *reflection.ErrorAnalyzer {
	return e.errorAnalyzer
}

// SetCheckpointStore sets the checkpoint store for persistence
func (e *Engine) SetCheckpointStore(store checkpoint.CheckpointStore) {
	e.checkpointStore = store
}

// GetCheckpointStore returns the checkpoint store
func (e *Engine) GetCheckpointStore() checkpoint.CheckpointStore {
	return e.checkpointStore
}

// SetMaxParallelWorkers sets the maximum number of parallel tool workers (default: 10)
func (e *Engine) SetMaxParallelWorkers(n int) {
	if n > 0 {
		e.maxParallelWorkers = n
	}
}

// SetToolExecTimeout sets the default timeout for individual tool executions (default: 30s)
func (e *Engine) SetToolExecTimeout(d time.Duration) {
	if d > 0 {
		e.toolExecTimeout = d
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

// ============================================================
// Parallel Tool Execution (extracted, semaphore-limited)
// ============================================================

// toolExecResult holds the outcome of a single tool execution within a parallel batch.
type toolExecResult struct {
	ToolCall ToolCall
	Result   string
	Err      error
	Duration int64
	Record   AgentExecutionRecord
	Status   string
}

// executeParallelTools executes regular (non-handoff) tool calls in parallel with
// semaphore-limited concurrency, per-tool timeout, approval checks, and error
// reflection. This replaces the inline goroutine+WaitGroup blocks that were
// previously duplicated in executeLoop and resumeLoop.
func (e *Engine) executeParallelTools(
	ctx context.Context,
	regularCalls []ToolCall,
	execCtx *ExecutionContext,
	currentAgent *Agent,
	llmResp *LLMResponse,
	stepStart time.Time,
) []toolExecResult {
	results := make([]toolExecResult, len(regularCalls))

	if len(regularCalls) == 0 {
		return results
	}

	// Semaphore limits concurrency (same pattern as ParallelExecutor)
	sem := make(chan struct{}, e.maxParallelWorkers)
	var wg sync.WaitGroup

	for i, tc := range regularCalls {
		wg.Add(1)
		go func(idx int, toolCall ToolCall) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			toolStart := time.Now()

			// Per-tool timeout context
			execTimeout := e.toolExecTimeout
			timeoutCtx, cancel := context.WithTimeout(ctx, execTimeout)
			defer cancel()

			// ★ Approval check: does this tool require human approval?
			if e.ruleEngine != nil && e.approvalManager != nil {
				needsApproval, matchedRule := e.ruleEngine.NeedsApproval(currentAgent.ID, toolCall.Name, nil)
				if needsApproval && matchedRule != nil && !matchedRule.AutoApprove {
					fmt.Printf("[AgentEngine] Approval required for tool: %s (rule: %s)\n", toolCall.Name, matchedRule.ID)

					approvalReq := &approval.ApprovalRequest{
						Type:        approval.ApprovalTypeToolCall,
						Priority:    approval.PriorityMedium,
						AgentID:     currentAgent.ID,
						SessionID:   execCtx.SessionID,
						Description: fmt.Sprintf("Agent %s requests to execute tool: %s", currentAgent.Name, toolCall.Name),
						Details: map[string]interface{}{
							"tool_name":  toolCall.Name,
							"arguments":  string(toolCall.Arguments),
							"risk_level": matchedRule.RiskThreshold,
						},
						RiskLevel:      matchedRule.RiskThreshold,
						RiskReason:     fmt.Sprintf("Tool %s requires approval (risk: %s)", toolCall.Name, matchedRule.RiskThreshold),
						TimeoutSeconds: matchedRule.TimeoutSeconds,
						AutoApprove:    false,
					}

					createdReq, err := e.approvalManager.CreateRequest(timeoutCtx, approvalReq)
					if err != nil {
						fmt.Printf("[AgentEngine] Failed to create approval request: %v\n", err)
						results[idx] = toolExecResult{
							ToolCall: toolCall,
							Result:   fmt.Sprintf("Approval request failed: %v", err),
							Err:      err,
							Duration: time.Since(toolStart).Milliseconds(),
							Status:   "error",
						}
						return
					}

					fmt.Printf("[AgentEngine] Waiting for approval: requestID=%s\n", createdReq.ID)

					decision, err := e.approvalManager.WaitForApproval(timeoutCtx, createdReq.ID)
					if err != nil {
						fmt.Printf("[AgentEngine] Approval wait failed: %v\n", err)
						results[idx] = toolExecResult{
							ToolCall: toolCall,
							Result:   fmt.Sprintf("Approval timeout/expired: %v", err),
							Err:      err,
							Duration: time.Since(toolStart).Milliseconds(),
							Status:   "error",
						}
						return
					}

					if decision.Decision != approval.StatusApproved {
						fmt.Printf("[AgentEngine] Tool %s rejected: %s\n", toolCall.Name, decision.Reason)
						results[idx] = toolExecResult{
							ToolCall: toolCall,
							Result:   fmt.Sprintf("Tool execution rejected: %s", decision.Reason),
							Err:      fmt.Errorf("tool rejected: %s", decision.Reason),
							Duration: time.Since(toolStart).Milliseconds(),
							Record: AgentExecutionRecord{
								AgentID:     currentAgent.ID,
								AgentName:   currentAgent.Name,
								Thought:     llmResp.Content,
								Action:      toolCall.Name,
								Arguments:   string(toolCall.Arguments),
								Result:      fmt.Sprintf("Rejected by user: %s", decision.Reason),
								TokensUsed:  llmResp.TotalTokens,
								StartedAt:   stepStart,
								CompletedAt: time.Now(),
								Duration:    time.Since(toolStart).Milliseconds(),
							},
							Status: "rejected",
						}
						return
					}

					// Approved — optionally apply modified params
					fmt.Printf("[AgentEngine] Tool %s approved\n", toolCall.Name)
					if decision.ModifiedParams != nil {
						if modifiedArgs, err := json.Marshal(decision.ModifiedParams); err == nil {
							toolCall.Arguments = modifiedArgs
						}
					}
				}
			}

			// Get tool config for current agent
			var toolCfg *ToolSpecificConfig
			if currentAgent.ToolConfig != nil {
				if cfg, ok := currentAgent.ToolConfig[toolCall.Name]; ok {
					toolCfg = &cfg
				}
			}

			result, err := e.tools.Execute(timeoutCtx, toolCall.Name, toolCall.Arguments, toolCfg)
			toolDuration := time.Since(toolStart).Milliseconds()

			var recordResult string
			var status string
			if err != nil {
				recordResult = fmt.Sprintf("Error: %v", err)
				status = "error"

				// Reflection: analyze error root cause
				if e.errorAnalyzer != nil {
					analysis, analyzeErr := e.errorAnalyzer.Analyze(timeoutCtx, execCtx.SessionID, execCtx.StepCount, err.Error(), "tool_execution")
					if analyzeErr == nil && analysis != nil {
						suggestedFix := ""
						if len(analysis.RecoveryOptions) > 0 {
							for _, opt := range analysis.RecoveryOptions {
								if opt.Recommended {
									suggestedFix = opt.Description
									break
								}
							}
							if suggestedFix == "" {
								suggestedFix = analysis.RecoveryOptions[0].Description
							}
						}
						execCtx.AddMessage("system", fmt.Sprintf("Error analysis: root cause=%s. Suggested fix: %s", analysis.RootCause, suggestedFix))
					}
				}
			} else {
				recordResult = result
				status = "completed"
			}

			// Record tool call status and result
			toolCall.Status = status
			toolCall.Result = result

			results[idx] = toolExecResult{
				ToolCall: toolCall,
				Result:   recordResult,
				Err:      err,
				Duration: toolDuration,
				Record: AgentExecutionRecord{
					AgentID:     currentAgent.ID,
					AgentName:   currentAgent.Name,
					Thought:     llmResp.Content,
					Action:      toolCall.Name,
					Arguments:   string(toolCall.Arguments),
					Result:      recordResult,
					TokensUsed:  llmResp.TotalTokens,
					StartedAt:   stepStart,
					CompletedAt: time.Now(),
					Duration:    toolDuration,
				},
				Status: status,
			}
		}(i, tc)
	}
	wg.Wait()

	return results
}

// processToolResults records all parallel tool results into the execution context.
func processToolResults(execCtx *ExecutionContext, results []toolExecResult) {
	for _, res := range results {
		execCtx.RecordToolCall(res.ToolCall)
		if res.Err != nil && res.Status == "rejected" {
			execCtx.AddToolMessage(res.ToolCall.Name, res.ToolCall.ID, res.Result)
			execCtx.AddAgentRecord(res.Record)
		} else if res.Err != nil {
			execCtx.AddToolMessage(res.ToolCall.Name, res.ToolCall.ID, res.Result)
			execCtx.AddAgentRecord(res.Record)
		} else {
			execCtx.AddToolMessage(res.ToolCall.Name, res.ToolCall.ID, res.ToolCall.Result)
			execCtx.AddAgentRecord(res.Record)
		}
	}
}

// ============================================================
// Execution Loops
// ============================================================

// executeLoop runs the agent execution loop
func (e *Engine) executeLoop(ctx context.Context, execCtx *ExecutionContext, startAgent *Agent, initialMessage string) (*ExecutionResult, error) {
	currentAgent := startAgent
	fmt.Printf("[AgentEngine] Starting executeLoop with agent: %s\n", currentAgent.ID)

	// Register session with intervention manager
	if e.interventionManager != nil {
		e.interventionManager.RegisterSession(execCtx.SessionID, currentAgent.ID)
		defer e.interventionManager.UnregisterSession(execCtx.SessionID)
	}

	for step := 0; step < e.config.MaxSteps; step++ {
		// Record step start
		stepStart := time.Now()
		execCtx.StepCount = step + 1
		fmt.Printf("[AgentEngine] Step %d: Agent=%s\n", step+1, currentAgent.ID)

		// Check for interventions before this step
		if e.interventionManager != nil {
			state, stateErr := e.interventionManager.GetSessionState(execCtx.SessionID)
			if stateErr == nil && state != nil {
				switch state.Status {
				case "paused":
					fmt.Printf("[AgentEngine] Session paused, waiting for intervention...\n")
					event, waitErr := e.interventionManager.WaitForEvent(ctx, execCtx.SessionID)
					if waitErr != nil {
						execCtx.MarkError("intervention wait failed")
						return e.buildResult(execCtx, ""), waitErr
					}
					if event.Type == intervention.InterventionStop {
						execCtx.MarkError("stopped by intervention")
						return e.buildResult(execCtx, ""), fmt.Errorf("session stopped by intervention")
					}
					// Resume continues the loop
				case "stopped":
					execCtx.MarkError("stopped by intervention")
					return e.buildResult(execCtx, ""), fmt.Errorf("session stopped by intervention")
				}
			}
		}

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

			// Reflection: summarize learning on task completion
			if e.reflectionLoop != nil {
				taskDesc := ""
				if v, ok := execCtx.Variables["task"]; ok {
					taskDesc = fmt.Sprintf("%v", v)
				}
				reflectionCtx := reflection.ReflectionContext{
					Task:       taskDesc,
					Goal:       currentAgent.Instructions,
					Success:    true,
					TokenUsage: execCtx.TotalTokens,
				}
				reflectResult, _ := e.reflectionLoop.Reflect(ctx, execCtx.SessionID, reflection.PhaseComplete, reflectionCtx)
				if reflectResult != nil {
					fmt.Printf("[AgentEngine] Final reflection: score=%.2f, lessons=%v\n", reflectResult.Score, reflectResult.LessonsLearned)
				}
			}

			return e.buildResult(execCtx, llmResp.Content), nil
		}

		// Process tool calls — separate handoff from regular tools
		var handoffCalls []ToolCall
		var regularCalls []ToolCall
		for _, tc := range llmResp.ToolCalls {
			if IsHandoffTool(tc.Name) {
				handoffCalls = append(handoffCalls, tc)
			} else {
				regularCalls = append(regularCalls, tc)
			}
		}

		fmt.Printf("[AgentEngine] Parallel execution: %d regular tools, %d handoffs\n", len(regularCalls), len(handoffCalls))

		// Execute regular tool calls in parallel (semaphore-limited, per-tool timeout)
		if len(regularCalls) > 0 {
			results := e.executeParallelTools(ctx, regularCalls, execCtx, currentAgent, llmResp, stepStart)
			processToolResults(execCtx, results)
		}

		// Process handoff calls serially
		for _, tc := range handoffCalls {
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
		// Reflection: evaluate step results after tool execution
		if e.reflectionLoop != nil && len(llmResp.ToolCalls) > 0 {
			taskDesc := ""
			if v, ok := execCtx.Variables["task"]; ok {
				taskDesc = fmt.Sprintf("%v", v)
			}
			reflectionCtx := reflection.ReflectionContext{
				Task:       taskDesc,
				Goal:       currentAgent.Instructions,
				Success:    true,
				TokenUsage: execCtx.TotalTokens,
			}
			reflectResult, err := e.reflectionLoop.Reflect(ctx, execCtx.SessionID, reflection.PhasePostAction, reflectionCtx)
			if err == nil && reflectResult != nil && reflectResult.Score < 0.5 && len(reflectResult.Suggestions) > 0 {
				execCtx.AddMessage("system", fmt.Sprintf("Reflection: %s. Suggestions: %s", reflectResult.Analysis, strings.Join(reflectResult.Suggestions, "; ")))
				fmt.Printf("[AgentEngine] Reflection score: %.2f, suggestions: %v\n", reflectResult.Score, reflectResult.Suggestions)
			}
		}

		// Save context after each step
		if e.store != nil {
			e.store.Save(ctx, execCtx)
		}

		// Log execution entry for intervention tracking
		if e.interventionManager != nil {
			e.interventionManager.LogExecution(execCtx.SessionID, intervention.ExecutionEntry{
				StepNum: step,
				Action:  "step_complete",
				Duration: time.Since(stepStart).Milliseconds(),
				Success: true,
			})
		}

		// Save checkpoint after each step
		if e.checkpointStore != nil {
			cp := &checkpoint.Checkpoint{
				SessionID:    execCtx.SessionID,
				Step:         step,
				AgentID:      currentAgent.ID,
				Messages:     messagesToCheckpoint(execCtx.Messages),
				Variables:    variablesToStringMap(execCtx.Variables),
				ToolResults:  toolResultsToStringMap(execCtx.ToolResults),
				AgentHistory: agentHistoryToCheckpoint(execCtx.AgentHistory),
				TotalTokens:  execCtx.TotalTokens,
				CreatedAt:    time.Now(),
			}
			if err := e.checkpointStore.Save(ctx, cp); err != nil {
				fmt.Printf("[AgentEngine] Failed to save checkpoint: %v\n", err)
			}
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

// ResumeFromCheckpoint loads a checkpoint, rebuilds the ExecutionContext, and
// resumes the execution loop from checkpoint.Step+1.
func (e *Engine) ResumeFromCheckpoint(ctx context.Context, checkpointID string) (*ExecutionResult, error) {
	if e.checkpointStore == nil {
		return nil, fmt.Errorf("checkpoint store not configured")
	}

	// Load checkpoint
	cp, err := e.checkpointStore.Get(ctx, checkpointID)
	if err != nil {
		return nil, fmt.Errorf("load checkpoint: %w", err)
	}

	// Rebuild ExecutionContext from checkpoint
	execCtx := &ExecutionContext{
		ID:           generateContextID(),
		SessionID:    cp.SessionID,
		Variables:    stringMapToAnyMap(cp.Variables),
		Messages:     checkpointMessagesToAgent(cp.Messages),
		AgentHistory: checkpointAgentHistoryToAgent(cp.AgentHistory),
		ToolResults:  stringMapToToolResults(cp.ToolResults),
		CurrentAgent: cp.AgentID,
		TotalTokens:  cp.TotalTokens,
		Status:       AgentStatusRunning,
		StartedAt:    cp.CreatedAt,
		StepCount:    cp.Step + 1,
	}
	execCtx.MarkRunning()

	// Get current agent from checkpoint
	currentAgent := e.registry.Get(cp.AgentID)
	if currentAgent == nil {
		return nil, fmt.Errorf("agent %s not found", cp.AgentID)
	}

	// Save restored context
	if e.store != nil {
		if err := e.store.Save(ctx, execCtx); err != nil {
			return nil, fmt.Errorf("save restored context: %w", err)
		}
	}

	// Resume execution from the step after the checkpoint.
	resumeStep := cp.Step + 1
	result, loopErr := e.resumeLoop(ctx, execCtx, currentAgent, resumeStep)

	// Save final context
	if e.store != nil {
		e.store.Save(ctx, execCtx)
	}

	return result, loopErr
}

// resumeLoop is like executeLoop but starts from a given step index instead of 0.
func (e *Engine) resumeLoop(ctx context.Context, execCtx *ExecutionContext, startAgent *Agent, startStep int) (*ExecutionResult, error) {
	currentAgent := startAgent
	fmt.Printf("[AgentEngine] Resuming executeLoop from step %d with agent: %s\n", startStep+1, currentAgent.ID)

	// Register session with intervention manager
	if e.interventionManager != nil {
		e.interventionManager.RegisterSession(execCtx.SessionID, currentAgent.ID)
		defer e.interventionManager.UnregisterSession(execCtx.SessionID)
	}

	for step := startStep; step < e.config.MaxSteps; step++ {
		stepStart := time.Now()
		execCtx.StepCount = step + 1
		fmt.Printf("[AgentEngine] Step %d: Agent=%s\n", step+1, currentAgent.ID)

		messages := e.buildAgentMessages(currentAgent, execCtx)
		fmt.Printf("[AgentEngine] Built %d messages\n", len(messages))

		tools, err := e.buildAgentTools(ctx, currentAgent)
		if err != nil {
			fmt.Printf("[AgentEngine] ERROR build tools: %v\n", err)
			return nil, fmt.Errorf("build tools: %w", err)
		}
		fmt.Printf("[AgentEngine] Built %d tools\n", len(tools))

		llmReq := &LLMRequest{
			Messages:    messages,
			Tools:       tools,
			Model:       currentAgent.Model,
			MaxTokens:   currentAgent.MaxTokens,
			Temperature: currentAgent.Temperature,
		}

		fmt.Printf("[AgentEngine] Calling LLM...\n")
		llmResp, err := e.llmClient.Chat(ctx, llmReq)
		if err != nil {
			fmt.Printf("[AgentEngine] ERROR LLM call: %v\n", err)
			execCtx.MarkError(err.Error())
			return e.buildResult(execCtx, ""), fmt.Errorf("llm call: %w", err)
		}
		fmt.Printf("[AgentEngine] LLM response: tokens=%d, tool_calls=%d\n", llmResp.TotalTokens, len(llmResp.ToolCalls))

		execCtx.AddTokens(llmResp.TotalTokens)
		execCtx.AddCost(llmResp.Cost)

		if len(llmResp.ToolCalls) == 0 {
			fmt.Printf("[AgentEngine] DONE! ContentLen=%d, Preview: %.200s\n", len(llmResp.Content), llmResp.Content)
			execCtx.AddMessage("assistant", llmResp.Content)
			execCtx.MarkCompleted()
			return e.buildResult(execCtx, llmResp.Content), nil
		}

		// Process tool calls — separate handoff from regular tools
		var handoffCalls []ToolCall
		var regularCalls []ToolCall
		for _, tc := range llmResp.ToolCalls {
			if IsHandoffTool(tc.Name) {
				handoffCalls = append(handoffCalls, tc)
			} else {
				regularCalls = append(regularCalls, tc)
			}
		}

		fmt.Printf("[AgentEngine] Parallel execution: %d regular tools, %d handoffs\n", len(regularCalls), len(handoffCalls))

		// Execute regular tool calls in parallel (semaphore-limited, per-tool timeout)
		if len(regularCalls) > 0 {
			results := e.executeParallelTools(ctx, regularCalls, execCtx, currentAgent, llmResp, stepStart)
			processToolResults(execCtx, results)
		}

		// Process handoff calls serially
		for _, tc := range handoffCalls {
			targetID := ParseHandoffTarget(tc.Name)
			if err := ValidateHandoff(currentAgent, targetID, e.registry); err != nil {
				execCtx.MarkError(err.Error())
				return e.buildResult(execCtx, ""), err
			}

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

			currentAgent = e.registry.Get(targetID)
			execCtx.SetCurrentAgent(targetID)

			if e.store != nil {
				e.store.Save(ctx, execCtx)
			}
			break
		}

		// Save context after each step
		if e.store != nil {
			e.store.Save(ctx, execCtx)
		}

		// Save checkpoint after each step
		if e.checkpointStore != nil {
			cp := &checkpoint.Checkpoint{
				SessionID:    execCtx.SessionID,
				Step:         step,
				AgentID:      currentAgent.ID,
				Messages:     messagesToCheckpoint(execCtx.Messages),
				Variables:    variablesToStringMap(execCtx.Variables),
				ToolResults:  toolResultsToStringMap(execCtx.ToolResults),
				AgentHistory: agentHistoryToCheckpoint(execCtx.AgentHistory),
				TotalTokens:  execCtx.TotalTokens,
				CreatedAt:    time.Now(),
			}
			if err := e.checkpointStore.Save(ctx, cp); err != nil {
				fmt.Printf("[AgentEngine] Failed to save checkpoint: %v\n", err)
			}
		}
	}

	execCtx.MarkError("maximum steps reached")
	return e.buildResult(execCtx, "Maximum execution steps reached. Please try a simpler request."), ErrMaxStepsReached
}

// ============================================================
// Checkpoint conversion helpers
// ============================================================

// variablesToStringMap converts map[string]any to map[string]string for checkpoint storage.
func variablesToStringMap(m map[string]any) map[string]string {
	if m == nil {
		return nil
	}
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result
}

// stringMapToAnyMap converts map[string]string back to map[string]any.
func stringMapToAnyMap(m map[string]string) map[string]any {
	if m == nil {
		return nil
	}
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// toolResultsToStringMap converts map[string]ToolCall to map[string]string for checkpoint storage.
func toolResultsToStringMap(m map[string]ToolCall) map[string]string {
	if m == nil {
		return nil
	}
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = v.Result
	}
	return result
}

// stringMapToToolResults converts map[string]string back to map[string]ToolCall.
func stringMapToToolResults(m map[string]string) map[string]ToolCall {
	if m == nil {
		return nil
	}
	result := make(map[string]ToolCall, len(m))
	for k, v := range m {
		result[k] = ToolCall{ID: k, Result: v, Status: "completed"}
	}
	return result
}

// messagesToCheckpoint converts []Message to []checkpoint.Message.
func messagesToCheckpoint(msgs []Message) []checkpoint.Message {
	if msgs == nil {
		return nil
	}
	result := make([]checkpoint.Message, len(msgs))
	for i, m := range msgs {
		result[i] = checkpoint.Message{
			Role:    m.Role,
			Content: m.Content,
			Name:    m.Name,
			ToolID:  m.ToolID,
		}
	}
	return result
}

// checkpointMessagesToAgent converts []checkpoint.Message back to []Message.
func checkpointMessagesToAgent(msgs []checkpoint.Message) []Message {
	if msgs == nil {
		return nil
	}
	result := make([]Message, len(msgs))
	for i, m := range msgs {
		result[i] = Message{
			Role:    m.Role,
			Content: m.Content,
			Name:    m.Name,
			ToolID:  m.ToolID,
		}
	}
	return result
}

// agentHistoryToCheckpoint converts []AgentExecutionRecord to []checkpoint.AgentExecutionRecord.
func agentHistoryToCheckpoint(records []AgentExecutionRecord) []checkpoint.AgentExecutionRecord {
	if records == nil {
		return nil
	}
	result := make([]checkpoint.AgentExecutionRecord, len(records))
	for i, r := range records {
		result[i] = checkpoint.AgentExecutionRecord{
			AgentID:     r.AgentID,
			AgentName:   r.AgentName,
			Thought:     r.Thought,
			Action:      r.Action,
			Arguments:   r.Arguments,
			Result:      r.Result,
			HandoffTo:   r.HandoffTo,
			TokensUsed:  r.TokensUsed,
			StartedAt:   r.StartedAt,
			CompletedAt: r.CompletedAt,
			Duration:    r.Duration,
		}
	}
	return result
}

// checkpointAgentHistoryToAgent converts []checkpoint.AgentExecutionRecord back to []AgentExecutionRecord.
func checkpointAgentHistoryToAgent(records []checkpoint.AgentExecutionRecord) []AgentExecutionRecord {
	if records == nil {
		return nil
	}
	result := make([]AgentExecutionRecord, len(records))
	for i, r := range records {
		result[i] = AgentExecutionRecord{
			AgentID:     r.AgentID,
			AgentName:   r.AgentName,
			Thought:     r.Thought,
			Action:      r.Action,
			Arguments:   r.Arguments,
			Result:      r.Result,
			HandoffTo:   r.HandoffTo,
			TokensUsed:  r.TokensUsed,
			StartedAt:   r.StartedAt,
			CompletedAt: r.CompletedAt,
			Duration:    r.Duration,
		}
	}
	return result
}
