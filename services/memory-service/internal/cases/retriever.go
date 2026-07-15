// Package cases provides experience replay functionality
package cases

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ============================================================
// Case Types
// ============================================================

// CaseType defines the type of case
type CaseType string

const (
	CaseTypeSuccess CaseType = "success" // Successful execution
	CaseTypeFailure CaseType = "failure" // Failed execution
	CaseTypeMixed   CaseType = "mixed"   // Partial success
)

// CaseStatus defines the status of a case
type CaseStatus string

const (
	CaseStatusActive  CaseStatus = "active"  // Active case, used for learning
	CaseStatusArchived CaseStatus = "archived" // Archived case
	CaseStatusDeleted  CaseStatus = "deleted" // Deleted case
)

// ============================================================
// Case Definition
// ============================================================

// Case represents an experience case
type Case struct {
	ID          string                 `json:"id"`
	SessionID   string                 `json:"session_id"`
	AgentID     string                 `json:"agent_id"`
	Type        CaseType               `json:"type"`
	Status      CaseStatus             `json:"status"`
	Task        string                 `json:"task"`          // Task description
	Goal        string                 `json:"goal"`          // Goal description
	Outcome     string                 `json:"outcome"`       // Outcome description
	Success     bool                   `json:"success"`       // Whether successful
	Score       float64                `json:"score"`         // Success score (0-1)

	// Execution details
	Steps       []CaseStep             `json:"steps"`         // Execution steps
	ToolsUsed   []string               `json:"tools_used"`    // Tools used
	TokenCount  int                    `json:"token_count"`   // Total tokens used
	Duration    int64                  `json:"duration_ms"`   // Execution duration

	// Learning elements
	Lessons     []string               `json:"lessons"`       // Lessons learned
	Patterns    []string               `json:"patterns"`      // Patterns identified
	KeyActions  []string               `json:"key_actions"`   // Key actions that worked
	Mistakes    []string               `json:"mistakes"`      // Mistakes made

	// Metadata
	Tags        []string               `json:"tags"`          // Tags for categorization
	Category    string                 `json:"category"`      // Category
	Importance  float64                `json:"importance"`    // Importance score (0-1)
	Vector      []float64              `json:"vector"`        // Embedding for similarity

	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	AccessCount int                    `json:"access_count"`  // Times accessed

	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// CaseStep represents a step in a case
type CaseStep struct {
	StepNum     int                    `json:"step_num"`
	Action      string                 `json:"action"`
	Thought     string                 `json:"thought"`       // Agent's reasoning
	ToolName    string                 `json:"tool_name"`
	Input       map[string]interface{} `json:"input"`
	Output      interface{}            `json:"output"`
	Success     bool                   `json:"success"`
	Error       string                 `json:"error,omitempty"`
	Duration    int64                  `json:"duration_ms"`
}

// ============================================================
// Case Library
// ============================================================

// CaseLibrary manages the collection of cases
type CaseLibrary struct {
	cases        map[string]*Case     // ID -> Case
	byAgent      map[string][]string  // AgentID -> Case IDs
	byCategory   map[string][]string  // Category -> Case IDs
	byTag        map[string][]string  // Tag -> Case IDs
	byType       map[CaseType][]string // Type -> Case IDs
	successCases []string             // IDs of success cases
	failureCases []string             // IDs of failure cases
	maxCases     int                  // Maximum cases to store
	mu           sync.RWMutex
}

// NewCaseLibrary creates a new case library
func NewCaseLibrary(maxCases int) *CaseLibrary {
	if maxCases <= 0 {
		maxCases = 10000
	}
	return &CaseLibrary{
		cases:        make(map[string]*Case),
		byAgent:      make(map[string][]string),
		byCategory:   make(map[string][]string),
		byTag:        make(map[string][]string),
		byType:       make(map[CaseType][]string),
		successCases: make([]string, 0),
		failureCases: make([]string, 0),
		maxCases:     maxCases,
	}
}

// Store stores a new case
func (l *CaseLibrary) Store(ctx context.Context, case_ *Case) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Check capacity
	if len(l.cases) >= l.maxCases {
		l.evictOldest()
	}

	// Generate ID if not set
	if case_.ID == "" {
		case_.ID = generateCaseID()
	}

	// Set timestamps
	now := time.Now()
	if case_.CreatedAt.IsZero() {
		case_.CreatedAt = now
	}
	case_.UpdatedAt = now

	// Store case
	l.cases[case_.ID] = case_

	// Update indexes
	l.byAgent[case_.AgentID] = append(l.byAgent[case_.AgentID], case_.ID)
	l.byCategory[case_.Category] = append(l.byCategory[case_.Category], case_.ID)
	l.byType[case_.Type] = append(l.byType[case_.Type], case_.ID)

	for _, tag := range case_.Tags {
		l.byTag[tag] = append(l.byTag[tag], case_.ID)
	}

	if case_.Success {
		l.successCases = append(l.successCases, case_.ID)
	} else {
		l.failureCases = append(l.failureCases, case_.ID)
	}

	return nil
}

