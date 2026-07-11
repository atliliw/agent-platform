// Package workflow provides the workflow execution engine for the harness service
package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	wfpkg "agent-platform/pkg/agent/workflow"
	"agent-platform/pkg/llm"
	agentpb "agent-platform/pkg/pb/agent"
	"agent-platform/services/harness-service/internal/repository"
)

// ExecutionStatus represents the status of a workflow execution
type ExecutionStatus string

const (
	StatusPending   ExecutionStatus = "pending"
	StatusRunning   ExecutionStatus = "running"
	StatusCompleted ExecutionStatus = "completed"
	StatusFailed    ExecutionStatus = "failed"
	StatusCancelled ExecutionStatus = "cancelled"
	StatusTimedOut  ExecutionStatus = "timed_out"
)

// MetricsRecorder records an LLM call metric (for trace display / cost / SLO)
type MetricsRecorder func(ctx context.Context, m *llm.CallMetrics)

// Engine wraps wfpkg.WorkflowExecutor with service-specific callbacks
type Engine struct {
	workflowRepo    *repository.WorkflowRepository
	executionRepo   *ExecutionRepository
	llmClient       llm.Client
	agentClient     agentpb.AgentServiceClient
	metricsRecorder MetricsRecorder
	defaultTimeout  time.Duration
	maxRetries      int
	retryDelay      time.Duration

	// Active runs for cancellation
	activeRuns map[string]context.CancelFunc
	runMu      sync.Mutex
}

// NewEngine creates a new workflow engine. If metricsRecorder is non-nil, direct
// LLM calls (workflow nodes with empty agent_id) are wrapped to report metrics.
func NewEngine(
	workflowRepo *repository.WorkflowRepository,
	executionRepo *ExecutionRepository,
	llmClient llm.Client,
	agentClient agentpb.AgentServiceClient,
	metricsRecorder MetricsRecorder,
) *Engine {
	wrappedLLM := llmClient
	if metricsRecorder != nil && llmClient != nil {
		wrappedLLM = llm.NewMetricsClient(llmClient, func(ctx context.Context, m *llm.CallMetrics) {
			metricsRecorder(ctx, m)
		}, "workflow")
	}
	return &Engine{
		workflowRepo:    workflowRepo,
		executionRepo:   executionRepo,
		llmClient:       wrappedLLM,
		agentClient:     agentClient,
		metricsRecorder: metricsRecorder,
		defaultTimeout:  5 * time.Minute,
		maxRetries:      2,
		retryDelay:      1 * time.Second,
		activeRuns:      make(map[string]context.CancelFunc),
	}
}

// SetDefaultTimeout configures the default workflow timeout
func (e *Engine) SetDefaultTimeout(d time.Duration) {
	e.defaultTimeout = d
}

// SetRetryPolicy configures retry behavior
func (e *Engine) SetRetryPolicy(maxRetries int, delay time.Duration) {
	e.maxRetries = maxRetries
	e.retryDelay = delay
}

