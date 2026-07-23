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
	Messages    []Message        `json:"messages"`
	Tools       []map[string]any `json:"tools,omitempty"`
	Model       string           `json:"model,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
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
	memoryClient        MemoryClient                 // ★ 记忆客户端:每步 recall/write,(nil 时降级为无记忆)
	skillStore          SkillStore                   // ★ 技能库:解析 agent.Skills,注入 name+desc,按需 load_skill (nil 时降级为无技能)
	verifier            Verifier                     // gates completion on success criteria (nil = LLM says done)
	strategyAdjuster    *reflection.StrategyAdjuster // closes the reflection loop (was dead code)
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

	// computer_use drives the whole desktop (mouse/keyboard/apps) - high risk, so
	// the agent must get human approval before invoking it. Fine-grained per-action
	// approval (e.g. launch_app) is handled inside pkg/computeruse via ActionGate.
	e.ruleEngine.AddRule(&approval.ApprovalRule{
		ID:             "rule-computer-use",
		Type:           approval.ApprovalTypeToolCall,
		ToolName:       "computer_use",
		RiskThreshold:  "high",
		AutoApprove:    false,
		TimeoutSeconds: 300,
	})

	// Initialize strategy adjuster with default rules. This closes the
	// reflection loop: step reflection now feeds StrategyAdjuster (previously
	// dead code) and the adjustment is injected as a system message.
	e.strategyAdjuster = reflection.NewStrategyAdjuster()
	e.strategyAdjuster.AddRule(reflection.AdjustmentRule{
		Trigger:         "low_score",
		CurrentStrategy: "current execution approach",
		NewStrategy:     "slow down, re-evaluate, and break the task into smaller steps",
		Priority:        1,
	})
	e.strategyAdjuster.AddRule(reflection.AdjustmentRule{
		Trigger:         "high_errors",
		CurrentStrategy: "current tool selection",
		NewStrategy:     "switch tools or take a different approach to reduce errors",
		Priority:        2,
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

// SetVerifier wires a verifier that gates task completion on success criteria.
func (e *Engine) SetVerifier(v Verifier) { e.verifier = v }

// GetVerifier returns the configured verifier (may be nil).
func (e *Engine) GetVerifier() Verifier { return e.verifier }

// SetMemoryClient sets the memory client used for recall/write in the execution loop.
// When nil, the engine runs without memory (graceful degradation). Injected by
// agent-service, which adapts this interface to the memory-service gRPC client.
func (e *Engine) SetMemoryClient(mc MemoryClient) {
	e.memoryClient = mc
}

// GetMemoryClient returns the memory client for external access.
func (e *Engine) GetMemoryClient() MemoryClient {
	return e.memoryClient
}

// SetSkillStore wires a skill store used for progressive-disclosure skill
// loading. The engine injects each mounted skill's Name+Description into the
// system prompt (cheap, always-on) and serves full Instructions on demand via
// the built-in load_skill tool. When nil, the engine runs without skills.
// Injected by agent-service.
func (e *Engine) SetSkillStore(ss SkillStore) {
	e.skillStore = ss
}

// GetSkillStore returns the skill store for external access.
func (e *Engine) GetSkillStore() SkillStore {
	return e.skillStore
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
	SessionID            string         `json:"session_id"`
	TenantID             string         `json:"tenant_id,omitempty"`
	UserID               string         `json:"user_id,omitempty"`
	Message              string         `json:"message"`
	EntryAgent           string         `json:"entry_agent,omitempty"`
	ContextVars          map[string]any `json:"context_vars,omitempty"`
	SystemPromptOverride string         `json:"system_prompt_override,omitempty"` // Rendered prompt from Prompt Management
	Goal                 string         `json:"goal,omitempty"`
	SuccessCriteria      string         `json:"success_criteria,omitempty"` // checkable completion condition; verifier gates done on this
}

// ExecutionResult represents an execution result
type ExecutionResult struct {
	ContextID    string                 `json:"context_id"`
	SessionID    string                 `json:"session_id"`
	Response     string                 `json:"response"`
	AgentHistory []AgentExecutionRecord `json:"agent_history"`
	TotalTokens  int                    `json:"total_tokens"`
	TotalCost    float64                `json:"total_cost"`
	Status       AgentStatus            `json:"status"`
	Error        string                 `json:"error,omitempty"`
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

	// Set system prompt override from Prompt Management
	if req.SystemPromptOverride != "" {
		execCtx.SystemPromptOverride = req.SystemPromptOverride
	}

	// Copy goal + success criteria for the verifier and planner. Fall back to
	// context_vars (keys: "goal", "success_criteria") so callers can set them
	// without a proto change.
	execCtx.Goal = req.Goal
	execCtx.SuccessCriteria = req.SuccessCriteria
	if execCtx.Goal == "" {
		if v, ok := execCtx.Variables["goal"]; ok {
			if s, ok := v.(string); ok && s != "" {
				execCtx.Goal = s
			}
		}
	}
	if execCtx.SuccessCriteria == "" {
		if v, ok := execCtx.Variables["success_criteria"]; ok {
			if s, ok := v.(string); ok && s != "" {
				execCtx.SuccessCriteria = s
			}
		}
	}

	// Planner step (option B): decompose the task into a todo list when a goal
	// is set. The plan is injected into the system prompt each step. Best-effort;
	// a planner failure does not block execution.
	if execCtx.Goal != "" {
		if plan, perr := e.planTask(ctx, execCtx); perr == nil && plan != nil {
			execCtx.Plan = plan
		} else if perr != nil {
			fmt.Printf("[AgentEngine] planner step skipped: %v\n", perr)
		}
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

			// Bind fine-grained browser primitives (navigate/click/type/extract/
			// scroll/wait) to the current session so they share one browser page
			// across calls. injectSessionID returns a copy; the agent's ToolConfig
			// is never mutated.
			if isFineGrainedBrowserTool(toolCall.Name) {
				toolCfg = injectSessionID(toolCfg, execCtx.SessionID)
			}

			// Built-in load_skill is served locally from the skill store - it does
			// not route to MCP and needs no tool config. It is read-only (returns
			// a skill's full Instructions), so it also bypasses approval naturally
			// (no rule matches "load_skill").
			var result string
			var err error
			if toolCall.Name == "load_skill" {
				result, err = e.executeLoadSkill(timeoutCtx, currentAgent, toolCall.Arguments)
			} else {
				result, err = e.tools.Execute(timeoutCtx, toolCall.Name, toolCall.Arguments, toolCfg)
			}
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
		messages := e.buildAgentMessages(ctx, currentAgent, execCtx)
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

			// Verify success criteria before declaring done. If a verifier is
			// configured and the criteria are not met, send the evidence back to
			// the agent and let it re-plan instead of completing.
			if e.verifier != nil && execCtx.SuccessCriteria != "" {
				passed, evidence, verr := e.verifier.Verify(ctx, execCtx)
				if verr == nil && !passed {
					fmt.Printf("[AgentEngine] Verification failed: %s\n", evidence)
					execCtx.AddMessage("system", fmt.Sprintf(
						"Task not complete. Verification: %s. Success criteria: %s. Continue working; do not stop until the criteria are met.",
						evidence, execCtx.SuccessCriteria))
					continue
				}
				if verr != nil {
					fmt.Printf("[AgentEngine] Verifier error (fail open): %v\n", verr)
				}
			}

			execCtx.MarkCompleted()

			// Persist the conclusion: write the final answer to memory and
			// checkpoint the completed state. The done-path used to return
			// early and skip both, so a direct answer was never durable.
			e.writeStepMemory(ctx, execCtx, currentAgent, llmResp, nil)
			if e.checkpointStore != nil {
				cp := buildCheckpoint(execCtx, currentAgent.ID, step)
				if err := e.checkpointStore.Save(ctx, cp); err != nil {
					fmt.Printf("[AgentEngine] Failed to save checkpoint: %v\n", err)
				}
			}

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
					// Write lessons to memory for cross-session reuse (closes the reflection loop).
					if e.memoryClient != nil && len(reflectResult.LessonsLearned) > 0 {
						lessons := strings.Join(reflectResult.LessonsLearned, "; ")
						content := fmt.Sprintf("Lessons from task [%s]: %s", taskDesc, lessons)
						if err := e.memoryClient.Write(ctx, execCtx.SessionID, execCtx.TenantID, currentAgent.ID, content, 0.7); err != nil {
							fmt.Printf("[AgentEngine] lessons memory write failed: %v\n", err)
						}
					}
					// Feed suggestions back into working memory so the next session sees them.
					if len(reflectResult.Suggestions) > 0 {
						execCtx.AddMessage("system", fmt.Sprintf(
							"Task reflection - strengths: %s; weaknesses: %s; next time: %s",
							strings.Join(reflectResult.Strengths, ", "),
							strings.Join(reflectResult.Weaknesses, ", "),
							strings.Join(reflectResult.Suggestions, "; ")))
					}
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
			// Write step outcome to memory (episodic). Degrades gracefully if no memory client.
			e.writeStepMemory(ctx, execCtx, currentAgent, llmResp, results)
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
			// Close the reflection loop: feed the result to the strategy adjuster
			// (previously dead code). The adjustment goes into working memory so
			// the next step can act on it.
			if err == nil && reflectResult != nil && e.strategyAdjuster != nil {
				if adj, aerr := e.strategyAdjuster.Evaluate(ctx, execCtx.SessionID, reflectResult); aerr == nil && adj != nil {
					execCtx.AddMessage("system", fmt.Sprintf(
						"Strategy adjustment: %s. Reason: %s. New strategy: %s",
						adj.Trigger, adj.Reason, adj.NewStrategy))
					_ = e.strategyAdjuster.Apply(adj.ID, "applied")
					fmt.Printf("[AgentEngine] Strategy adjustment: %s -> %s\n", adj.Trigger, adj.NewStrategy)
				}
			}
		}

		// Save context after each step
		if e.store != nil {
			e.store.Save(ctx, execCtx)
		}

		// Log execution entry for intervention tracking
		if e.interventionManager != nil {
			e.interventionManager.LogExecution(execCtx.SessionID, intervention.ExecutionEntry{
				StepNum:  step,
				Action:   "step_complete",
				Duration: time.Since(stepStart).Milliseconds(),
				Success:  true,
			})
		}

		// Save checkpoint after each step
		if e.checkpointStore != nil {
			cp := buildCheckpoint(execCtx, currentAgent.ID, step)
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
func (e *Engine) buildAgentMessages(ctx context.Context, agent *Agent, execCtx *ExecutionContext) []Message {
	messages := make([]Message, 0)

	// Add system prompt — prefer override from Prompt Management, fallback to agent.Instructions
	systemPrompt := agent.Instructions
	if execCtx.SystemPromptOverride != "" {
		systemPrompt = execCtx.SystemPromptOverride
	}

	// Inject explicit goal, plan, and success criteria so the agent knows what
	// "done" means. The verifier checks SuccessCriteria before completion.
	if execCtx.Goal != "" {
		systemPrompt += fmt.Sprintf("\n\nGoal: %s", execCtx.Goal)
	}
	if execCtx.Plan != nil && len(execCtx.Plan.Items) > 0 {
		systemPrompt += "\n\nTask plan (work through these steps):\n"
		for _, item := range execCtx.Plan.Items {
			systemPrompt += fmt.Sprintf("- [%s] %s\n", item.Status, item.Description)
		}
	}
	if execCtx.SuccessCriteria != "" {
		systemPrompt += fmt.Sprintf("\nSuccess criteria (you are NOT done until these are met): %s", execCtx.SuccessCriteria)
	}

	// Add context variables to system prompt
	if len(execCtx.Variables) > 0 {
		varsJSON, _ := json.Marshal(execCtx.Variables)
		systemPrompt += fmt.Sprintf("\n\nCurrent context variables:\n%s", string(varsJSON))
	}

	// Add agent history summary
	if len(execCtx.AgentHistory) > 0 {
		systemPrompt += "\n\nPrevious agent actions:\n"
		for _, r := range execCtx.AgentHistory {
			// Cap each result so a single huge tool payload cannot blow up the
			// system prompt. 1000 runes is enough to convey outcome without
			// dominating the context budget.
			systemPrompt += fmt.Sprintf("- %s (%s): %s\n", r.AgentName, r.Action, truncateRunes(r.Result, 1000))
		}
	}

	// Recall relevant memories from the memory service and inject into the system prompt.
	// This is what makes the agent "remember": every decision step pulls related past
	// experience. Degrades gracefully - on error or nil client, no memory is added.
	if e.memoryClient != nil {
		if query := e.extractRecallQuery(execCtx); query != "" {
			if recalled, err := e.memoryClient.Recall(ctx, execCtx.SessionID, execCtx.TenantID, query); err == nil && recalled != "" {
				systemPrompt += fmt.Sprintf("\n\nRelevant memories from past experience:\n%s", recalled)
			}
		}
	}

	// Inject available skills (progressive disclosure): only Name + Description
	// are added to the prompt - cheap and always-on. The agent loads the full
	// Instructions on demand via the load_skill built-in tool. Degrades
	// gracefully: nil store, no mounted skills, or unresolved IDs = nothing added.
	if e.skillStore != nil && len(agent.Skills) > 0 {
		if skills, serr := e.skillStore.GetSkillsByIDs(ctx, agent.Skills); serr == nil && len(skills) > 0 {
			systemPrompt += "\n\nAvailable skills (call load_skill with the skill name to load full instructions before using one):"
			for _, sk := range skills {
				if sk.Status != SkillStatusActive {
					continue
				}
				systemPrompt += fmt.Sprintf("\n- %s: %s", sk.Name, sk.Description)
			}
			systemPrompt += "\nLoad a skill's instructions with load_skill only when you need to use it."
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

// truncateRunes caps s to at most max runes, appending a marker when cut.
// Rune-based (not byte-based) so CJK text is sized fairly. Used to keep tool
// results and other volatile content from dominating the system prompt.
func truncateRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…[truncated]"
}

// planTask asks the LLM to decompose the task goal into a short todo list.
// Best-effort: on any failure returns nil so execution continues without a plan.
func (e *Engine) planTask(ctx context.Context, execCtx *ExecutionContext) (*TaskPlan, error) {
	if e.llmClient == nil {
		return nil, nil
	}
	prompt := fmt.Sprintf("Break the following task into 3-7 concrete, ordered steps. Reply with JSON only: an array of step strings. Task: %v. Goal: %s", execCtx.Variables["task"], execCtx.Goal)
	resp, err := e.llmClient.Chat(ctx, &LLMRequest{
		Messages: []Message{
			{Role: "system", Content: "You are a task planner. Reply with a JSON array of step strings only."},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   512,
		Temperature: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("planner llm call: %w", err)
	}
	var steps []string
	if err := json.Unmarshal([]byte(resp.Content), &steps); err != nil {
		return nil, nil // unparseable plan -> proceed without one
	}
	plan := &TaskPlan{UpdatedAt: time.Now()}
	now := time.Now()
	for i, s := range steps {
		if s = strings.TrimSpace(s); s == "" {
			continue
		}
		plan.Items = append(plan.Items, TaskItem{
			ID:          fmt.Sprintf("p%d", i+1),
			Description: s,
			Status:      "pending",
			AddedAt:     now,
		})
	}
	return plan, nil
}

// extractRecallQuery builds a query string for memory recall from the current context.
// Prefers the "task" context variable, falls back to the most recent user message.
func (e *Engine) extractRecallQuery(execCtx *ExecutionContext) string {
	if v, ok := execCtx.Variables["task"]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	for i := len(execCtx.Messages) - 1; i >= 0; i-- {
		if execCtx.Messages[i].Role == "user" {
			return execCtx.Messages[i].Content
		}
	}
	return ""
}

// writeStepMemory writes the outcome of a step (thought + actions + results) to episodic
// memory. Called after tool execution. Degrades gracefully: nil client or error does not
// break the loop. Errors are scored more memorable than successes (lessons learned).
func (e *Engine) writeStepMemory(ctx context.Context, execCtx *ExecutionContext, agent *Agent, llmResp *LLMResponse, results []toolExecResult) {
	if e.memoryClient == nil {
		return
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Agent %s thought: %s\n", agent.Name, llmResp.Content))
	hasError := false
	for _, r := range results {
		status := r.Status
		if status == "" {
			status = "unknown"
		}
		sb.WriteString(fmt.Sprintf("action: %s | args: %s | result: %s | status: %s\n",
			r.ToolCall.Name, string(r.ToolCall.Arguments), r.Result, status))
		if r.Err != nil {
			hasError = true
		}
	}
	importance := 0.5
	if hasError {
		importance = 0.8
	}
	if err := e.memoryClient.Write(ctx, execCtx.SessionID, execCtx.TenantID, agent.ID, sb.String(), importance); err != nil {
		fmt.Printf("[AgentEngine] memory write failed: %v\n", err)
	}
}

// buildAgentTools builds tools available to an agent
func (e *Engine) buildAgentTools(ctx context.Context, agent *Agent) ([]map[string]any, error) {
	tools := make([]map[string]any, 0)
	seen := make(map[string]bool) // dedup tool names across agent + skill sources

	// Get all available tools
	allTools, err := e.tools.ListTools(ctx)
	if err == nil {
		// Add agent's regular tools
		for _, toolName := range agent.Tools {
			if def, ok := allTools[toolName]; ok && !seen[toolName] {
				tools = append(tools, def.(map[string]any))
				seen[toolName] = true
			}
		}

		// Grant tools declared by mounted active skills (dynamic tool gating).
		// A skill may list tools it expects to use; mounting the skill unlocks
		// those tools for the agent so the skill's instructions can actually be
		// carried out. Only tools that already exist in the registry are
		// granted - a skill cannot invent tools, only unlock existing ones.
		// Draft skills are skipped (consistent with prompt injection and
		// load_skill). Degrades gracefully: nil store, no mounted skills, or
		// unresolved IDs = nothing added.
		if e.skillStore != nil && len(agent.Skills) > 0 {
			if skills, serr := e.skillStore.GetSkillsByIDs(ctx, agent.Skills); serr == nil {
				for _, sk := range skills {
					if sk.Status != SkillStatusActive {
						continue
					}
					for _, toolName := range sk.Tools {
						if seen[toolName] {
							continue
						}
						if def, ok := allTools[toolName]; ok {
							tools = append(tools, def.(map[string]any))
							seen[toolName] = true
						}
					}
				}
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

	// Add the load_skill built-in tool when the agent has mounted skills.
	// This is the progressive-disclosure trigger: the agent calls it to fetch a
	// skill's full Instructions on demand. Interception (not MCP) happens in
	// executeToolCalls.
	if e.skillStore != nil && len(agent.Skills) > 0 {
		tools = append(tools, loadSkillToolDefinition())
	}

	return tools, nil
}

// loadSkillToolDefinition returns the tool definition for the built-in
// load_skill tool. It is added to an agent's tool set only when the agent has
// mounted skills, and is served locally (never routed to MCP).
func loadSkillToolDefinition() map[string]any {
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        "load_skill",
			"description": "Load the full instructions for a mounted skill by name. Call this before using a skill listed in 'Available skills'.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "The name of the skill to load (as listed in Available skills).",
					},
				},
				"required": []string{"name"},
			},
		},
	}
}

// executeLoadSkill serves the load_skill built-in tool. It resolves the skill
// by name (or ID) within the agent's mounted skills and returns the full
// Instructions so the agent can follow them. This is the on-demand half of
// progressive disclosure: the cheap Name+Description is always in the prompt,
// the expensive Instructions are loaded only when used.
func (e *Engine) executeLoadSkill(ctx context.Context, agent *Agent, args json.RawMessage) (string, error) {
	if e.skillStore == nil {
		return "", fmt.Errorf("skill store not available")
	}
	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse load_skill arguments: %w", err)
	}
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return "", fmt.Errorf("load_skill requires a 'name' argument")
	}
	// Resolve within the agent's mounted skills only - an agent cannot load a
	// skill it has not mounted. This is the security boundary for skill access.
	skills, err := e.skillStore.GetSkillsByIDs(ctx, agent.Skills)
	if err != nil {
		return "", fmt.Errorf("resolve skills: %w", err)
	}
	for _, sk := range skills {
		if sk.Status != SkillStatusActive {
			continue
		}
		if sk.Name == name || sk.ID == name {
			return fmt.Sprintf("Skill: %s\n\n%s", sk.Name, sk.Instructions), nil
		}
	}
	return "", fmt.Errorf("skill %q is not available (not mounted on this agent or not active)", name)
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

	// Restore goal + success criteria from the checkpoint so the verifier
	// gates completion on resume, not just on the initial execution path.
	// Before this, cp.Goal/cp.SuccessCriteria were empty on resume, so the
	// verifier skipped and only reflection ran.
	execCtx.Goal = cp.Goal
	execCtx.SuccessCriteria = cp.SuccessCriteria

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

		messages := e.buildAgentMessages(ctx, currentAgent, execCtx)
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

			// Verify success criteria before declaring done (parity with executeLoop).
			// If the criteria are not met, send the evidence back and re-plan.
			if e.verifier != nil && execCtx.SuccessCriteria != "" {
				passed, evidence, verr := e.verifier.Verify(ctx, execCtx)
				if verr == nil && !passed {
					fmt.Printf("[AgentEngine] Verification failed: %s\n", evidence)
					execCtx.AddMessage("system", fmt.Sprintf(
						"Task not complete. Verification: %s. Success criteria: %s. Continue working; do not stop until the criteria are met.",
						evidence, execCtx.SuccessCriteria))
					continue
				}
				if verr != nil {
					fmt.Printf("[AgentEngine] Verifier error (fail open): %v\n", verr)
				}
			}

			execCtx.MarkCompleted()

			// Persist the conclusion: write the final answer to memory and
			// checkpoint the completed state, mirroring executeLoop.
			e.writeStepMemory(ctx, execCtx, currentAgent, llmResp, nil)
			if e.checkpointStore != nil {
				cp := buildCheckpoint(execCtx, currentAgent.ID, step)
				if err := e.checkpointStore.Save(ctx, cp); err != nil {
					fmt.Printf("[AgentEngine] Failed to save checkpoint: %v\n", err)
				}
			}

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
					if e.memoryClient != nil && len(reflectResult.LessonsLearned) > 0 {
						lessons := strings.Join(reflectResult.LessonsLearned, "; ")
						content := fmt.Sprintf("Lessons from task [%s]: %s", taskDesc, lessons)
						if err := e.memoryClient.Write(ctx, execCtx.SessionID, execCtx.TenantID, currentAgent.ID, content, 0.7); err != nil {
							fmt.Printf("[AgentEngine] lessons memory write failed: %v\n", err)
						}
					}
					if len(reflectResult.Suggestions) > 0 {
						execCtx.AddMessage("system", fmt.Sprintf(
							"Task reflection - strengths: %s; weaknesses: %s; next time: %s",
							strings.Join(reflectResult.Strengths, ", "),
							strings.Join(reflectResult.Weaknesses, ", "),
							strings.Join(reflectResult.Suggestions, "; ")))
					}
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
			// Write step outcome to memory (episodic). Degrades gracefully if no memory client.
			e.writeStepMemory(ctx, execCtx, currentAgent, llmResp, results)
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
			cp := buildCheckpoint(execCtx, currentAgent.ID, step)
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

// buildCheckpoint captures the current execution state as a Checkpoint snapshot.
// Centralized so executeLoop and resumeLoop persist identical fields -
// including Goal/SuccessCriteria, which the verifier gates completion on -
// and the two loops cannot drift. (A prior gap left resume checkpoints
// without criteria, so the verifier did not gate on resume.)
func buildCheckpoint(execCtx *ExecutionContext, agentID string, step int) *checkpoint.Checkpoint {
	return &checkpoint.Checkpoint{
		SessionID:       execCtx.SessionID,
		Step:            step,
		AgentID:         agentID,
		Messages:        messagesToCheckpoint(execCtx.Messages),
		Variables:       variablesToStringMap(execCtx.Variables),
		ToolResults:     toolResultsToStringMap(execCtx.ToolResults),
		AgentHistory:    agentHistoryToCheckpoint(execCtx.AgentHistory),
		TotalTokens:     execCtx.TotalTokens,
		Goal:            execCtx.Goal,
		SuccessCriteria: execCtx.SuccessCriteria,
		CreatedAt:       time.Now(),
	}
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
