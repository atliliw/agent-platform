package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"agent-platform/pkg/agent"
)

// EngineFunc is the callback for executing an agent
type EngineFunc func(ctx context.Context, agentID string, input string) (string, error)

// ToolFunc is the callback for executing a tool directly
type ToolFunc func(ctx context.Context, toolName string, input string, config map[string]interface{}) (string, error)

// StreamEventFunc is the callback for streaming execution events
type StreamEventFunc func(eventType string, nodeID string, nodeName string, nodeType string, output string, err error)

// NodeTimeoutProvider returns timeout duration for a given node
type NodeTimeoutProvider func(nodeID string) time.Duration

// ExecutionConfig holds configuration for workflow execution
type ExecutionConfig struct {
	NodeTimeout  time.Duration // per-node timeout (0 = no timeout)
	MaxRetries   int           // retries on failure (0 = no retry)
	RetryDelay   time.Duration // initial retry delay
}

// NodeResult holds the result of executing a single node
type NodeResult struct {
	NodeID    string `json:"node_id"`
	NodeName  string `json:"node_name,omitempty"`
	NodeType  string `json:"node_type,omitempty"`
	Output    string `json:"output"`
	Error     string `json:"error,omitempty"`
	Duration  int64  `json:"duration_ms,omitempty"`
	Retries   int    `json:"retries,omitempty"`
	Status    string `json:"status,omitempty"` // pending, running, completed, failed, timed_out
}

// WorkflowResult holds the result of executing a workflow
type WorkflowResult struct {
	WorkflowID  string       `json:"workflow_id"`
	Nodes       []NodeResult `json:"nodes"`
	FinalOutput string       `json:"final_output"`
	Error       string       `json:"error,omitempty"`
	Duration    int64        `json:"duration_ms,omitempty"`
}

// WorkflowExecutor executes workflows
type WorkflowExecutor struct {
	registry  *agent.Registry
	engine    EngineFunc
	toolFunc  ToolFunc
	config    ExecutionConfig
	timeoutFn NodeTimeoutProvider
	onEvent   StreamEventFunc
}

// NewWorkflowExecutor creates a new workflow executor
func NewWorkflowExecutor(registry *agent.Registry, engine EngineFunc) *WorkflowExecutor {
	return &WorkflowExecutor{
		registry: registry,
		engine:   engine,
		config:   ExecutionConfig{},
	}
}

// NewWorkflowExecutorWithConfig creates a workflow executor with configuration
func NewWorkflowExecutorWithConfig(registry *agent.Registry, engine EngineFunc, config ExecutionConfig) *WorkflowExecutor {
	return &WorkflowExecutor{
		registry: registry,
		engine:   engine,
		config:   config,
	}
}

// SetToolFunc sets the tool execution callback
func (e *WorkflowExecutor) SetToolFunc(fn ToolFunc) {
	e.toolFunc = fn
}

// SetTimeoutProvider sets the per-node timeout provider
func (e *WorkflowExecutor) SetTimeoutProvider(fn NodeTimeoutProvider) {
	e.timeoutFn = fn
}

// SetStreamEventFunc sets the streaming event callback
func (e *WorkflowExecutor) SetStreamEventFunc(fn StreamEventFunc) {
	e.onEvent = fn
}

// Execute runs a workflow from its entry node
func (e *WorkflowExecutor) Execute(ctx context.Context, wf *Workflow, input string) (*WorkflowResult, error) {
	if err := wf.Validate(); err != nil {
		return nil, fmt.Errorf("workflow validation failed: %w", err)
	}

	startTime := time.Now()
	result := &WorkflowResult{
		WorkflowID: wf.ID,
		Nodes:      make([]NodeResult, 0),
	}

	// Track node outputs for condition evaluation and merge collection
	outputs := make(map[string]string)
	outputs[wf.EntryNodeID] = input

	// Execute starting from entry node
	finalOutput, err := e.executeNode(ctx, wf, wf.EntryNodeID, input, outputs, result)
	if err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(startTime).Milliseconds()
		return result, fmt.Errorf("%w: %v", ErrExecutionFailed, err)
	}

	result.FinalOutput = finalOutput
	result.Duration = time.Since(startTime).Milliseconds()
	return result, nil
}