// Execute runs a workflow with the given input
func (e *Engine) Execute(ctx context.Context, workflowID string, input string, tenantID string, timeoutMs int64) (*wfpkg.WorkflowResult, string, error) {
	// Load workflow from repository
	wfModel, err := e.workflowRepo.Get(ctx, workflowID)
	if err != nil {
		return nil, "", fmt.Errorf("get workflow: %w", err)
	}

	// Convert to workflow.Workflow struct
	wf, err := modelToWorkflow(wfModel)
	if err != nil {
		return nil, "", fmt.Errorf("parse workflow: %w", err)
	}

	// Validate the workflow DAG
	if err := wf.Validate(); err != nil {
		return nil, "", fmt.Errorf("workflow validation: %w", err)
	}

	// Create execution record
	execID := fmt.Sprintf("exec-%d", time.Now().UnixNano())
	execRecord := &ExecutionRecord{
		ID:         execID,
		WorkflowID: workflowID,
		Status:     string(StatusRunning),
		Input:      input,
		TenantID:   tenantID,
		StartedAt:  time.Now(),
	}
	if saveErr := e.executionRepo.Save(ctx, execRecord); saveErr != nil {
		return nil, "", fmt.Errorf("save execution record: %w", saveErr)
	}

	// Set up cancellable context with timeout
	var cancel context.CancelFunc
	if timeoutMs > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	} else if e.defaultTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, e.defaultTimeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	// Register for cancellation
	e.runMu.Lock()
	e.activeRuns[execID] = cancel
	e.runMu.Unlock()
	defer func() {
		e.runMu.Lock()
		delete(e.activeRuns, execID)
		e.runMu.Unlock()
	}()

	// Create the executor with our EngineFunc callback
	engineFunc := e.makeEngineFunc(execID, tenantID)
	executor := wfpkg.NewWorkflowExecutorWithConfig(nil, engineFunc, wfpkg.ExecutionConfig{
		NodeTimeout: 30 * time.Second,
		MaxRetries:  e.maxRetries,
		RetryDelay:  e.retryDelay,
	})

	// Execute the workflow
	result, err := executor.Execute(ctx, wf, input)

	// Update execution record
	now := time.Now()
	execRecord.CompletedAt = &now
	execRecord.Duration = time.Since(execRecord.StartedAt).Milliseconds()

	if err != nil {
		execRecord.Status = string(StatusFailed)
		if ctx.Err() == context.DeadlineExceeded {
			execRecord.Status = string(StatusTimedOut)
		}
		execRecord.Error = err.Error()
		if result != nil {
			execRecord.FinalOutput = result.FinalOutput
			execRecord.NodeResults = nodeResultsToJSON(result.Nodes)
		}
	} else {
		execRecord.Status = string(StatusCompleted)
		execRecord.FinalOutput = result.FinalOutput
		execRecord.NodeResults = nodeResultsToJSON(result.Nodes)
	}

	// Best-effort update of execution record
	_ = e.executionRepo.Save(ctx, execRecord)

	return result, execID, err
}

// CancelExecution cancels a running workflow execution
func (e *Engine) CancelExecution(ctx context.Context, executionID string) error {
	e.runMu.Lock()
	cancel, ok := e.activeRuns[executionID]
	e.runMu.Unlock()

	if !ok {
		return fmt.Errorf("execution %s not found or not running", executionID)
	}

	cancel()

	// Update execution record status
	execRecord, err := e.executionRepo.Get(ctx, executionID)
	if err != nil {
		return fmt.Errorf("get execution record: %w", err)
	}
	execRecord.Status = string(StatusCancelled)
	now := time.Now()
	execRecord.CompletedAt = &now
	return e.executionRepo.Save(ctx, execRecord)
}

// GetExecution retrieves an execution record
func (e *Engine) GetExecution(ctx context.Context, executionID string) (*ExecutionRecord, error) {
	return e.executionRepo.Get(ctx, executionID)
}

// ListExecutions lists execution records for a workflow
func (e *Engine) ListExecutions(ctx context.Context, workflowID string, limit int) ([]*ExecutionRecord, error) {
	return e.executionRepo.ListByWorkflow(ctx, workflowID, limit)
}

// ValidateWorkflow validates a workflow's DAG structure without saving
func (e *Engine) ValidateWorkflow(nodesJSON, edgesJSON, entryNodeID string) error {
	// Reuse modelToWorkflow's normalization by creating a temporary model
	tmpModel := &repository.WorkflowModel{
		Name:        "validation",
		EntryNodeID: entryNodeID,
		Nodes:       nodesJSON,
		Edges:       edgesJSON,
	}
	wf, err := modelToWorkflow(tmpModel)
	if err != nil {
		return err
	}
	return wf.Validate()
}

// makeEngineFunc creates the EngineFunc callback that routes execution
func (e *Engine) makeEngineFunc(sessionID, tenantID string) wfpkg.EngineFunc {
	return func(ctx context.Context, agentID string, input string) (string, error) {
		// If agentID is set, route to agent service
		if agentID != "" && e.agentClient != nil {
			resp, err := e.agentClient.Execute(ctx, &agentpb.ExecuteRequest{
				SessionId:  sessionID,
				TenantId:   tenantID,
				Message:    input,
				EntryAgent: agentID,
			})
			if err != nil {
				return "", fmt.Errorf("agent %s execution failed: %w", agentID, err)
			}
			return resp.Response, nil
		}

		// Fallback: direct LLM call
		if e.llmClient != nil {
			resp, err := e.llmClient.Chat(ctx, &llm.ChatRequest{
				Messages: []llm.Message{{Role: "user", Content: input}},
			})
			if err != nil {
				return "", fmt.Errorf("LLM call failed: %w", err)
			}
			return resp.Content, nil
		}

		return "", fmt.Errorf("no agent client or LLM client available")
	}
}

