package workflow

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"google.golang.org/grpc"
	"gorm.io/gorm"

	"agent-platform/pkg/llm"
	agentpb "agent-platform/pkg/pb/agent"
	"agent-platform/services/harness-service/internal/repository"
)

// --- fakes ---

type fakeLLMClient struct {
	llm.Client
	fn func(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error)
}

func (f *fakeLLMClient) Chat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	return f.fn(ctx, req)
}

type fakeAgentClient struct {
	agentpb.AgentServiceClient
	fn func(ctx context.Context, req *agentpb.ExecuteRequest) (*agentpb.ExecuteResponse, error)
}

func (f *fakeAgentClient) Execute(ctx context.Context, req *agentpb.ExecuteRequest, opts ...grpc.CallOption) (*agentpb.ExecuteResponse, error) {
	return f.fn(ctx, req)
}

// --- helpers ---

func newTestEngine(t *testing.T, llmClient llm.Client, agentClient agentpb.AgentServiceClient) (*Engine, *repository.WorkflowRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlDB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1) // in-memory sqlite must share a single connection

	wfRepo := repository.NewWorkflowRepositoryWithDB(db)
	execRepo := NewExecutionRepositoryWithDB(db)
	eng := NewEngine(wfRepo, execRepo, llmClient, agentClient, nil)
	eng.SetDefaultTimeout(5 * time.Second)
	eng.SetRetryPolicy(0, 1*time.Millisecond) // no retries -> fast, deterministic tests
	return eng, wfRepo
}

// rfNode builds a ReactFlow-format node JSON (data nested).
func rfNodes(nodes ...map[string]interface{}) string {
	b, _ := json.Marshal(nodes)
	return string(b)
}

func rfEdges(edges ...map[string]interface{}) string {
	b, _ := json.Marshal(edges)
	return string(b)
}

func saveWorkflow(t *testing.T, repo *repository.WorkflowRepository, name, nodes, edges, entry string) string {
	t.Helper()
	wf := &repository.WorkflowModel{
		Name:        name,
		Nodes:       nodes,
		Edges:       edges,
		EntryNodeID: entry,
	}
	if err := repo.Save(context.Background(), wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}
	return wf.ID
}

// --- Execute ---

func TestEngine_Execute_LLMFallback(t *testing.T) {
	llmClient := &fakeLLMClient{
		fn: func(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
			return &llm.ChatResponse{Content: "llm-response"}, nil
		},
	}
	eng, repo := newTestEngine(t, llmClient, nil)

	nodes := rfNodes(map[string]interface{}{"id": "a", "type": "agent", "data": map[string]interface{}{"label": "A"}})
	wfID := saveWorkflow(t, repo, "llm-flow", nodes, "[]", "a")

	res, execID, err := eng.Execute(context.Background(), wfID, "hello", "tenant-1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.FinalOutput != "llm-response" {
		t.Errorf("final output = %q, want llm-response", res.FinalOutput)
	}
	if execID == "" {
		t.Error("expected non-empty execution ID")
	}

	// Execution record should be persisted as completed.
	exec, err := eng.GetExecution(context.Background(), execID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if exec.Status != string(StatusCompleted) {
		t.Errorf("execution status = %q, want completed", exec.Status)
	}
	if exec.WorkflowID != wfID {
		t.Errorf("execution workflow_id = %q, want %q", exec.WorkflowID, wfID)
	}
}

func TestEngine_Execute_AgentRouting(t *testing.T) {
	var gotReq *agentpb.ExecuteRequest
	agentClient := &fakeAgentClient{
		fn: func(ctx context.Context, req *agentpb.ExecuteRequest) (*agentpb.ExecuteResponse, error) {
			gotReq = req
			return &agentpb.ExecuteResponse{Response: "agent-response"}, nil
		},
	}
	eng, repo := newTestEngine(t, nil, agentClient)

	nodes := rfNodes(map[string]interface{}{
		"id":   "a",
		"type": "agent",
		"data": map[string]interface{}{"label": "A", "agent_id": "researcher"},
	})
	wfID := saveWorkflow(t, repo, "agent-flow", nodes, "[]", "a")

	res, _, err := eng.Execute(context.Background(), wfID, "do research", "tenant-1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.FinalOutput != "agent-response" {
		t.Errorf("final output = %q, want agent-response", res.FinalOutput)
	}
	if gotReq == nil {
		t.Fatal("agent client was not called")
	}
	if gotReq.EntryAgent != "researcher" {
		t.Errorf("EntryAgent = %q, want researcher", gotReq.EntryAgent)
	}
	if gotReq.Message != "do research" {
		t.Errorf("Message = %q, want %q", gotReq.Message, "do research")
	}
}

func TestEngine_ListExecutions_AfterExecute(t *testing.T) {
	llmClient := &fakeLLMClient{
		fn: func(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
			return &llm.ChatResponse{Content: "ok"}, nil
		},
	}
	eng, repo := newTestEngine(t, llmClient, nil)
	nodes := rfNodes(map[string]interface{}{"id": "a", "type": "agent", "data": map[string]interface{}{"label": "A"}})
	wfID := saveWorkflow(t, repo, "list-flow", nodes, "[]", "a")

	_, _, _ = eng.Execute(context.Background(), wfID, "in", "t", 0)

	executions, err := eng.ListExecutions(context.Background(), wfID, 10)
	if err != nil {
		t.Fatalf("list executions: %v", err)
	}
	if len(executions) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(executions))
	}
	if executions[0].Status != string(StatusCompleted) {
		t.Errorf("status = %q, want completed", executions[0].Status)
	}
}