// Get retrieves a case by ID
func (l *CaseLibrary) Get(ctx context.Context, id string) (*Case, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	case_, ok := l.cases[id]
	if !ok {
		return nil, fmt.Errorf("case not found: %s", id)
	}

	// Increment access count
	l.mu.RUnlock()
	l.mu.Lock()
	case_.AccessCount++
	l.mu.Unlock()
	l.mu.RLock()

	return case_, nil
}

// GetByAgent retrieves cases by agent
func (l *CaseLibrary) GetByAgent(ctx context.Context, agentID string) []*Case {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []*Case
	for _, id := range l.byAgent[agentID] {
		if c, ok := l.cases[id]; ok {
			result = append(result, c)
		}
	}
	return result
}

// GetSuccessCases retrieves all success cases
func (l *CaseLibrary) GetSuccessCases(ctx context.Context) []*Case {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []*Case
	for _, id := range l.successCases {
		if c, ok := l.cases[id]; ok {
			result = append(result, c)
		}
	}
	return result
}

// GetFailureCases retrieves all failure cases
func (l *CaseLibrary) GetFailureCases(ctx context.Context) []*Case {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []*Case
	for _, id := range l.failureCases {
		if c, ok := l.cases[id]; ok {
			result = append(result, c)
		}
	}
	return result
}

// GetByCategory retrieves cases by category
func (l *CaseLibrary) GetByCategory(ctx context.Context, category string) []*Case {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []*Case
	for _, id := range l.byCategory[category] {
		if c, ok := l.cases[id]; ok {
			result = append(result, c)
		}
	}
	return result
}

// GetByTags retrieves cases by tags
func (l *CaseLibrary) GetByTags(ctx context.Context, tags []string) []*Case {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Get cases that match any of the tags
	matchedIDs := make(map[string]bool)
	for _, tag := range tags {
		for _, id := range l.byTag[tag] {
			matchedIDs[id] = true
		}
	}

	var result []*Case
	for id := range matchedIDs {
		if c, ok := l.cases[id]; ok {
			result = append(result, c)
		}
	}
	return result
}

// Delete deletes a case
func (l *CaseLibrary) Delete(ctx context.Context, id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	case_, ok := l.cases[id]
	if !ok {
		return fmt.Errorf("case not found: %s", id)
	}

	// Remove from indexes
	l.removeFromIndexes(case_)

	// Remove from main storage
	delete(l.cases, id)

	return nil
}

// Archive archives a case
func (l *CaseLibrary) Archive(ctx context.Context, id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	case_, ok := l.cases[id]
	if !ok {
		return fmt.Errorf("case not found: %s", id)
	}

	case_.Status = CaseStatusArchived
	case_.UpdatedAt = time.Now()

	return nil
}