// executeNode executes a single node and follows edges to the next nodes
func (e *WorkflowExecutor) executeNode(ctx context.Context, wf *Workflow, nodeID string, input string, outputs map[string]string, result *WorkflowResult) (string, error) {
	node := wf.GetNode(nodeID)
	if node == nil {
		return "", ErrNodeNotFound
	}

	// Apply per-node timeout
	if e.timeoutFn != nil {
		if timeout := e.timeoutFn(nodeID); timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
	} else if e.config.NodeTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.config.NodeTimeout)
		defer cancel()
	}

	// Emit stream event: node started
	if e.onEvent != nil {
		e.onEvent("node_started", node.ID, node.Name, string(node.Type), "", nil)
	}

	var output string
	var err error

	startTime := time.Now()

	switch node.Type {
	case NodeAgent:
		output, err = e.executeAgentNode(ctx, node, input)
	case NodeTool:
		output, err = e.executeToolNode(ctx, node, input)
	case NodeCondition:
		output, err = e.executeConditionNode(ctx, wf, node, input, outputs, result)
		// Condition nodes handle their own edge traversal
		return output, err
	case NodeParallel:
		output, err = e.executeParallelNode(ctx, wf, node, input, outputs, result)
		// Parallel nodes handle their own edge traversal
		return output, err
	case NodeMerge:
		output, err = e.executeMergeNode(ctx, wf, node, outputs, result)
		return output, err
	default:
		return "", fmt.Errorf("unknown node type: %s", node.Type)
	}

	if err != nil {
		// Retry logic
		retries := 0
		if e.config.MaxRetries > 0 {
			for retries = 1; retries <= e.config.MaxRetries; retries++ {
				select {
				case <-ctx.Done():
					nodeResult := NodeResult{
						NodeID:   nodeID,
						NodeName: node.Name,
						NodeType: string(node.Type),
						Error:    fmt.Sprintf("context cancelled after %d retries: %v", retries, ctx.Err()),
						Duration: time.Since(startTime).Milliseconds(),
						Retries:  retries,
						Status:   "timed_out",
					}
					result.Nodes = append(result.Nodes, nodeResult)
					if e.onEvent != nil {
						e.onEvent("node_error", node.ID, node.Name, string(node.Type), "", ErrNodeTimeout)
					}
					return "", ErrNodeTimeout
				case <-time.After(e.config.RetryDelay * time.Duration(1<<uint(retries-1))):
					// Exponential backoff retry
				}

				switch node.Type {
				case NodeAgent:
					output, err = e.executeAgentNode(ctx, node, input)
				case NodeTool:
					output, err = e.executeToolNode(ctx, node, input)
				default:
					break
				}

				if err == nil {
					break
				}
			}
		}

		if err != nil {
			status := "failed"
			if ctx.Err() == context.DeadlineExceeded {
				status = "timed_out"
			}
			nodeResult := NodeResult{
				NodeID:   nodeID,
				NodeName: node.Name,
				NodeType: string(node.Type),
				Error:    err.Error(),
				Duration: time.Since(startTime).Milliseconds(),
				Retries:  retries,
				Status:   status,
			}
			result.Nodes = append(result.Nodes, nodeResult)
			if e.onEvent != nil {
				e.onEvent("node_error", node.ID, node.Name, string(node.Type), "", err)
			}
			return "", err
		}
	}

	outputs[nodeID] = output
	nodeResult := NodeResult{
		NodeID:   nodeID,
		NodeName: node.Name,
		NodeType: string(node.Type),
		Output:   output,
		Duration: time.Since(startTime).Milliseconds(),
		Status:   "completed",
	}
	result.Nodes = append(result.Nodes, nodeResult)

	// Emit stream event: node completed
	if e.onEvent != nil {
		e.onEvent("node_completed", node.ID, node.Name, string(node.Type), output, nil)
	}

	// Follow edges to next nodes
	edges := wf.GetOutgoingEdges(nodeID)
	if len(edges) == 0 {
		return output, nil
	}

	// For simple linear flow, follow the first edge
	nextOutput, err := e.executeNode(ctx, wf, edges[0].To, output, outputs, result)
	if err != nil {
		return "", err
	}

	return nextOutput, nil
}

// executeAgentNode executes an agent node
func (e *WorkflowExecutor) executeAgentNode(ctx context.Context, node *Node, input string) (string, error) {
	if e.engine == nil {
		return "", fmt.Errorf("engine callback not configured")
	}

	// If agent_id is set, route to specific agent; otherwise use default
	agentID := node.AgentID
	if agentID == "" && e.registry != nil {
		if defaultAgent := e.registry.GetDefault(); defaultAgent != nil {
			agentID = defaultAgent.ID
		}
	}

	output, err := e.engine(ctx, agentID, input)
	if err != nil {
		return "", fmt.Errorf("agent %s execution failed: %w", agentID, err)
	}

	return output, nil
}

