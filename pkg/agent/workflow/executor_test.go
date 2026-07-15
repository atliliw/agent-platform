package workflow

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// --- test helpers ---

func agentNode(id string) *Node {
	return &Node{ID: id, Type: NodeAgent, Name: id, AgentID: id}
}

func edge(from, to, cond string) *Edge {
	return &Edge{ID: from + "->" + to, From: from, To: to, Condition: cond}
}

func wf(name, entry string, nodes []*Node, edges []*Edge) *Workflow {
	return &Workflow{ID: "wf-test", Name: name, EntryNodeID: entry, Nodes: nodes, Edges: edges}
}

// echoEngine returns input + "|" + agentID so we can trace the data flow.
func echoEngine(ctx context.Context, agentID, input string) (string, error) {
	return input + "|" + agentID, nil
}

// --- Execute ---

func TestExecute_LinearFlow(t *testing.T) {
	w := wf("linear", "a", []*Node{agentNode("a"), agentNode("b")}, []*Edge{edge("a", "b", "")})
	ex := NewWorkflowExecutor(nil, echoEngine)

	res, err := ex.Execute(context.Background(), w, "in")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// a: "in|a" -> b: "in|a|b"
	if res.FinalOutput != "in|a|b" {
		t.Fatalf("final output = %q, want %q", res.FinalOutput, "in|a|b")
	}
	if len(res.Nodes) != 2 {
		t.Fatalf("expected 2 node results, got %d", len(res.Nodes))
	}
	for _, n := range res.Nodes {
		if n.Status != "completed" {
			t.Errorf("node %s status = %q, want completed", n.NodeID, n.Status)
		}
	}
}

func TestExecute_Condition_TrueBranch(t *testing.T) {
	cond := &Node{ID: "c", Type: NodeCondition, Name: "c", Condition: "contains:hello"}
	w := wf("cond", "c",
		[]*Node{cond, agentNode("yes"), agentNode("no")},
		[]*Edge{edge("c", "yes", "true"), edge("c", "no", "false")},
	)
	ex := NewWorkflowExecutor(nil, echoEngine)

	res, err := ex.Execute(context.Background(), w, "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.FinalOutput != "hello world|yes" {
		t.Fatalf("final output = %q, want %q", res.FinalOutput, "hello world|yes")
	}
	ran := map[string]bool{}
	for _, n := range res.Nodes {
		ran[n.NodeID] = true
	}
	if !ran["yes"] || ran["no"] {
		t.Errorf("expected true branch 'yes' only, ran=%v", ran)
	}
}

func TestExecute_Condition_FalseBranch(t *testing.T) {
	cond := &Node{ID: "c", Type: NodeCondition, Name: "c", Condition: "contains:hello"}
	w := wf("cond", "c",
		[]*Node{cond, agentNode("yes"), agentNode("no")},
		[]*Edge{edge("c", "yes", "true"), edge("c", "no", "false")},
	)
	ex := NewWorkflowExecutor(nil, echoEngine)

	res, err := ex.Execute(context.Background(), w, "goodbye")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.FinalOutput != "goodbye|no" {
		t.Fatalf("final output = %q, want %q", res.FinalOutput, "goodbye|no")
	}
}

func TestExecute_MergeNode(t *testing.T) {
	// a -> merge -> c : merge collects a's output and forwards to c
	w := wf("merge", "a",
		[]*Node{agentNode("a"), {ID: "m", Type: NodeMerge, Name: "m"}, agentNode("c")},
		[]*Edge{edge("a", "m", ""), edge("m", "c", "")},
	)
	ex := NewWorkflowExecutor(nil, echoEngine)

	res, err := ex.Execute(context.Background(), w, "in")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// a: "in|a" -> merge collects "in|a" -> c: "in|a|c"
	if res.FinalOutput != "in|a|c" {
		t.Fatalf("final output = %q, want %q", res.FinalOutput, "in|a|c")
	}
}

func TestExecute_Timeout(t *testing.T) {
	w := wf("timeout", "a", []*Node{agentNode("a")}, nil)
	slow := func(ctx context.Context, agentID, input string) (string, error) {
		select {
		case <-time.After(300 * time.Millisecond):
			return "done", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	ex := NewWorkflowExecutorWithConfig(nil, slow, ExecutionConfig{NodeTimeout: 80 * time.Millisecond, MaxRetries: 0})

	res, err := ex.Execute(context.Background(), w, "in")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if len(res.Nodes) != 1 {
		t.Fatalf("expected 1 node result, got %d", len(res.Nodes))
	}
	if res.Nodes[0].Status != "timed_out" {
		t.Errorf("node status = %q, want timed_out", res.Nodes[0].Status)
	}
}

func TestExecute_Retry(t *testing.T) {
	w := wf("retry", "a", []*Node{agentNode("a")}, nil)
	var calls int32
	engine := func(ctx context.Context, agentID, input string) (string, error) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			return "", fmt.Errorf("transient %d", n)
		}
		return "ok", nil
	}
	ex := NewWorkflowExecutorWithConfig(nil, engine, ExecutionConfig{MaxRetries: 2, RetryDelay: 1 * time.Millisecond})

	res, err := ex.Execute(context.Background(), w, "in")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.FinalOutput != "ok" {
		t.Fatalf("final output = %q, want ok", res.FinalOutput)
	}
	if len(res.Nodes) != 1 || res.Nodes[0].Retries != 2 {
		t.Errorf("expected 1 node with 2 retries, got %+v", res.Nodes)
	}
	if calls != 3 {
		t.Errorf("expected 3 total calls (1 initial + 2 retries), got %d", calls)
	}
}

