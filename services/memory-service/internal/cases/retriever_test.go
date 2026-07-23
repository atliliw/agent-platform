package cases

import (
	"context"
	"strings"
	"testing"
)

// recordingLLM records the request handed to Chat and returns a fixed pattern
// JSON, so we can assert learnWithLLM now passes a real prompt (it previously
// called Chat(ctx, nil)).
type recordingLLM struct {
	receivedReq interface{}
	resp        string
}

func (m *recordingLLM) Embed(_ context.Context, _ string) ([]float64, error) {
	return []float64{1.0}, nil
}

func (m *recordingLLM) Chat(_ context.Context, req interface{}) (string, error) {
	m.receivedReq = req
	return m.resp, nil
}

// TestLearnWithLLMPassesPrompt locks the fix where learnWithLLM called
// Chat(ctx, nil) instead of forwarding the constructed prompt. The LLM must
// receive a non-nil request whose messages contain the analysis prompt.
func TestLearnWithLLMPassesPrompt(t *testing.T) {
	// Arrange
	lib := NewCaseLibrary(100)
	c := &Case{
		ID:       "case-1",
		Type:     CaseTypeSuccess,
		Status:   CaseStatusActive,
		Task:     "deploy service",
		Success:  true,
		ToolsUsed: []string{"kubectl"},
	}
	_ = lib.Store(context.Background(), c)

	stub := &recordingLLM{
		resp: `[{"name":"LLM模式","description":"d","conditions":["c"],"actions":["a"],"confidence":0.8}]`,
	}
	learner := NewCaseLearner(lib)
	learner.SetLLM(stub, "test-model")

	// Act
	_, err := learner.Learn(context.Background(), []string{"case-1"})

	// Assert
	if err != nil {
		t.Fatalf("Learn returned error: %v", err)
	}
	if stub.receivedReq == nil {
		t.Fatal("LLM Chat was called with nil request; learnWithLLM must forward the prompt")
	}
	req, ok := stub.receivedReq.(map[string]interface{})
	if !ok {
		t.Fatalf("Chat request has unexpected type %T", stub.receivedReq)
	}
	msgs, ok := req["messages"].([]map[string]string)
	if !ok {
		t.Fatalf("request messages missing or wrong type: %T", req["messages"])
	}
	if len(msgs) == 0 {
		t.Fatal("request messages empty; learnWithLLM must send the prompt as a user message")
	}
	if !strings.Contains(msgs[0]["content"], "分析以下案例") {
		t.Errorf("prompt not forwarded, message content = %q", msgs[0]["content"])
	}
	if !strings.Contains(msgs[0]["content"], "deploy service") {
		t.Errorf("prompt does not include case task, message content = %q", msgs[0]["content"])
	}
}

// TestLearnFallsBackToRulesWithoutLLM locks the nil-client fallback: when no LLM
// is set, Learn must not call an LLM and must still return rule-based patterns
// instead of panicking.
func TestLearnFallsBackToRulesWithoutLLM(t *testing.T) {
	// Arrange
	lib := NewCaseLibrary(100)
	for i := 0; i < 3; i++ {
		_ = lib.Store(context.Background(), &Case{
			ID:        string(rune('a' + i)),
			Type:      CaseTypeSuccess,
			Status:    CaseStatusActive,
			Task:      "search and summarize",
			Success:   true,
			ToolsUsed: []string{"search", "summarize"},
		})
	}
	learner := NewCaseLearner(lib) // no SetLLM -> llmClient is nil

	// Act
	patterns, err := learner.Learn(context.Background(), []string{"a", "b", "c"})

	// Assert
	if err != nil {
		t.Fatalf("Learn returned error: %v", err)
	}
	if len(patterns) == 0 {
		t.Fatal("expected rule-based patterns when LLM is nil, got none")
	}
	foundRulePattern := false
	for _, p := range patterns {
		if p.Name == "成功工具组合" { // produced by learnWithRules
			foundRulePattern = true
		}
	}
	if !foundRulePattern {
		t.Error("expected the rule-based '成功工具组合' pattern from learnWithRules fallback")
	}
}