// executeToolNode executes a tool node
func (e *WorkflowExecutor) executeToolNode(ctx context.Context, node *Node, input string) (string, error) {
	if node.ToolName == "" {
		return "", ErrMissingToolName
	}

	// Use ToolFunc if available for direct tool execution
	if e.toolFunc != nil {
		output, err := e.toolFunc(ctx, node.ToolName, input, node.Config)
		if err != nil {
			return "", fmt.Errorf("tool %s execution failed: %w", node.ToolName, err)
		}
		return output, nil
	}

	// Fallback: execute via agent engine with a tool-specific prompt
	if e.engine == nil {
		return "", fmt.Errorf("engine callback not configured")
	}

	// Build a tool execution prompt
	prompt := fmt.Sprintf("Execute tool '%s' with input: %s", node.ToolName, input)
	if node.Config != nil {
		if configJSON, err := json.Marshal(node.Config); err == nil {
			prompt = fmt.Sprintf("Execute tool '%s' with config: %s and input: %s", node.ToolName, string(configJSON), input)
		}
	}

	output, err := e.engine(ctx, node.AgentID, prompt)
	if err != nil {
		return "", fmt.Errorf("tool %s execution failed: %w", node.ToolName, err)
	}

	return output, nil
}

// executeConditionNode evaluates a condition and follows the matching edge
func (e *WorkflowExecutor) executeConditionNode(ctx context.Context, wf *Workflow, node *Node, input string, outputs map[string]string, result *WorkflowResult) (string, error) {
	if node.Condition == "" {
		return "", ErrMissingCondition
	}

	outputs[node.ID] = input
	nodeResult := NodeResult{
		NodeID:   node.ID,
		NodeName: node.Name,
		NodeType: string(node.Type),
		Output:   input,
		Status:   "completed",
	}
	result.Nodes = append(result.Nodes, nodeResult)

	if e.onEvent != nil {
		e.onEvent("node_completed", node.ID, node.Name, string(node.Type), input, nil)
	}

	// Evaluate condition against the input
	matched, err := evaluateCondition(node.Condition, input, outputs)
	if err != nil {
		return "", fmt.Errorf("condition evaluation failed: %w", err)
	}

	// Find matching edge
	edges := wf.GetOutgoingEdges(node.ID)
	for _, edge := range edges {
		if edge.Condition == "" {
			continue
		}

		edgeMatched, _ := evaluateCondition(edge.Condition, input, outputs)
		if edgeMatched || (edge.Condition == "true" && matched) || (edge.Condition == "false" && !matched) {
			nextOutput, err := e.executeNode(ctx, wf, edge.To, input, outputs, result)
			if err != nil {
				return "", err
			}
			return nextOutput, nil
		}
	}

	// If no condition edge matched, try the default edge (no condition)
	for _, edge := range edges {
		if edge.Condition == "" {
			nextOutput, err := e.executeNode(ctx, wf, edge.To, input, outputs, result)
			if err != nil {
				return "", err
			}
			return nextOutput, nil
		}
	}

	return "", ErrConditionNotMatched
}

// executeParallelNode fans out to multiple branches concurrently
func (e *WorkflowExecutor) executeParallelNode(ctx context.Context, wf *Workflow, node *Node, input string, outputs map[string]string, result *WorkflowResult) (string, error) {
	outputs[node.ID] = input
	nodeResult := NodeResult{
		NodeID:   node.ID,
		NodeName: node.Name,
		NodeType: string(node.Type),
		Output:   input,
		Status:   "completed",
	}
	result.Nodes = append(result.Nodes, nodeResult)

	if e.onEvent != nil {
		e.onEvent("node_completed", node.ID, node.Name, string(node.Type), input, nil)
	}

	edges := wf.GetOutgoingEdges(node.ID)
	if len(edges) == 0 {
		return input, nil
	}

	type branchResult struct {
		output string
		err    error
		index  int
	}

	results := make([]branchResult, len(edges))
	var wg sync.WaitGroup

	for i, edge := range edges {
		wg.Add(1)
		go func(idx int, edgeID string, toNodeID string) {
			defer wg.Done()
			out, err := e.executeNode(ctx, wf, toNodeID, input, outputs, result)
			results[idx] = branchResult{output: out, err: err, index: idx}
		}(i, edge.ID, edge.To)
	}

	wg.Wait()

	// Collect outputs from all branches
	var combinedOutputs []string
	for _, r := range results {
		if r.err != nil {
			return "", fmt.Errorf("parallel branch %d failed: %w", r.index, r.err)
		}
		combinedOutputs = append(combinedOutputs, r.output)
	}

	// Return combined output
	return strings.Join(combinedOutputs, "\n---\n"), nil
}