// removeFromIndexes removes a case from all indexes
func (l *CaseLibrary) removeFromIndexes(case_ *Case) {
	l.byAgent[case_.AgentID] = removeFromSlice(l.byAgent[case_.AgentID], case_.ID)
	l.byCategory[case_.Category] = removeFromSlice(l.byCategory[case_.Category], case_.ID)
	l.byType[case_.Type] = removeFromSlice(l.byType[case_.Type], case_.ID)

	for _, tag := range case_.Tags {
		l.byTag[tag] = removeFromSlice(l.byTag[tag], case_.ID)
	}

	if case_.Success {
		l.successCases = removeFromSlice(l.successCases, case_.ID)
	} else {
		l.failureCases = removeFromSlice(l.failureCases, case_.ID)
	}
}

// evictOldest removes the oldest cases
func (l *CaseLibrary) evictOldest() {
	// Remove 10% of oldest cases
	removeCount := l.maxCases / 10
	if removeCount < 1 {
		removeCount = 1
	}

	// Find oldest cases
	var oldestIDs []string
	var oldestTimes []time.Time

	for id, c := range l.cases {
		oldestIDs = append(oldestIDs, id)
		oldestTimes = append(oldestTimes, c.CreatedAt)
	}

	// Sort by time
	for i := 0; i < len(oldestTimes)-1; i++ {
		for j := i + 1; j < len(oldestTimes); j++ {
			if oldestTimes[j].Before(oldestTimes[i]) {
				oldestTimes[i], oldestTimes[j] = oldestTimes[j], oldestTimes[i]
				oldestIDs[i], oldestIDs[j] = oldestIDs[j], oldestIDs[i]
			}
		}
	}

	// Remove oldest
	for i := 0; i < removeCount && i < len(oldestIDs); i++ {
		id := oldestIDs[i]
		if c, ok := l.cases[id]; ok {
			l.removeFromIndexes(c)
			delete(l.cases, id)
		}
	}
}

// GetStatistics returns library statistics
func (l *CaseLibrary) GetStatistics() map[string]interface{} {
	l.mu.RLock()
	defer l.mu.RUnlock()

	successRate := 0.0
	if len(l.cases) > 0 {
		successRate = float64(len(l.successCases)) / float64(len(l.cases))
	}

	stats := map[string]interface{}{
		"total_cases":     len(l.cases),
		"success_cases":   len(l.successCases),
		"failure_cases":   len(l.failureCases),
		"success_rate":    successRate,
		"by_agent_count":  len(l.byAgent),
		"by_category_count": len(l.byCategory),
	}

	return stats
}

// ============================================================
// Similar Case Retriever
// ============================================================

// SimilarCaseResult represents a similar case match
type SimilarCaseResult struct {
	Case        *Case   `json:"case"`
	Similarity  float64 `json:"similarity"` // 0-1 similarity score
	MatchType   string  `json:"match_type"` // vector, keyword, pattern
}

// CaseRetriever retrieves similar cases
type CaseRetriever struct {
	library   *CaseLibrary
	llmClient LLMClient
	llmModel  string
}

// LLMClient interface for LLM operations
type LLMClient interface {
	Embed(ctx context.Context, text string) ([]float64, error)
	Chat(ctx context.Context, req interface{}) (string, error)
}

// NewCaseRetriever creates a new case retriever
func NewCaseRetriever(library *CaseLibrary) *CaseRetriever {
	return &CaseRetriever{
		library: library,
	}
}

// SetLLM sets LLM client
func (r *CaseRetriever) SetLLM(client LLMClient, model string) {
	r.llmClient = client
	r.llmModel = model
}

