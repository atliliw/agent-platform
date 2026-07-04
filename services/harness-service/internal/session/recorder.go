// Package session provides session replay functionality
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Recorder handles session recording and replay operations
type Recorder struct {
	repo *Repository
}

// NewRecorder creates a new session recorder
func NewRecorder(repo *Repository) *Recorder {
	return &Recorder{repo: repo}
}

// CreateSession creates a new session for recording
func (r *Recorder) CreateSession(ctx context.Context, agentID, traceID, model string, metadata map[string]string, tenantID string) (*Session, error) {
	session := &Session{
		AgentID:  agentID,
		TraceID:  traceID,
		Model:    model,
		Metadata: metadata,
		TenantID: tenantID,
	}

	if err := r.repo.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	fmt.Printf("[Session] Created session %s for agent %s (model: %s)\n", session.ID, agentID, model)
	return session, nil
}

// RecordStep records a step in a session
func (r *Recorder) RecordStep(ctx context.Context, sessionID string, stepType StepType, parentStepID string, input, output string, metadata map[string]interface{}, duration int64) (*SessionStep, error) {
	// Encode metadata to JSON
	var metaJSON string
	if metadata != nil {
		metaBytes, err := json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal metadata: %w", err)
		}
		metaJSON = string(metaBytes)
	}

	step := &SessionStep{
		SessionID:    sessionID,
		StepType:     stepType,
		ParentStepID: parentStepID,
		Input:        input,
		Output:       output,
		Metadata:     metaJSON,
		Duration:     duration,
	}

	if err := r.repo.CreateStep(ctx, step); err != nil {
		return nil, fmt.Errorf("create step: %w", err)
	}

	fmt.Printf("[Session] Recorded step %d (%s) for session %s, duration: %dms\n", step.StepNumber, stepType, sessionID, duration)
	return step, nil
}

// RecordLLMCall records an LLM call as a session step
func (r *Recorder) RecordLLMCall(ctx context.Context, sessionID string, parentStepID string, prompt, response string, model string, tokens int64, cost float64, latencyMs int64) (*SessionStep, error) {
	metadata := map[string]interface{}{
		"model":      model,
		"tokens":     tokens,
		"cost":       cost,
		"latency_ms": latencyMs,
	}

	inputJSON, _ := json.Marshal(map[string]interface{}{
		"prompt": prompt,
		"model":  model,
	})

	outputJSON, _ := json.Marshal(map[string]interface{}{
		"response": response,
		"tokens":   tokens,
	})

	return r.RecordStep(ctx, sessionID, StepTypeLLMCall, parentStepID, string(inputJSON), string(outputJSON), metadata, latencyMs)
}

// RecordToolCall records a tool call as a session step
func (r *Recorder) RecordToolCall(ctx context.Context, sessionID string, parentStepID string, toolName, toolInput string, toolOutput string, success bool, duration int64) (*SessionStep, error) {
	metadata := map[string]interface{}{
		"tool":    toolName,
		"success": success,
	}

	inputJSON, _ := json.Marshal(map[string]interface{}{
		"tool":  toolName,
		"input": toolInput,
	})

	outputJSON, _ := json.Marshal(map[string]interface{}{
		"output":  toolOutput,
		"success": success,
	})

	return r.RecordStep(ctx, sessionID, StepTypeToolCall, parentStepID, string(inputJSON), string(outputJSON), metadata, duration)
}

// RecordThink records a thinking step
func (r *Recorder) RecordThink(ctx context.Context, sessionID string, parentStepID string, thoughts string, duration int64) (*SessionStep, error) {
	metadata := map[string]interface{}{
		"type": "thinking",
	}

	return r.RecordStep(ctx, sessionID, StepTypeThink, parentStepID, thoughts, thoughts, metadata, duration)
}

// RecordDecision records a decision step
func (r *Recorder) RecordDecision(ctx context.Context, sessionID string, parentStepID string, decision, rationale string, duration int64) (*SessionStep, error) {
	metadata := map[string]interface{}{
		"type": "decision",
	}

	inputJSON, _ := json.Marshal(map[string]interface{}{
		"rationale": rationale,
	})

	return r.RecordStep(ctx, sessionID, StepTypeDecision, parentStepID, string(inputJSON), decision, metadata, duration)
}

// RecordObservation records an observation step
func (r *Recorder) RecordObservation(ctx context.Context, sessionID string, parentStepID string, observation string, duration int64) (*SessionStep, error) {
	metadata := map[string]interface{}{
		"type": "observation",
	}

	return r.RecordStep(ctx, sessionID, StepTypeObservation, parentStepID, "", observation, metadata, duration)
}