// --- CancelExecution ---

func TestEngine_CancelExecution_NotRunning(t *testing.T) {
	eng, _ := newTestEngine(t, nil, nil)
	// No active run for this ID -> should error, not panic.
	err := eng.CancelExecution(context.Background(), "exec-does-not-exist")
	if err == nil {
		t.Fatal("expected error cancelling non-running execution, got nil")
	}
	if !strings.Contains(err.Error(), "not running") {
		t.Errorf("error = %q, want it to mention 'not running'", err.Error())
	}
}

// --- ValidateWorkflow ---

func TestEngine_ValidateWorkflow_Cycle(t *testing.T) {
	eng, _ := newTestEngine(t, nil, nil)
	nodes := rfNodes(
		map[string]interface{}{"id": "a", "type": "agent", "data": map[string]interface{}{"label": "A"}},
		map[string]interface{}{"id": "b", "type": "agent", "data": map[string]interface{}{"label": "B"}},
	)
	edges := rfEdges(
		map[string]interface{}{"id": "e1", "source": "a", "target": "b"},
		map[string]interface{}{"id": "e2", "source": "b", "target": "a"},
	)
	err := eng.ValidateWorkflow(nodes, edges, "a")
	if err == nil {
		t.Fatal("expected cycle validation error, got nil")
	}
}

// --- modelToWorkflow (ReactFlow normalization) ---

func TestModelToWorkflow_NormalizesReactFlow(t *testing.T) {
	nodes := rfNodes(
		map[string]interface{}{
			"id":       "n1",
			"type":     "agent",
			"position": map[string]interface{}{"x": 10.0, "y": 20.0},
			"data":     map[string]interface{}{"label": "First", "agent_id": "researcher"},
		},
		map[string]interface{}{
			"id":   "n2",
			"type": "tool",
			"data": map[string]interface{}{"label": "Search", "tool_name": "web_search"},
		},
	)
	edges := rfEdges(
		map[string]interface{}{"id": "e1", "source": "n1", "target": "n2", "label": "next"},
	)

	m := &repository.WorkflowModel{
		Name:        "rf",
		EntryNodeID: "n1",
		Nodes:       nodes,
		Edges:       edges,
	}
	wf, err := modelToWorkflow(m)
	if err != nil {
		t.Fatalf("modelToWorkflow: %v", err)
	}
	if len(wf.Nodes) != 2 || len(wf.Edges) != 1 {
		t.Fatalf("expected 2 nodes + 1 edge, got %d nodes / %d edges", len(wf.Nodes), len(wf.Edges))
	}
	// Node fields flattened from ReactFlow data
	n1 := wf.GetNode("n1")
	if n1 == nil {
		t.Fatal("n1 not found")
	}
	if n1.Name != "First" || n1.AgentID != "researcher" || n1.Type != "agent" {
		t.Errorf("n1 normalized wrong: %+v", n1)
	}
	if n1.Position == nil || n1.Position.X != 10 || n1.Position.Y != 20 {
		t.Errorf("n1 position wrong: %+v", n1.Position)
	}
	// Edge source/target -> from/to
	e := wf.Edges[0]
	if e.From != "n1" || e.To != "n2" {
		t.Errorf("edge from/to = %q/%q, want n1/n2", e.From, e.To)
	}
}