// Retrieve retrieves similar cases based on a query
func (r *CaseRetriever) Retrieve(ctx context.Context, query string, topK int) ([]*SimilarCaseResult, error) {
	// Get query embedding if LLM available
	var queryVector []float64
	if r.llmClient != nil {
		vector, err := r.llmClient.Embed(ctx, query)
		if err == nil {
			queryVector = vector
		}
	}

	// Search cases
	r.library.mu.RLock()
	defer r.library.mu.RUnlock()

	type scored struct {
		case_      *Case
		similarity float64
		matchType  string
	}

	var scoredCases []scored

	for _, case_ := range r.library.cases {
		if case_.Status != CaseStatusActive {
			continue
		}

		similarity := 0.0
		matchType := "keyword"

		// Vector similarity if available
		if queryVector != nil && case_.Vector != nil && len(queryVector) == len(case_.Vector) {
			similarity = cosineSimilarity(queryVector, case_.Vector)
			matchType = "vector"
		} else {
			// Keyword similarity
			similarity = keywordSimilarity(query, case_)
			matchType = "keyword"
		}

		scoredCases = append(scoredCases, scored{
			case_:      case_,
			similarity: similarity,
			matchType:  matchType,
		})
	}

	// Sort by similarity
	for i := 0; i < len(scoredCases)-1; i++ {
		for j := i + 1; j < len(scoredCases); j++ {
			if scoredCases[j].similarity > scoredCases[i].similarity {
				scoredCases[i], scoredCases[j] = scoredCases[j], scoredCases[i]
			}
		}
	}

	// Return top K
	result := make([]*SimilarCaseResult, 0, topK)
	for i := 0; i < topK && i < len(scoredCases); i++ {
		result = append(result, &SimilarCaseResult{
			Case:       scoredCases[i].case_,
			Similarity: scoredCases[i].similarity,
			MatchType:  scoredCases[i].matchType,
		})
	}

	return result, nil
}

// RetrieveByTask retrieves cases similar to a task
func (r *CaseRetriever) RetrieveByTask(ctx context.Context, task string, topK int, preferSuccess bool) ([]*SimilarCaseResult, error) {
	results, err := r.Retrieve(ctx, task, topK*2)
	if err != nil {
		return nil, err
	}

	// Filter by success if preferred
	if preferSuccess {
		var filtered []*SimilarCaseResult
		for _, result := range results {
			if result.Case.Success {
				filtered = append(filtered, result)
			}
		}
		if len(filtered) >= topK {
			return filtered[:topK], nil
		}
		// Add failure cases if not enough success cases
		for _, result := range results {
			if !result.Case.Success {
				filtered = append(filtered, result)
			}
			if len(filtered) >= topK {
				break
			}
		}
		return filtered, nil
	}

	return results[:min(topK, len(results))], nil
}

// RetrieveByPattern retrieves cases matching a pattern
func (r *CaseRetriever) RetrieveByPattern(ctx context.Context, pattern string, topK int) ([]*SimilarCaseResult, error) {
	r.library.mu.RLock()
	defer r.library.mu.RUnlock()

	var results []*SimilarCaseResult

	for _, case_ := range r.library.cases {
		if case_.Status != CaseStatusActive {
			continue
		}

		// Check if pattern matches any of the case patterns
		for _, p := range case_.Patterns {
			if strings.Contains(strings.ToLower(p), strings.ToLower(pattern)) {
				results = append(results, &SimilarCaseResult{
					Case:       case_,
					Similarity: 1.0,
					MatchType:  "pattern",
				})
				break
			}
		}

		if len(results) >= topK {
			break
		}
	}

	return results, nil
}

// cosineSimilarity calculates cosine similarity between vectors
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// keywordSimilarity calculates keyword-based similarity
func keywordSimilarity(query string, case_ *Case) float64 {
	queryWords := tokenize(query)
	var matchCount int

	// Match against task, goal, outcome
	taskWords := tokenize(case_.Task)
	goalWords := tokenize(case_.Goal)
	outcomeWords := tokenize(case_.Outcome)

	for _, qw := range queryWords {
		for _, tw := range taskWords {
			if qw == tw {
				matchCount++
			}
		}
		for _, gw := range goalWords {
			if qw == gw {
				matchCount++
			}
		}
		for _, ow := range outcomeWords {
			if qw == ow {
				matchCount++
			}
		}
	}

	totalWords := len(queryWords) + len(taskWords) + len(goalWords) + len(outcomeWords)
	if totalWords == 0 {
		return 0
	}

	return float64(matchCount) / float64(totalWords)
}

