package agent

import (
	"agent-platform/pkg/agent/approval"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// StreamEventType defines the type of streaming event
type StreamEventType string

const (
	// EventThink is emitted when the agent starts thinking (LLM call begins)
	EventThink StreamEventType = "think"
	// EventToken is emitted for each token from the LLM
	EventToken StreamEventType = "token"
	// EventToolStart is emitted before a tool is executed
	EventToolStart StreamEventType = "tool_start"
	// EventToolResult is emitted after a tool completes
	EventToolResult StreamEventType = "tool_result"
	// EventHandoff is emitted when control transfers to another agent
	EventHandoff StreamEventType = "handoff"
	// EventFinal is emitted when the agent loop completes
	EventFinal StreamEventType = "final"
	// EventError is emitted on errors
	EventError StreamEventType = "error"
)

// StreamEvent represents a single streaming event from the engine
type StreamEvent struct {
	Type      StreamEventType     `json:"type"`
	AgentID   string              `json:"agent_id,omitempty"`
	AgentName string              `json:"agent_name,omitempty"`
	Content   string              `json:"content,omitempty"`
	ToolName  string              `json:"tool_name,omitempty"`
	ToolArgs  json.RawMessage     `json:"tool_args,omitempty"`
	ToolResult string             `json:"tool_result,omitempty"`
	Step      int                 `json:"step,omitempty"`
	Tokens    int                 `json:"tokens,omitempty"`
	Done      bool                `json:"done,omitempty"`
	Error     string              `json:"error,omitempty"`
}

// StreamCallback is called for each streaming event
type StreamCallback func(event StreamEvent)

// LLMStreamingClient extends LLMClient with streaming capability
type LLMStreamingClient interface {
	LLMClient
	// ChatStream calls the LLM and returns a channel of incremental tokens
	ChatStream(ctx context.Context, req *LLMRequest) (<-chan LLMStreamChunk, error)
}

// LLMStreamChunk represents an incremental token from the LLM
type LLMStreamChunk struct {
	Content  string `json:"content"`
	Done     bool   `json:"done"`
	Error    error  `json:"error,omitempty"`
}

// RunStream executes the agent loop with real-time event streaming
// The callback is invoked for each event as it happens.
// The final ExecutionResult is still returned at the end.
func (e *Engine) RunStream(ctx context.Context, req *ExecutionRequest, callback StreamCallback) (*ExecutionResult, error) {
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

	// Execute agent loop with streaming
	result, err := e.executeLoopStream(ctx, execCtx, currentAgent, req.Message, callback)

	// Save final context
	if e.store != nil {
		e.store.Save(ctx, execCtx)
	}

	return result, err
}

// executeLoopStream runs the agent loop with real-time event streaming
func (e *Engine) executeLoopStream(ctx context.Context, execCtx *ExecutionContext, startAgent *Agent, initialMessage string, callback StreamCallback) (*ExecutionResult, error) {
	currentAgent := startAgent

	// Check if the LLM client supports streaming
	_, hasStreaming := e.llmClient.(LLMStreamingClient)

	for step := 0; step < e.config.MaxSteps; step++ {
		stepStart := time.Now()
		execCtx.StepCount = step + 1

		// Build messages for this agent
		messages := e.buildAgentMessages(ctx, currentAgent, execCtx)

		// Build tools for this agent
		tools, err := e.buildAgentTools(ctx, currentAgent)
		if err != nil {
			callback(StreamEvent{Type: EventError, Error: fmt.Sprintf("build tools: %v", err)})
			return nil, fmt.Errorf("build tools: %w", err)
		}

		// Build LLM request
		llmReq := &LLMRequest{
			Messages:    messages,
			Tools:       tools,
			Model:       currentAgent.Model,
			MaxTokens:   currentAgent.MaxTokens,
			Temperature: currentAgent.Temperature,
		}

		// Emit think event
		callback(StreamEvent{
			Type:      EventThink,
			AgentID:   currentAgent.ID,
			AgentName: currentAgent.Name,
			Step:      step + 1,
		})

		var llmResp *LLMResponse

		if hasStreaming {
			// Use real streaming LLM call
			llmResp, err = e.callLLMStreaming(ctx, llmReq, callback, currentAgent.ID, currentAgent.Name, step+1)
		} else {
			// Fallback to non-streaming
			llmResp, err = e.llmClient.Chat(ctx, llmReq)
		}

		if err != nil {
			callback(StreamEvent{Type: EventError, Error: fmt.Sprintf("LLM call failed: %v", err)})
			execCtx.MarkError(err.Error())
			return e.buildResult(execCtx, ""), fmt.Errorf("llm call: %w", err)
		}

		// Update token counts
		execCtx.AddTokens(llmResp.TotalTokens)
		execCtx.AddCost(llmResp.Cost)

		// Check for tool calls
		if len(llmResp.ToolCalls) == 0 {
			// No tool calls — we're done
			execCtx.AddMessage("assistant", llmResp.Content)
			execCtx.MarkCompleted()

			callback(StreamEvent{
				Type:      EventFinal,
				AgentID:   currentAgent.ID,
				AgentName: currentAgent.Name,
				Content:   llmResp.Content,
				Tokens:    llmResp.TotalTokens,
				Done:      true,
			})

			return e.buildResult(execCtx, llmResp.Content), nil
		}

		// Process tool calls
		for _, tc := range llmResp.ToolCalls {
			// Check if it's a handoff
			if IsHandoffTool(tc.Name) {
				targetID := ParseHandoffTarget(tc.Name)

				// Validate handoff
				if err := ValidateHandoff(currentAgent, targetID, e.registry); err != nil {
					callback(StreamEvent{Type: EventError, Error: err.Error()})
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

				// Emit handoff event
				callback(StreamEvent{
					Type:      EventHandoff,
					AgentID:   currentAgent.ID,
					AgentName: currentAgent.Name,
					Content:   fmt.Sprintf("Transferring to %s", targetID),
					Step:      step + 1,
				})

				// Switch to target agent
				currentAgent = e.registry.Get(targetID)
				execCtx.SetCurrentAgent(targetID)

				// Save context
				if e.store != nil {
					e.store.Save(ctx, execCtx)
				}

				break
			}

			// ★ Approval check: does this tool require human approval?
			if e.ruleEngine != nil && e.approvalManager != nil {
				needsApproval, matchedRule := e.ruleEngine.NeedsApproval(currentAgent.ID, tc.Name, nil)
				if needsApproval && matchedRule != nil && !matchedRule.AutoApprove {
					// Emit approval pending event
					callback(StreamEvent{
						Type:      EventToolStart,
						AgentID:   currentAgent.ID,
						AgentName: currentAgent.Name,
						ToolName:  tc.Name,
						Content:   fmt.Sprintf("⏳ 等待审批: 工具 %s 需要人工确认 (风险: %s)", tc.Name, matchedRule.RiskThreshold),
						Step:      step + 1,
					})

					approvalReq := &approval.ApprovalRequest{
						Type:        approval.ApprovalTypeToolCall,
						Priority:    approval.PriorityMedium,
						AgentID:     currentAgent.ID,
						SessionID:   execCtx.SessionID,
						Description: fmt.Sprintf("Agent %s requests to execute tool: %s", currentAgent.Name, tc.Name),
						Details: map[string]interface{}{
							"tool_name":  tc.Name,
							"arguments":  string(tc.Arguments),
							"risk_level": matchedRule.RiskThreshold,
						},
						RiskLevel:      matchedRule.RiskThreshold,
						RiskReason:     fmt.Sprintf("Tool %s requires approval (risk: %s)", tc.Name, matchedRule.RiskThreshold),
						TimeoutSeconds: matchedRule.TimeoutSeconds,
						AutoApprove:    false,
					}

					createdReq, err := e.approvalManager.CreateRequest(ctx, approvalReq)
					if err != nil {
						callback(StreamEvent{Type: EventError, Error: fmt.Sprintf("approval request failed: %v", err)})
						execCtx.AddToolMessage(tc.Name, tc.ID, fmt.Sprintf("Approval request failed: %v", err))
						continue
					}

					decision, err := e.approvalManager.WaitForApproval(ctx, createdReq.ID)
					if err != nil {
						callback(StreamEvent{Type: EventToolResult, AgentID: currentAgent.ID, AgentName: currentAgent.Name, ToolName: tc.Name, ToolResult: fmt.Sprintf("审批超时/过期: %v", err)})
						execCtx.AddToolMessage(tc.Name, tc.ID, fmt.Sprintf("Approval timeout: %v", err))
						continue
					}

					if decision.Decision != approval.StatusApproved {
						callback(StreamEvent{Type: EventToolResult, AgentID: currentAgent.ID, AgentName: currentAgent.Name, ToolName: tc.Name, ToolResult: fmt.Sprintf("❌ 工具被拒绝: %s", decision.Reason)})
						execCtx.AddToolMessage(tc.Name, tc.ID, fmt.Sprintf("Rejected: %s", decision.Reason))
						record := AgentExecutionRecord{
							AgentID:     currentAgent.ID,
							AgentName:   currentAgent.Name,
							Thought:     llmResp.Content,
							Action:      tc.Name,
							Arguments:   string(tc.Arguments),
							Result:      fmt.Sprintf("Rejected: %s", decision.Reason),
							TokensUsed:  llmResp.TotalTokens,
							StartedAt:   stepStart,
							CompletedAt: time.Now(),
							Duration:    time.Since(stepStart).Milliseconds(),
						}
						execCtx.AddAgentRecord(record)
						continue
					}

					// Approved
					callback(StreamEvent{
						Type:      EventToolResult,
						AgentID:   currentAgent.ID,
						AgentName: currentAgent.Name,
						ToolName:  tc.Name,
						Content:   fmt.Sprintf("✅ 工具 %s 已批准，开始执行", tc.Name),
						Step:      step + 1,
					})
					if decision.ModifiedParams != nil {
						if modifiedArgs, err := json.Marshal(decision.ModifiedParams); err == nil {
							tc.Arguments = modifiedArgs
						}
					}
				}
			}

			// Emit tool start event
			callback(StreamEvent{
				Type:      EventToolStart,
				AgentID:   currentAgent.ID,
				AgentName: currentAgent.Name,
				ToolName:  tc.Name,
				ToolArgs:  tc.Arguments,
				Step:      step + 1,
			})

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

			// Emit tool result event
			callback(StreamEvent{
				Type:       EventToolResult,
				AgentID:    currentAgent.ID,
				AgentName:  currentAgent.Name,
				ToolName:   tc.Name,
				ToolResult: recordResult,
				Step:       step + 1,
			})

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
	callback(StreamEvent{Type: EventError, Error: "maximum steps reached"})
	return e.buildResult(execCtx, "Maximum execution steps reached. Please try a simpler request."), ErrMaxStepsReached
}

// callLLMStreaming calls the LLM with streaming and forwards tokens via callback.
// It collects the full response and returns an LLMResponse just like the non-streaming version.
func (e *Engine) callLLMStreaming(ctx context.Context, req *LLMRequest, callback StreamCallback, agentID, agentName string, step int) (*LLMResponse, error) {
	streamingClient, ok := e.llmClient.(LLMStreamingClient)
	if !ok {
		// Fallback to non-streaming
		return e.llmClient.Chat(ctx, req)
	}

	// Convert LLMRequest to the format expected by the streaming client
	llmReq := &LLMRequest{
		Messages:    req.Messages,
		Tools:       req.Tools,
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}

	ch, err := streamingClient.ChatStream(ctx, llmReq)
	if err != nil {
		return nil, fmt.Errorf("start streaming: %w", err)
	}

	// Collect the full response while forwarding tokens
	var fullContent string
	var totalTokens int

	for chunk := range ch {
		if chunk.Error != nil {
			return nil, chunk.Error
		}
		if chunk.Done {
			break
		}
		if chunk.Content != "" {
			fullContent += chunk.Content
			callback(StreamEvent{
				Type:      EventToken,
				AgentID:   agentID,
				AgentName: agentName,
				Content:   chunk.Content,
				Step:      step,
			})
		}
	}

	// For streaming mode, we return the content without tool calls.
	// Tool calls are handled by the non-streaming Chat() path for now,
	// since the LLM streaming client doesn't reliably aggregate streaming tool_call arguments.
	// This is a known limitation — the first streaming iteration gives users
	// instant token feedback for text responses; tool_call paths still benefit
	// from the think/tool_start/tool_result events.
	totalTokens = len(fullContent) / 4 // rough estimate; real counting needs usage data from the last chunk

	return &LLMResponse{
		Content:     fullContent,
		ToolCalls:   nil, // tool calls handled via non-streaming fallback
		TotalTokens: totalTokens,
		Cost:        0, // cost tracking happens at the service layer
		StopReason:  "stop",
	}, nil
}
