// Package evaluate provides evaluation functionality
package evaluate

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"regexp"
	"strings"

	"agent-platform/pkg/llm"
)

// ScoreResult represents evaluation scores
type ScoreResult struct {
	Overall       float64 `json:"overall"`
	Faithfulness  float64 `json:"faithfulness"`
	Relevancy     float64 `json:"relevancy"`
	Precision     float64 `json:"precision"`
	Hallucination float64 `json:"hallucination"`
}

// EvalCase represents a test case
type EvalCase struct {
	ID       string   `json:"id"`
	SuiteID  string   `json:"suite_id"`
	Name     string   `json:"name"`
	Input    string   `json:"input"`
	Expected string   `json:"expected"`
	Tools    []string `json:"tools"`
}

// TestResult represents a test result
type TestResult struct {
	Name    string  `json:"name"`
	Score   float64 `json:"score"`
	Passed  bool    `json:"passed"`
	Output  string  `json:"output"`
	Error   string  `json:"error"`
}

// TestReport represents a test report
type TestReport struct {
	RunID     string        `json:"run_id"`
	SuiteID   string        `json:"suite_id"`
	Model     string        `json:"model"`
	AvgScore  float64       `json:"avg_score"`
	Results   []TestResult  `json:"results"`
	Timestamp int64         `json:"timestamp"`
}

// Scorer evaluates AI responses
type Scorer struct {
	llmClient  llm.Client
	llmModel   string
	llmEnabled bool
	fallback   bool
}

// NewScorer creates a new scorer with fallback
func NewScorer() *Scorer {
	return &Scorer{fallback: true}
}

// NewScorerWithLLM creates a scorer with LLM judge
func NewScorerWithLLM(client llm.Client, model string) *Scorer {
	return &Scorer{
		llmClient:  client,
		llmModel:   model,
		llmEnabled: true,
		fallback:   true,
	}
}

// Score evaluates the response quality
func (s *Scorer) Score(ctx context.Context, input, output, expected string) ScoreResult {
	if output == "" {
		return ScoreResult{}
	}

	if s.llmEnabled && s.llmClient != nil {
		result, err := s.scoreWithLLM(ctx, input, output, expected)
		if err == nil {
			return result
		}
		log.Printf("[harness:scorer] LLM scoring failed: %v, falling back to keywords", err)
	}
	return s.scoreWithKeywords(input, output, expected)
}

