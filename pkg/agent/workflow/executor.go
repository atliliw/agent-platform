package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"agent-platform/pkg/agent"
)

// EngineFunc is the callback for executing an agent
type EngineFunc func(ctx context.Context, agentID string, input string) (string, error)

// NodeResult holds the result of executing a single node
type NodeResult struct {
	NodeID string `json:"node_id"`
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// WorkflowResult holds the result of executing a workflow
type WorkflowResult struct {
	WorkflowID  string       `json:"workflow_id"`
	Nodes       []NodeResult `json:"nodes"`
	FinalOutput string       `json:"final_output"`
	Error       string       `json:"error,omitempty"`
}

// WorkflowExecutor executes workflows
type WorkflowExecutor struct {
	registry *agent.Registry
	engine   EngineFunc
}

// NewWorkflowExecutor creates a new workflow executor
func NewWorkflowExecutor(registry *agent.Registry, engine EngineFunc) *WorkflowExecutor {
	return &WorkflowExecutor{
		registry: registry,
		engine:   engine,
	}
}

// Execute runs a workflow from its entry node
func (e *WorkflowExecutor) Execute(ctx context.Context, wf *Workflow, input string) (*WorkflowResult, error) {
	if err := wf.Validate(); err != nil {
		return nil, fmt.Errorf("workflow validation failed: %w", err)
	}

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
		return result, fmt.Errorf("%w: %v", ErrExecutionFailed, err)
	}

	result.FinalOutput = finalOutput
	return result, nil
}

// executeNode executes a single node and follows edges to the next nodes
func (e *WorkflowExecutor) executeNode(ctx context.Context, wf *Workflow, nodeID string, input string, outputs map[string]string, result *WorkflowResult) (string, error) {
	node := wf.GetNode(nodeID)
	if node == nil {
		return "", ErrNodeNotFound
	}

	var output string
	var err error

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
	default:
		return "", fmt.Errorf("unknown node type: %s", node.Type)
	}

	if err != nil {
		nodeResult := NodeResult{NodeID: nodeID, Output: "", Error: err.Error()}
		result.Nodes = append(result.Nodes, nodeResult)
		return "", err
	}

	outputs[nodeID] = output
	nodeResult := NodeResult{NodeID: nodeID, Output: output}
	result.Nodes = append(result.Nodes, nodeResult)

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
	if node.AgentID == "" {
		return "", ErrMissingAgentID
	}

	if e.engine == nil {
		return "", fmt.Errorf("engine callback not configured")
	}

	output, err := e.engine(ctx, node.AgentID, input)
	if err != nil {
		return "", fmt.Errorf("agent %s execution failed: %w", node.AgentID, err)
	}

	return output, nil
}

// executeToolNode executes a tool node
func (e *WorkflowExecutor) executeToolNode(ctx context.Context, node *Node, input string) (string, error) {
	if node.ToolName == "" {
		return "", ErrMissingToolName
	}

	// Tool nodes are executed via the agent engine with a tool-specific prompt
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
	nodeResult := NodeResult{NodeID: node.ID, Output: input}
	result.Nodes = append(result.Nodes, nodeResult)

	// Evaluate condition against the input
	matched, err := evaluateCondition(node.Condition, input)
	if err != nil {
		return "", fmt.Errorf("condition evaluation failed: %w", err)
	}

	// Find matching edge
	edges := wf.GetOutgoingEdges(node.ID)
	for _, edge := range edges {
		if edge.Condition == "" {
			continue
		}

		edgeMatched, _ := evaluateCondition(edge.Condition, input)
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
	nodeResult := NodeResult{NodeID: node.ID, Output: input}
	result.Nodes = append(result.Nodes, nodeResult)

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
	nodeResult := NodeResult{NodeID: node.ID, Output: merged}
	result.Nodes = append(result.Nodes, nodeResult)

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

// evaluateCondition evaluates a simple condition expression against input
// Supports:
//   - "contains:substring" - checks if input contains substring
//   - "equals:value" - checks if input equals value
//   - "not_contains:substring" - checks if input does not contain substring
//   - "not_equals:value" - checks if input does not equal value
//   - "starts_with:prefix" - checks if input starts with prefix
//   - "ends_with:suffix" - checks if input ends with suffix
//   - "true" / "false" - literal boolean
func evaluateCondition(condition string, input string) (bool, error) {
	condition = strings.TrimSpace(condition)

	switch {
	case condition == "true":
		return true, nil
	case condition == "false":
		return false, nil
	case strings.HasPrefix(condition, "contains:"):
		substr := strings.TrimPrefix(condition, "contains:")
		return strings.Contains(input, substr), nil
	case strings.HasPrefix(condition, "not_contains:"):
		substr := strings.TrimPrefix(condition, "not_contains:")
		return !strings.Contains(input, substr), nil
	case strings.HasPrefix(condition, "equals:"):
		value := strings.TrimPrefix(condition, "equals:")
		return input == value, nil
	case strings.HasPrefix(condition, "not_equals:"):
		value := strings.TrimPrefix(condition, "not_equals:")
		return input != value, nil
	case strings.HasPrefix(condition, "starts_with:"):
		prefix := strings.TrimPrefix(condition, "starts_with:")
		return strings.HasPrefix(input, prefix), nil
	case strings.HasPrefix(condition, "ends_with:"):
		suffix := strings.TrimPrefix(condition, "ends_with:")
		return strings.HasSuffix(input, suffix), nil
	default:
		// Default: check if input contains the condition string
		return strings.Contains(input, condition), nil
	}
}