// RecordAction records an action step
func (r *Recorder) RecordAction(ctx context.Context, sessionID string, parentStepID string, actionType, actionInput, actionOutput string, duration int64) (*SessionStep, error) {
	metadata := map[string]interface{}{
		"action_type": actionType,
	}

	return r.RecordStep(ctx, sessionID, StepTypeAction, parentStepID, actionInput, actionOutput, metadata, duration)
}

// EndSession ends a session
func (r *Recorder) EndSession(ctx context.Context, sessionID string, status SessionStatus) (*Session, error) {
	session, err := r.repo.EndSession(ctx, sessionID, status)
	if err != nil {
		return nil, fmt.Errorf("end session: %w", err)
	}

	fmt.Printf("[Session] Ended session %s with status %s, duration: %dms, tokens: %d, cost: %.6f\n",
		sessionID, status, session.Duration, session.TotalTokens, session.TotalCost)
	return session, nil
}

// GetSession retrieves a session with its details
func (r *Recorder) GetSession(ctx context.Context, sessionID string) (*SessionDetail, error) {
	session, err := r.repo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	steps, err := r.repo.ListSteps(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list steps: %w", err)
	}

	graph := r.BuildExecutionGraph(steps)

	return &SessionDetail{
		Session: *session,
		Steps:   steps,
		Graph:   graph,
	}, nil
}

// ListSessions lists sessions with filters
func (r *Recorder) ListSessions(ctx context.Context, filter *ListSessionsFilter) ([]*Session, int64, error) {
	return r.repo.ListSessions(ctx, filter)
}

// BuildExecutionGraph builds an execution graph from steps
func (r *Recorder) BuildExecutionGraph(steps []SessionStep) SessionGraph {
	graph := SessionGraph{
		Nodes: []GraphNode{},
		Edges: []GraphEdge{},
	}

	// Create nodes for each step
	for _, step := range steps {
		node := GraphNode{
			ID:       step.ID,
			Type:     string(step.StepType),
			Label:    r.getStepLabel(step),
			Duration: step.Duration,
			Status:   string(step.Status),
			Metadata: make(map[string]string),
		}

		// Add metadata if available
		if step.Metadata != "" {
			var meta map[string]interface{}
			if err := json.Unmarshal([]byte(step.Metadata), &meta); err == nil {
				// Extract key fields
				if model, ok := meta["model"].(string); ok {
					node.Metadata["model"] = model
				}
				if tool, ok := meta["tool"].(string); ok {
					node.Metadata["tool"] = tool
				}
				if tokens, ok := meta["tokens"].(float64); ok {
					node.Metadata["tokens"] = fmt.Sprintf("%d", int64(tokens))
				}
				if cost, ok := meta["cost"].(float64); ok {
					node.Metadata["cost"] = fmt.Sprintf("%.4f", cost)
				}
			}
		}

		graph.Nodes = append(graph.Nodes, node)
	}

	// Create edges for parent-child relationships and sequential flow
	for i, step := range steps {
		// Add edge for parent relationship
		if step.ParentStepID != "" {
			edge := GraphEdge{
				From:  step.ParentStepID,
				To:    step.ID,
				Label: "parent",
			}
			graph.Edges = append(graph.Edges, edge)
		} else if i > 0 {
			// Add edge for sequential flow if no parent
			// Find the previous step without a parent that this might follow
			for j := i - 1; j >= 0; j-- {
				if steps[j].ParentStepID == "" || steps[j].ParentStepID == steps[j-1].ID {
					edge := GraphEdge{
						From:  steps[j].ID,
						To:    step.ID,
						Label: "next",
					}
					graph.Edges = append(graph.Edges, edge)
					break
				}
			}
		}
	}

	return graph
}

// getStepLabel generates a human-readable label for a step
func (r *Recorder) getStepLabel(step SessionStep) string {
	switch step.StepType {
	case StepTypeThink:
		return "Think"
	case StepTypeToolCall:
		// Extract tool name from metadata
		if step.Metadata != "" {
			var meta map[string]interface{}
			if err := json.Unmarshal([]byte(step.Metadata), &meta); err == nil {
				if tool, ok := meta["tool"].(string); ok {
					return fmt.Sprintf("Tool: %s", tool)
				}
			}
		}
		return "Tool Call"
	case StepTypeAction:
		return "Action"
	case StepTypeObservation:
		return "Observation"
	case StepTypeDecision:
		// Extract decision from output
		if step.Output != "" && len(step.Output) < 50 {
			return fmt.Sprintf("Decision: %s", step.Output)
		}
		return "Decision"
	case StepTypeLLMCall:
		// Extract model from metadata
		if step.Metadata != "" {
			var meta map[string]interface{}
			if err := json.Unmarshal([]byte(step.Metadata), &meta); err == nil {
				if model, ok := meta["model"].(string); ok {
					return fmt.Sprintf("LLM: %s", model)
				}
			}
		}
		return "LLM Call"
	default:
		return string(step.StepType)
	}
}

