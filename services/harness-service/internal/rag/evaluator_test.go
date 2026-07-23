package rag

import (
	"context"
	"math"
	"strings"
	"testing"

	"agent-platform/pkg/llm"
)

// ---- pure metric function tests (lock the math) ----

func TestCalculateMRR(t *testing.T) {
	tests := []struct {
		name    string
		scores  []float64
		wantMRR float64
	}{
		{"first relevant at rank 1", []float64{1, 0, 0}, 1.0},
		{"first relevant at rank 2", []float64{0, 1, 0}, 0.5},
		{"first relevant at rank 3", []float64{0, 0, 1}, 1.0 / 3.0},
		{"no relevant doc", []float64{0, 0, 0}, 0.0},
		{"empty input", []float64{}, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateMRR(tt.scores)
			if math.Abs(got-tt.wantMRR) > 1e-9 {
				t.Errorf("calculateMRR(%v) = %v, want %v", tt.scores, got, tt.wantMRR)
			}
		})
	}
}

func TestCalculateNDCG(t *testing.T) {
	// Ideal ranking yields NDCG = 1.0.
	ideal := calculateNDCG([]float64{1, 1, 0}, 3)
	if math.Abs(ideal-1.0) > 1e-9 {
		t.Errorf("NDCG of ideal ranking = %v, want 1.0", ideal)
	}

	// [0,1,0]: DCG = 1/log2(3); IDCG (sorted [1,0,0]) = 1/log2(2) = 1.
	want := 1.0 / math.Log2(3)
	got := calculateNDCG([]float64{0, 1, 0}, 3)
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("NDCG([0,1,0],3) = %v, want %v", got, want)
	}

	// Empty input is safe and returns 0.
	if got := calculateNDCG([]float64{}, 0); got != 0.0 {
		t.Errorf("NDCG([],0) = %v, want 0", got)
	}
}

// ---- end-to-end loop test (lock the indentation fix in EvaluateAll) ----

// relevanceLLM is a stub llm.Client that answers checkRelevance prompts
// deterministically based on the "Context: <key>" line in the user message.
type relevanceLLM struct {
	relevant map[string]bool
}

func (m *relevanceLLM) Chat(_ context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	content := ""
	if len(req.Messages) > 0 {
		content = req.Messages[len(req.Messages)-1].Content
	}
	answer := "no"
	for key, rel := range m.relevant {
		if rel && strings.Contains(content, "Context: "+key) {
			answer = "yes"
			break
		}
	}
	return &llm.ChatResponse{Content: answer}, nil
}

func (m *relevanceLLM) ChatStream(_ context.Context, _ *llm.ChatRequest) (<-chan llm.ChatStreamChunk, error) {
	return nil, nil
}

func (m *relevanceLLM) Embed(_ context.Context, _ string) ([]float64, error) {
	return []float64{1.0}, nil
}

func (m *relevanceLLM) EmbedBatch(_ context.Context, _ []string) ([][]float64, error) {
	return [][]float64{{1.0}}, nil
}

// TestEvaluateAllMRRNDCGFromRelevance locks the EvaluateAll MRR/NDCG block, whose
// brace/indentation was previously misaligned so relevanceScores were populated
// incorrectly. With Answer and GroundTruth empty, the only LLM calls in the loop
// are checkRelevance per context, which the stub answers deterministically.
func TestEvaluateAllMRRNDCGFromRelevance(t *testing.T) {
	// Arrange
	stub := &relevanceLLM{relevant: map[string]bool{
		"golang-tutorial": true,
		"python-guide":    false,
		"golang-tips":     true,
	}}
	eval := NewRAGEvaluator(stub, nil, nil)

	// relevanceScores -> [1, 0, 1]
	// MRR  = 1/1 = 1.0 (first relevant at rank 1)
	// NDCG = (1/log2(2) + 0 + 1/log2(4)) / (1/log2(2) + 1/log2(3) + 0)
	//      = 1.5 / (1 + 1/log2(3))
	wantNDCG := 1.5 / (1.0 + 1.0/math.Log2(3))

	// Act
	result, err := eval.EvaluateAll(context.Background(), EvaluationRequest{
		Query:    "golang",
		Contexts: []string{"golang-tutorial", "python-guide", "golang-tips"},
		// Answer and GroundTruth intentionally empty to short-circuit LLM-heavy paths.
	})

	// Assert
	if err != nil {
		t.Fatalf("EvaluateAll returned error: %v", err)
	}
	if math.Abs(result.MRR-1.0) > 1e-9 {
		t.Errorf("MRR = %v, want 1.0", result.MRR)
	}
	if math.Abs(result.NDCG-wantNDCG) > 1e-9 {
		t.Errorf("NDCG = %v, want %v", result.NDCG, wantNDCG)
	}
	// Context precision: 2 of 3 contexts relevant.
	if math.Abs(result.ContextPrecision-(2.0/3.0)) > 1e-9 {
		t.Errorf("ContextPrecision = %v, want %v", result.ContextPrecision, 2.0/3.0)
	}
}