// ============================================================
// Case Learner
// ============================================================

// LearningPattern represents a learned pattern
type LearningPattern struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Conditions  []string `json:"conditions"` // Conditions that trigger this pattern
	Actions     []string `json:"actions"`    // Recommended actions
	Confidence  float64  `json:"confidence"` // Pattern confidence
	Occurrences int      `json:"occurrences"` // Times pattern occurred
	CreatedAt   time.Time `json:"created_at"`
}

// CaseLearner learns patterns from cases
type CaseLearner struct {
	library   *CaseLibrary
	patterns  map[string]*LearningPattern
	llmClient LLMClient
	llmModel  string
	mu        sync.RWMutex
}

// NewCaseLearner creates a new case learner
func NewCaseLearner(library *CaseLibrary) *CaseLearner {
	return &CaseLearner{
		library:  library,
		patterns: make(map[string]*LearningPattern),
	}
}

// SetLLM sets LLM client
func (l *CaseLearner) SetLLM(client LLMClient, model string) {
	l.llmClient = client
	l.llmModel = model
}

// Learn learns patterns from cases
func (l *CaseLearner) Learn(ctx context.Context, caseIDs []string) ([]*LearningPattern, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Get cases
	var cases []*Case
	for _, id := range caseIDs {
		if c, ok := l.library.cases[id]; ok {
			cases = append(cases, c)
		}
	}

	if len(cases) == 0 {
		return nil, fmt.Errorf("no cases to learn from")
	}

	// Extract patterns
	var patterns []*LearningPattern

	// Use LLM if available
	if l.llmClient != nil {
		llmPatterns, err := l.learnWithLLM(ctx, cases)
		if err == nil {
			patterns = llmPatterns
		}
	}

	// Use rule-based learning as fallback or supplement
	rulePatterns := l.learnWithRules(cases)
	patterns = mergePatterns(patterns, rulePatterns)

	// Store patterns
	for _, pattern := range patterns {
		if pattern.ID == "" {
			pattern.ID = generatePatternID()
			pattern.CreatedAt = time.Now()
		}
		l.patterns[pattern.ID] = pattern
	}

	return patterns, nil
}

// learnWithLLM uses LLM to learn patterns
func (l *CaseLearner) learnWithLLM(ctx context.Context, cases []*Case) ([]*LearningPattern, error) {
	// Build prompt
	var prompt strings.Builder
	prompt.WriteString("分析以下案例，提取成功和失败的模式：\n\n")

	for i, c := range cases {
		prompt.WriteString(fmt.Sprintf("案例%d:\n", i+1))
		prompt.WriteString(fmt.Sprintf("- 任务: %s\n", c.Task))
		prompt.WriteString(fmt.Sprintf("- 成功: %v\n", c.Success))
		prompt.WriteString(fmt.Sprintf("- 关键行动: %v\n", c.KeyActions))
		prompt.WriteString(fmt.Sprintf("- 错误: %v\n", c.Mistakes))
		prompt.WriteString(fmt.Sprintf("- 经验: %v\n", c.Lessons))
		prompt.WriteString("\n")
	}

	prompt.WriteString("请提取模式，以JSON格式输出：\n")
	prompt.WriteString("[{\"name\": \"模式名\", \"description\": \"描述\", \"conditions\": [\"条件1\"], \"actions\": [\"行动1\"], \"confidence\": 0.8}]")

	// Call LLM
	resp, err := l.llmClient.Chat(ctx, nil) // Placeholder
	if err != nil {
		return nil, err
	}

	// Parse response
	var patterns []*LearningPattern
	jsonStr := extractJSON(resp)
	if jsonStr != "" {
		json.Unmarshal([]byte(jsonStr), &patterns)
	}

	return patterns, nil
}