// ReplaySession replays a session and compares outputs
func (r *Recorder) ReplaySession(ctx context.Context, sessionID string, fromStep, toStep int32, dryRun bool, executor func(ctx context.Context, step SessionStep) (string, error)) (*ReplaySession, error) {
	// Get original session
	session, err := r.repo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	_ = session // session info for potential future use

	// Get steps
	steps, err := r.repo.ListSteps(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list steps: %w", err)
	}

	// Filter steps if from/to specified
	filteredSteps := steps
	if fromStep > 0 || toStep > 0 {
		filteredSteps = []SessionStep{}
		for _, step := range steps {
			if fromStep > 0 && step.StepNumber < fromStep {
				continue
			}
			if toStep > 0 && step.StepNumber > toStep {
				continue
			}
			filteredSteps = append(filteredSteps, step)
		}
	}

	replay := &ReplaySession{
		SessionID: sessionID,
		FromStep:  fromStep,
		ToStep:    toStep,
		DryRun:    dryRun,
		Diffs:     []ReplayDiff{},
	}

	if err := r.repo.CreateReplaySession(ctx, replay); err != nil {
		return nil, fmt.Errorf("create replay: %w", err)
	}

	fmt.Printf("[Session] Starting replay %s for session %s (dry_run: %v, steps: %d-%d)\n",
		replay.ID, sessionID, dryRun, fromStep, toStep)

	// Execute each step
	for _, step := range filteredSteps {
		var replayOutput string
		var err error

		if dryRun {
			// In dry-run mode, just simulate with original output
			replayOutput = step.Output
		} else if executor != nil {
			// Use the provided executor to replay the step
			replayOutput, err = executor(ctx, step)
			if err != nil {
				fmt.Printf("[Session] Replay step %d failed: %v\n", step.StepNumber, err)
				replay.Error = fmt.Sprintf("Step %d failed: %v", step.StepNumber, err)
				replay.Success = false
				now := time.Now()
				replay.EndedAt = &now
				r.repo.UpdateReplaySession(ctx, replay)
				return replay, nil
			}
		} else {
			// Without executor, just use original output
			replayOutput = step.Output
		}

		// Compare outputs
		diff := ReplayDiff{
			StepID:         step.ID,
			StepNumber:     step.StepNumber,
			OriginalOutput: step.Output,
			ReplayOutput:   replayOutput,
			Matches:        r.compareOutputs(step.Output, replayOutput),
		}
		replay.Diffs = append(replay.Diffs, diff)

		if !diff.Matches {
			fmt.Printf("[Session] Step %d output differs\n", step.StepNumber)
		}
	}

	replay.Success = true
	now := time.Now()
	replay.EndedAt = &now

	if err := r.repo.UpdateReplaySession(ctx, replay); err != nil {
		return nil, fmt.Errorf("update replay: %w", err)
	}

	fmt.Printf("[Session] Replay %s completed with %d diffs\n", replay.ID, len(replay.Diffs))
	return replay, nil
}

// compareOutputs compares two outputs for equivalence
func (r *Recorder) compareOutputs(original, replay string) bool {
	// Normalize JSON outputs
	var origJSON, replayJSON interface{}

	if err := json.Unmarshal([]byte(original), &origJSON); err == nil {
		if err := json.Unmarshal([]byte(replay), &replayJSON); err == nil {
			// Compare as JSON objects
			origNorm, _ := json.Marshal(origJSON)
			replayNorm, _ := json.Marshal(replayJSON)
			return string(origNorm) == string(replayNorm)
		}
	}

	// Fall back to string comparison
	// Normalize whitespace and quotes
	origNorm := strings.TrimSpace(original)
	replayNorm := strings.TrimSpace(replay)
	return origNorm == replayNorm
}

