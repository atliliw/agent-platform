package workflow

import "errors"

// Workflow errors
var (
	ErrWorkflowNameRequired  = errors.New("workflow name is required")
	ErrEntryNodeRequired     = errors.New("entry node ID is required")
	ErrEntryNodeNotFound     = errors.New("entry node not found in workflow nodes")
	ErrDuplicateNodeID       = errors.New("duplicate node ID detected")
	ErrEdgeFromNotFound      = errors.New("edge source node not found")
	ErrEdgeToNotFound        = errors.New("edge target node not found")
	ErrCycleDetected         = errors.New("workflow contains a cycle")
	ErrNodeNotFound          = errors.New("node not found")
	ErrNoOutgoingEdges       = errors.New("no outgoing edges for node")
	ErrConditionNotMatched   = errors.New("no condition matched")
	ErrExecutionFailed       = errors.New("workflow execution failed")
	ErrMissingAgentID        = errors.New("agent node missing agent_id")
	ErrMissingToolName       = errors.New("tool node missing tool_name")
	ErrMissingCondition      = errors.New("condition node missing condition expression")
	ErrNodeTimeout           = errors.New("node execution timed out")
	ErrNodeRetriesExhausted  = errors.New("node retries exhausted")
	ErrWorkflowCancelled     = errors.New("workflow execution cancelled")
	ErrToolFuncNotConfigured = errors.New("tool function not configured")
)