// reactFlowNode represents the raw JSON from the ReactFlow frontend.
// ReactFlow stores custom data in a nested "data" object and uses
// "source"/"target" for edges. The workflow executor expects a flat
// structure with "agent_id", "tool_name" etc. at node level and
// "from"/"to" on edges.
//
// Both shapes are accepted: the flat top-level fields (Name/AgentID/...)
// cover the app's own WorkflowNode contract, while the nested Data map
// covers native ReactFlow payloads. modelToWorkflow prefers top-level
// and falls back to Data.
type reactFlowNode struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Name      string                 `json:"name,omitempty"`
	AgentID   string                 `json:"agent_id,omitempty"`
	ToolName  string                 `json:"tool_name,omitempty"`
	Condition string                 `json:"condition,omitempty"`
	Position  *wfpkg.NodePosition    `json:"position"`
	Data      map[string]interface{} `json:"data"`
}

type reactFlowEdge struct {
	ID        string `json:"id"`
	Source    string `json:"source,omitempty"`
	Target    string `json:"target,omitempty"`
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	Label     string `json:"label,omitempty"`
	Condition string `json:"condition,omitempty"`
}

// modelToWorkflow converts a GORM WorkflowModel to a workflow.Workflow struct.
// It normalizes ReactFlow-format nodes (data nested) and edges (source/target)
// into the flat format expected by the workflow executor.
func modelToWorkflow(m *repository.WorkflowModel) (*wfpkg.Workflow, error) {
	wf := &wfpkg.Workflow{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		EntryNodeID: m.EntryNodeID,
		TenantID:    m.TenantID,
		CreatedAt:   m.CreatedAt.Unix(),
		UpdatedAt:   m.UpdatedAt.Unix(),
		Nodes:       make([]*wfpkg.Node, 0),
		Edges:       make([]*wfpkg.Edge, 0),
	}

	// Parse nodes — handle both ReactFlow format (data nested) and flat format
	var rfNodes []reactFlowNode
	if err := json.Unmarshal([]byte(m.Nodes), &rfNodes); err != nil {
		return nil, fmt.Errorf("parse nodes: %w", err)
	}
	for _, rfn := range rfNodes {
		node := &wfpkg.Node{
			ID:        rfn.ID,
			Type:      wfpkg.NodeType(rfn.Type),
			Position:  rfn.Position,
			Name:      rfn.Name,
			AgentID:   rfn.AgentID,
			ToolName:  rfn.ToolName,
			Condition: rfn.Condition,
		}
		// Fall back to ReactFlow nested data fields when flat fields are empty
		if rfn.Data != nil {
			if node.Name == "" {
				if v, ok := rfn.Data["label"].(string); ok {
					node.Name = v
				}
			}
			if node.AgentID == "" {
				if v, ok := rfn.Data["agent_id"].(string); ok {
					node.AgentID = v
				}
			}
			if node.ToolName == "" {
				if v, ok := rfn.Data["tool_name"].(string); ok {
					node.ToolName = v
				}
			}
			if node.Condition == "" {
				if v, ok := rfn.Data["condition"].(string); ok {
					node.Condition = v
				}
			}
			if v, ok := rfn.Data["config"].(map[string]interface{}); ok {
				node.Config = v
			}
		}
		wf.Nodes = append(wf.Nodes, node)
	}

	// Parse edges — handle ReactFlow source/target → from/to
	var rfEdges []reactFlowEdge
	if err := json.Unmarshal([]byte(m.Edges), &rfEdges); err != nil {
		return nil, fmt.Errorf("parse edges: %w", err)
	}
	for _, rfe := range rfEdges {
		// Accept both ReactFlow (source/target) and flat (from/to) edge shapes
		from := rfe.Source
		if from == "" {
			from = rfe.From
		}
		to := rfe.Target
		if to == "" {
			to = rfe.To
		}
		edge := &wfpkg.Edge{
			ID:        rfe.ID,
			From:      from,
			To:        to,
			Label:     rfe.Label,
			Condition: rfe.Condition,
		}
		wf.Edges = append(wf.Edges, edge)
	}

	return wf, nil
}

// nodeResultsToJSON converts NodeResult slice to JSON string
func nodeResultsToJSON(results []wfpkg.NodeResult) string {
	data, err := json.Marshal(results)
	if err != nil {
		return "[]"
	}
	return string(data)
}