// ExportSession exports a session to a specified format
func (r *Recorder) ExportSession(ctx context.Context, sessionID string, format string) (string, error) {
	detail, err := r.GetSession(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("get session: %w", err)
	}

	switch format {
	case "json":
		data, err := json.MarshalIndent(detail, "", "  ")
		if err != nil {
			return "", fmt.Errorf("marshal json: %w", err)
		}
		return string(data), nil

	case "markdown":
		return r.exportMarkdown(detail), nil

	case "html":
		return r.exportHTML(detail), nil

	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

// exportMarkdown exports session as Markdown
func (r *Recorder) exportMarkdown(detail *SessionDetail) string {
	md := fmt.Sprintf("# Session Replay: %s\n\n", detail.Session.ID)
	md += fmt.Sprintf("- **Agent**: %s\n", detail.Session.AgentID)
	md += fmt.Sprintf("- **Model**: %s\n", detail.Session.Model)
	md += fmt.Sprintf("- **Status**: %s\n", detail.Session.Status)
	md += fmt.Sprintf("- **Duration**: %dms\n", detail.Session.Duration)
	md += fmt.Sprintf("- **Tokens**: %d\n", detail.Session.TotalTokens)
	md += fmt.Sprintf("- **Cost**: %.6f\n", detail.Session.TotalCost)
	md += fmt.Sprintf("- **Started**: %s\n", detail.Session.StartTime.Format(time.RFC3339))
	if detail.Session.EndTime != nil {
		md += fmt.Sprintf("- **Ended**: %s\n", detail.Session.EndTime.Format(time.RFC3339))
	}

	md += "\n## Execution Steps\n\n"
	for _, step := range detail.Steps {
		md += fmt.Sprintf("### Step %d: %s\n\n", step.StepNumber, step.StepType)
		md += fmt.Sprintf("- **Duration**: %dms\n", step.Duration)
		md += fmt.Sprintf("- **Status**: %s\n", step.Status)

		if step.Input != "" {
			md += "\n**Input**:\n```\n" + step.Input + "\n```\n"
		}
		if step.Output != "" {
			md += "\n**Output**:\n```\n" + step.Output + "\n```\n"
		}
		md += "\n"
	}

	md += "\n## Execution Graph\n\n"
	md += "```mermaid\ngraph TD\n"
	for _, node := range detail.Graph.Nodes {
		md += fmt.Sprintf("    %s[%s\\n%s\\n%dms]\n", node.ID, node.Type, node.Label, node.Duration)
	}
	for _, edge := range detail.Graph.Edges {
		md += fmt.Sprintf("    %s -->|%s| %s\n", edge.From, edge.Label, edge.To)
	}
	md += "```\n"

	return md
}

// exportHTML exports session as HTML
func (r *Recorder) exportHTML(detail *SessionDetail) string {
	html := fmt.Sprintf(`<html>
<head>
<title>Session Replay: %s</title>
<style>
body { font-family: Arial, sans-serif; margin: 20px; }
.session-header { background: #f5f5f5; padding: 15px; border-radius: 5px; margin-bottom: 20px; }
.step { border: 1px solid #ddd; padding: 10px; margin: 10px 0; border-radius: 5px; }
.step-header { font-weight: bold; color: #333; }
.step-content { margin-top: 10px; }
.input, .output { background: #f9f9f9; padding: 8px; border-radius: 3px; margin: 5px 0; }
pre { white-space: pre-wrap; word-wrap: break-word; }
</style>
</head>
<body>
<h1>Session Replay: %s</h1>
<div class="session-header">
<p><strong>Agent:</strong> %s</p>
<p><strong>Model:</strong> %s</p>
<p><strong>Status:</strong> %s</p>
<p><strong>Duration:</strong> %dms</p>
<p><strong>Tokens:</strong> %d</p>
<p><strong>Cost:</strong> %.6f</p>
<p><strong>Started:</strong> %s</p>
`, detail.Session.ID, detail.Session.ID, detail.Session.AgentID, detail.Session.Model,
		detail.Session.Status, detail.Session.Duration, detail.Session.TotalTokens,
		detail.Session.TotalCost, detail.Session.StartTime.Format(time.RFC3339))

	if detail.Session.EndTime != nil {
		html += fmt.Sprintf("<p><strong>Ended:</strong> %s</p>\n", detail.Session.EndTime.Format(time.RFC3339))
	}

	html += "</div>\n<h2>Execution Steps</h2>\n"

	for _, step := range detail.Steps {
		html += fmt.Sprintf(`<div class="step">
<div class="step-header">Step %d: %s (%dms, %s)</div>
<div class="step-content">
`, step.StepNumber, step.StepType, step.Duration, step.Status)

		if step.Input != "" {
			html += fmt.Sprintf(`<div class="input"><strong>Input:</strong><pre>%s</pre></div>
`, step.Input)
		}
		if step.Output != "" {
			html += fmt.Sprintf(`<div class="output"><strong>Output:</strong><pre>%s</pre></div>
`, step.Output)
		}

		html += "</div>\n</div>\n"
	}

	html += "</body>\n</html>"
	return html
}

// GetStats returns session statistics for an agent
func (r *Recorder) GetStats(ctx context.Context, agentID string) (*SessionStats, error) {
	return r.repo.GetSessionStats(ctx, agentID)
}

// DeleteSession deletes a session
func (r *Recorder) DeleteSession(ctx context.Context, sessionID string) error {
	return r.repo.DeleteSession(ctx, sessionID)
}