// executeMergeNode collects results from parallel branches
func (e *WorkflowExecutor) executeMergeNode(ctx context.Context, wf *Workflow, node *Node, outputs map[string]string, result *WorkflowResult) (string, error) {
	incoming := wf.GetIncomingEdges(node.ID)

	var collected []string
	for _, edge := range incoming {
		if out, ok := outputs[edge.From]; ok {
			collected = append(collected, out)
		}
	}

	merged := strings.Join(collected, "\n---\n")
	outputs[node.ID] = merged
	nodeResult := NodeResult{
		NodeID:   node.ID,
		NodeName: node.Name,
		NodeType: string(node.Type),
		Output:   merged,
		Status:   "completed",
	}
	result.Nodes = append(result.Nodes, nodeResult)

	if e.onEvent != nil {
		e.onEvent("node_completed", node.ID, node.Name, string(node.Type), merged, nil)
	}

	// Follow edges after merge
	edges := wf.GetOutgoingEdges(node.ID)
	if len(edges) == 0 {
		return merged, nil
	}

	nextOutput, err := e.executeNode(ctx, wf, edges[0].To, merged, outputs, result)
	if err != nil {
		return "", err
	}

	return nextOutput, nil
}

// nodeRefPattern matches "nodes.<node_id>.output" references in condition expressions
var nodeRefPattern = regexp.MustCompile(`nodes\.([^.]+)\.output`)

// resolveNodeRefs replaces nodes.X.output references with actual output values
func resolveNodeRefs(condition string, outputs map[string]string) string {
	return nodeRefPattern.ReplaceAllStringFunc(condition, func(match string) string {
		submatch := nodeRefPattern.FindStringSubmatch(match)
		if len(submatch) >= 2 {
			if out, ok := outputs[submatch[1]]; ok {
				return out
			}
		}
		return match
	})
}

// nodeRefOperatorPattern matches "nodes.<id>.output <operator>:<value>"
var nodeRefOperatorPattern = regexp.MustCompile(
	`^nodes\.([^.]+)\.output\s+(contains|not_contains|equals|not_equals|starts_with|ends_with|len_gt|len_lt|regex):(.+)$`,
)

// evaluateCondition evaluates a condition expression against input
// Supports:
//   - "contains:substring" - checks if input contains substring
//   - "not_contains:substring" - checks if input does not contain substring
//   - "equals:value" - checks if input equals value
//   - "not_equals:value" - checks if input does not equal value
//   - "starts_with:prefix" - checks if input starts with prefix
//   - "ends_with:suffix" - checks if input ends with suffix
//   - "len_gt:N" - input length greater than N
//   - "len_lt:N" - input length less than N
//   - "regex:PATTERN" - regex match
//   - "true" / "false" - literal boolean
//   - "nodes.<node_id>.output <operator>:<value>" - reference previous node output
func evaluateCondition(condition string, input string, outputs map[string]string) (bool, error) {
	condition = strings.TrimSpace(condition)

	// Check for node reference with operator pattern: "nodes.X.output <op>:<val>"
	if submatch := nodeRefOperatorPattern.FindStringSubmatch(condition); len(submatch) >= 4 {
		nodeID := submatch[1]
		operator := submatch[2]
		operand := submatch[3]

		// Resolve the referenced node's output
		refInput, ok := outputs[nodeID]
		if !ok {
			return false, fmt.Errorf("referenced node %s output not found", nodeID)
		}

		return evaluateOperator(operator, operand, refInput)
	}

	// Resolve inline node references (e.g., "contains:nodes.X.output")
	resolved := resolveNodeRefs(condition, outputs)

	// Evaluate the resolved condition
	parts := strings.SplitN(resolved, ":", 2)
	if len(parts) == 2 {
		operator := parts[0]
		operand := parts[1]
		return evaluateOperator(operator, operand, input)
	}

	// Simple literal checks
	switch {
	case condition == "true":
		return true, nil
	case condition == "false":
		return false, nil
	default:
		// Default: check if input contains the condition string
		return strings.Contains(input, condition), nil
	}
}

// evaluateOperator evaluates a condition operator against an input value
func evaluateOperator(operator, operand, input string) (bool, error) {
	switch operator {
	case "contains":
		return strings.Contains(input, operand), nil
	case "not_contains":
		return !strings.Contains(input, operand), nil
	case "equals":
		return input == operand, nil
	case "not_equals":
		return input != operand, nil
	case "starts_with":
		return strings.HasPrefix(input, operand), nil
	case "ends_with":
		return strings.HasSuffix(input, operand), nil
	case "len_gt":
		var n int
		if _, err := fmt.Sscanf(operand, "%d", &n); err != nil {
			return false, fmt.Errorf("invalid len_gt value: %s", operand)
		}
		return len(input) > n, nil
	case "len_lt":
		var n int
		if _, err := fmt.Sscanf(operand, "%d", &n); err != nil {
			return false, fmt.Errorf("invalid len_lt value: %s", operand)
		}
		return len(input) < n, nil
	case "regex":
		matched, err := regexp.MatchString(operand, input)
		if err != nil {
			return false, fmt.Errorf("invalid regex pattern: %w", err)
		}
		return matched, nil
	default:
		// Unknown operator, fall back to contains
		return strings.Contains(input, operand), nil
	}
}