func TestExecute_RetryExhausted(t *testing.T) {
	w := wf("retry-ex", "a", []*Node{agentNode("a")}, nil)
	engine := func(ctx context.Context, agentID, input string) (string, error) {
		return "", errors.New("permanent")
	}
	ex := NewWorkflowExecutorWithConfig(nil, engine, ExecutionConfig{MaxRetries: 2, RetryDelay: 1 * time.Millisecond})

	res, err := ex.Execute(context.Background(), w, "in")
	if err == nil {
		t.Fatal("expected error after retries exhausted, got nil")
	}
	if len(res.Nodes) != 1 {
		t.Fatalf("expected 1 node result, got %d", len(res.Nodes))
	}
	if res.Nodes[0].Status != "failed" {
		t.Errorf("node status = %q, want failed", res.Nodes[0].Status)
	}
	if res.Nodes[0].Retries != 2 {
		t.Errorf("expected 2 retries recorded, got %d", res.Nodes[0].Retries)
	}
}

func TestExecute_ParallelBranches(t *testing.T) {
	// parallel -> a, b : both branches run, outputs combined
	par := &Node{ID: "p", Type: NodeParallel, Name: "p"}
	w := wf("parallel", "p",
		[]*Node{par, agentNode("a"), agentNode("b")},
		[]*Edge{edge("p", "a", ""), edge("p", "b", "")},
	)
	var ran int32
	engine := func(ctx context.Context, agentID, input string) (string, error) {
		atomic.AddInt32(&ran, 1)
		return input + "|" + agentID, nil
	}
	ex := NewWorkflowExecutor(nil, engine)

	res, err := ex.Execute(context.Background(), w, "in")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ran != 2 {
		t.Errorf("expected 2 branch executions, got %d", ran)
	}
	// Both "in|a" and "in|b" must be present in the combined output.
	if !contains(res.FinalOutput, "in|a") || !contains(res.FinalOutput, "in|b") {
		t.Errorf("final output %q missing branch outputs", res.FinalOutput)
	}
}

// --- Validate ---

func TestValidate_CycleDetected(t *testing.T) {
	w := wf("cyc", "a",
		[]*Node{agentNode("a"), agentNode("b")},
		[]*Edge{edge("a", "b", ""), edge("b", "a", "")},
	)
	if err := w.Validate(); err != ErrCycleDetected {
		t.Errorf("expected ErrCycleDetected, got %v", err)
	}
}

func TestValidate_DuplicateNodeID(t *testing.T) {
	w := &Workflow{
		ID:          "wf-dup",
		Name:        "dup",
		EntryNodeID: "a",
		Nodes:       []*Node{agentNode("a"), agentNode("a")},
		Edges:       nil,
	}
	if err := w.Validate(); err != ErrDuplicateNodeID {
		t.Errorf("expected ErrDuplicateNodeID, got %v", err)
	}
}

func TestValidate_EntryNotFound(t *testing.T) {
	w := &Workflow{
		ID:          "wf-ent",
		Name:        "ent",
		EntryNodeID: "missing",
		Nodes:       []*Node{agentNode("a")},
		Edges:       nil,
	}
	if err := w.Validate(); err != ErrEntryNodeNotFound {
		t.Errorf("expected ErrEntryNodeNotFound, got %v", err)
	}
}

// --- evaluateCondition ---

func TestEvaluateCondition(t *testing.T) {
	cases := []struct {
		name      string
		condition string
		input     string
		outputs   map[string]string
		want      bool
	}{
		{"contains match", "contains:hello", "hello world", nil, true},
		{"contains miss", "contains:hello", "goodbye", nil, false},
		{"equals", "equals:42", "42", nil, true},
		{"len_gt true", "len_gt:5", "hello world", nil, true},
		{"len_gt false", "len_gt:50", "short", nil, false},
		{"regex match", "regex:^hello", "hello world", nil, true},
		{"regex miss", "regex:^world", "hello world", nil, false},
		{"literal true", "true", "anything", nil, true},
		{"literal false", "false", "anything", nil, false},
		{"node ref operator", "nodes.n1.output contains:hello", "x", map[string]string{"n1": "hello there"}, true},
		{"node ref operator miss", "nodes.n1.output contains:zzz", "x", map[string]string{"n1": "hello there"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := evaluateCondition(c.condition, c.input, c.outputs)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("evaluateCondition(%q) = %v, want %v", c.condition, got, c.want)
			}
		})
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