// scoreWithLLM uses LLM as judge
func (s *Scorer) scoreWithLLM(ctx context.Context, input, output, expected string) (ScoreResult, error) {
	prompt := fmt.Sprintf(`你是一个严格的 AI 回答质量评估专家。请对以下回答进行评分。

用户问题: %s

AI 回答: %s

参考答案: %s

请从以下四个维度分别打分（0-1的浮点数），并以 JSON 格式输出：
{"faithfulness": 0.8, "relevancy": 0.9, "precision": 0.7, "hallucination": 0.1}

评分标准：
- faithfulness: 回答与参考答案的一致性（关键信息是否匹配）
- relevancy: 回答与问题的相关性（是否偏题）
- precision: 回答的精确度和完整性（是否遗漏关键点）
- hallucination: 回答中包含参考答案之外的错误信息的程度（0=无幻觉，1=严重幻觉）

只输出 JSON，不要其他解释。`, input, output, expected)

	resp, err := s.llmClient.Chat(ctx, &llm.ChatRequest{
		Model:    s.llmModel,
		Messages: []llm.Message{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return ScoreResult{}, fmt.Errorf("LLM judge failed: %w", err)
	}

	result := parseScoreFromLLM(resp.Content)
	return result, nil
}

// parseScoreFromLLM parses LLM response into scores
func parseScoreFromLLM(content string) ScoreResult {
	jsonStr := extractJSON(content)
	if jsonStr == "" {
		return ScoreResult{Overall: 0.5, Faithfulness: 0.5, Relevancy: 0.5, Precision: 0.5, Hallucination: 0.5}
	}

	var scores struct {
		Faithfulness  float64 `json:"faithfulness"`
		Relevancy     float64 `json:"relevancy"`
		Precision     float64 `json:"precision"`
		Hallucination float64 `json:"hallucination"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &scores); err != nil {
		return ScoreResult{Overall: 0.5, Faithfulness: 0.5, Relevancy: 0.5, Precision: 0.5, Hallucination: 0.5}
	}

	overall := (scores.Faithfulness + scores.Relevancy + scores.Precision + (1.0 - scores.Hallucination)) / 4.0
	return ScoreResult{
		Overall:       clamp(overall, 0, 1),
		Faithfulness:  clamp(scores.Faithfulness, 0, 1),
		Relevancy:     clamp(scores.Relevancy, 0, 1),
		Precision:     clamp(scores.Precision, 0, 1),
		Hallucination: clamp(scores.Hallucination, 0, 1),
	}
}

// extractJSON extracts JSON from LLM response
func extractJSON(content string) string {
	re := regexp.MustCompile(`\{[^{}]*"faithfulness"[^{}]*"relevancy"[^{}]*"precision"[^{}]*"hallucination"[^{}]*\}`)
	match := re.FindString(content)
	return match
}

// scoreWithKeywords uses keyword matching as fallback
func (s *Scorer) scoreWithKeywords(input, output, expected string) ScoreResult {
	faithfulness := s.scoreFaithfulness(output, expected)
	relevancy := s.scoreRelevancy(output, input)
	precision := s.scorePrecision(output, expected)
	hallucination := s.scoreHallucination(output, expected)

	overall := (faithfulness + relevancy + precision + (1.0 - hallucination)) / 4.0

	return ScoreResult{
		Overall:       clamp(overall, 0, 1),
		Faithfulness:  clamp(faithfulness, 0, 1),
		Relevancy:     clamp(relevancy, 0, 1),
		Precision:     clamp(precision, 0, 1),
		Hallucination: clamp(hallucination, 0, 1),
	}
}

func (s *Scorer) scoreFaithfulness(output, expected string) float64 {
	if expected == "" {
		return 0.8
	}
	outputWords := tokenize(output)
	expectedWords := tokenize(expected)
	if len(expectedWords) == 0 {
		return 0.8
	}
	matchCount := 0
	for _, w := range outputWords {
		for _, ew := range expectedWords {
			if strings.EqualFold(w, ew) {
				matchCount++
				break
			}
		}
	}
	recall := float64(matchCount) / float64(len(expectedWords))
	return math.Min(recall*1.2, 1.0)
}

func (s *Scorer) scoreRelevancy(output, input string) float64 {
	if input == "" {
		return 0.8
	}
	inputWords := tokenize(input)
	if len(inputWords) == 0 {
		return 0.8
	}
	matchCount := 0
	for _, iw := range inputWords {
		if strings.Contains(strings.ToLower(output), strings.ToLower(iw)) {
			matchCount++
		}
	}
	ratio := float64(matchCount) / float64(len(inputWords))
	return math.Min(ratio*1.5, 1.0)
}

func (s *Scorer) scorePrecision(output, expected string) float64 {
	if expected == "" {
		return 0.7
	}
	outputWords := tokenize(output)
	expectedWords := tokenize(expected)
	if len(outputWords) == 0 || len(expectedWords) == 0 {
		return 0.7
	}
	matchCount := 0
	for _, ow := range outputWords {
		for _, ew := range expectedWords {
			if strings.EqualFold(ow, ew) {
				matchCount++
				break
			}
		}
	}
	precision := float64(matchCount) / float64(len(outputWords))
	return math.Min(precision*1.2, 1.0)
}

func (s *Scorer) scoreHallucination(output, expected string) float64 {
	if expected == "" {
		return 0.1
	}
	outputLen := len(strings.Fields(output))
	expectedLen := len(strings.Fields(expected))
	if outputLen == 0 || expectedLen == 0 {
		return 0.1
	}
	ratio := float64(outputLen) / float64(expectedLen)
	if ratio > 2.0 {
		return math.Min((ratio-2.0)/3.0, 1.0)
	}
	return 0.0
}

func tokenize(s string) []string {
	return strings.Fields(strings.ToLower(s))
}

func clamp(v, minV, maxV float64) float64 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

// Runner runs evaluation suites
type Runner struct {
	llmClient llm.Client
	scorer    *Scorer
	suiteRepo SuiteRepository
}

// SuiteRepository manages evaluation suites
type SuiteRepository interface {
	GetSuite(id string) (*EvalSuite, error)
	ListCases(suiteID string) ([]EvalCase, error)
}

// EvalSuite represents an evaluation suite
type EvalSuite struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   int64  `json:"created_at"`
}

// NewRunner creates a new evaluation runner
func NewRunner(llmClient llm.Client) *Runner {
	return &Runner{
		llmClient: llmClient,
		scorer:    NewScorerWithLLM(llmClient, ""),
	}
}

// Run runs evaluation on a suite
func (r *Runner) Run(ctx context.Context, suiteID, model string) ([]*EvalResult, float64, error) {
	// TODO: Load actual test cases from repository
	// For now, run with sample cases
	results := []*EvalResult{
		{
			CaseID: "case-1",
			Actual: "Sample response",
			Score:  8.5,
			Passed: true,
		},
	}

	// Calculate average score
	var total float64
	for _, res := range results {
		total += res.Score
	}
	avgScore := total / float64(len(results))

	return results, avgScore, nil
}

// EvalResult represents an evaluation result
type EvalResult struct {
	CaseID string  `json:"case_id"`
	Actual string  `json:"actual"`
	Score  float64 `json:"score"`
	Passed bool    `json:"passed"`
	Error  string  `json:"error"`
}

// Regression represents a regression issue
type Regression struct {
	CaseName string  `json:"case_name"`
	Before   float64 `json:"before"`
	After    float64 `json:"after"`
	Delta    float64 `json:"delta"`
}

// RegressionDetector detects regressions in test results
type RegressionDetector struct {
	threshold float64
	baseline  *TestReport
}

// NewRegressionDetector creates a new regression detector
func NewRegressionDetector(threshold float64) *RegressionDetector {
	if threshold <= 0 {
		threshold = 0.05
	}
	return &RegressionDetector{threshold: threshold}
}

// SetBaseline sets the baseline report
func (d *RegressionDetector) SetBaseline(report *TestReport) {
	d.baseline = report
}

// Check checks for regressions
func (d *RegressionDetector) Check(current, baseline *TestReport) []Regression {
	if baseline == nil || current == nil {
		return nil
	}

	baselineByName := make(map[string]TestResult)
	for _, r := range baseline.Results {
		baselineByName[r.Name] = r
	}

	var regressions []Regression
	for _, cur := range current.Results {
		base, ok := baselineByName[cur.Name]
		if !ok {
			continue
		}
		delta := cur.Score - base.Score
		if delta < -d.threshold {
			regressions = append(regressions, Regression{
				CaseName: cur.Name,
				Before:   base.Score,
				After:    cur.Score,
				Delta:    delta,
			})
		}
	}

	return regressions
}