// learnWithRules uses rules to learn patterns
func (l *CaseLearner) learnWithRules(cases []*Case) []*LearningPattern {
	var patterns []*LearningPattern

	// Group by outcome
	successCases := make([]*Case, 0)
	failureCases := make([]*Case, 0)

	for _, c := range cases {
		if c.Success {
			successCases = append(successCases, c)
		} else {
			failureCases = append(failureCases, c)
		}
	}

	// Find common patterns in success cases
	if len(successCases) >= 3 {
		commonTools := findCommonTools(successCases)
		if len(commonTools) > 0 {
			patterns = append(patterns, &LearningPattern{
				ID:          generatePatternID(),
				Name:        "成功工具组合",
				Description: fmt.Sprintf("成功案例中常用的工具: %v", commonTools),
				Conditions:  []string{"需要完成类似任务"},
				Actions:     commonTools,
				Confidence:  float64(len(successCases)) / float64(len(cases)),
				Occurrences: len(successCases),
			})
		}
	}

	// Find common mistakes in failure cases
	if len(failureCases) >= 3 {
		commonMistakes := findCommonMistakes(failureCases)
		if len(commonMistakes) > 0 {
			patterns = append(patterns, &LearningPattern{
				ID:          generatePatternID(),
				Name:        "常见失败模式",
				Description: fmt.Sprintf("失败案例中常见的错误: %v", commonMistakes),
				Conditions:  []string{"遇到类似情况"},
				Actions:     []string{"避免这些错误"},
				Confidence:  float64(len(failureCases)) / float64(len(cases)),
				Occurrences: len(failureCases),
			})
		}
	}

	return patterns
}

// GetPatterns retrieves all learned patterns
func (l *CaseLearner) GetPatterns() []*LearningPattern {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var patterns []*LearningPattern
	for _, p := range l.patterns {
		patterns = append(patterns, p)
	}
	return patterns
}

// GetMatchingPatterns retrieves patterns matching conditions
func (l *CaseLearner) GetMatchingPatterns(conditions []string) []*LearningPattern {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var matching []*LearningPattern
	for _, p := range l.patterns {
		for _, cond := range p.Conditions {
			for _, inputCond := range conditions {
				if strings.Contains(strings.ToLower(cond), strings.ToLower(inputCond)) {
					matching = append(matching, p)
					break
				}
			}
		}
	}
	return matching
}

// ============================================================
// Helper Functions
// ============================================================

func generateCaseID() string {
	return fmt.Sprintf("case-%d", time.Now().UnixNano())
}

func generatePatternID() string {
	return fmt.Sprintf("pattern-%d", time.Now().UnixNano())
}

func removeFromSlice(slice []string, item string) []string {
	var result []string
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

func tokenize(s string) []string {
	return strings.Fields(strings.ToLower(s))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func findCommonTools(cases []*Case) []string {
	toolCounts := make(map[string]int)
	for _, c := range cases {
		for _, tool := range c.ToolsUsed {
			toolCounts[tool]++
		}
	}

	var common []string
	for tool, count := range toolCounts {
		if count >= len(cases)/2 {
			common = append(common, tool)
		}
	}
	return common
}

func findCommonMistakes(cases []*Case) []string {
	mistakeCounts := make(map[string]int)
	for _, c := range cases {
		for _, mistake := range c.Mistakes {
			mistakeCounts[mistake]++
		}
	}

	var common []string
	for mistake, count := range mistakeCounts {
		if count >= len(cases)/2 {
			common = append(common, mistake)
		}
	}
	return common
}

func mergePatterns(a, b []*LearningPattern) []*LearningPattern {
	// Simple merge - would need deduplication in production
	return append(a, b...)
}

func extractJSON(content string) string {
	re := regexp.MustCompile(`\[[\s\S]*?\]|\{[\s\S]*?\}`)
	return re.FindString(content)
}