// Package rag provides RAG evaluation functionality
package rag

import (
	"context"
	"fmt"
	"sync"
	"time"

	"agent-platform/pkg/llm"
)

// RAGEvaluator evaluates RAG system performance
type RAGEvaluator struct {
	llmClient llm.Client
	repo      *Repository
}

// NewRAGEvaluator creates a new RAG evaluator
func NewRAGEvaluator(llmClient llm.Client, repo *Repository) *RAGEvaluator {
	return &RAGEvaluator{
		llmClient: llmClient,
		repo:      repo,
	}
}

// EvaluateContextPrecision calculates context precision
// Context Precision measures if all retrieved contexts are relevant to the query
func (e *RAGEvaluator) EvaluateContextPrecision(ctx context.Context, query string, contexts []string) (float64, error) {
	if len(contexts) == 0 {
		return 0.0, nil
	}

	// Use LLM to check relevance of each context
	relevantCount := 0
	for _, context := range contexts {
		isRelevant, err := e.checkRelevance(ctx, query, context)
		if err != nil {
			// On error, skip this context
			continue
		}
		if isRelevant {
			relevantCount++
		}
	}

	return float64(relevantCount) / float64(len(contexts)), nil
}

// EvaluateContextRecall calculates context recall
// Context Recall measures if all ground truth contexts were retrieved
func (e *RAGEvaluator) EvaluateContextRecall(ctx context.Context, groundTruthContexts []string, retrievedContexts []string) (float64, error) {
	if len(groundTruthContexts) == 0 {
		return 1.0, nil // No ground truth means full recall
	}

	// Count how many ground truth contexts appear in retrieved contexts
	matchedCount := 0
	for _, gt := range groundTruthContexts {
		for _, retrieved := range retrievedContexts {
			similar, err := e.checkContextSimilarity(ctx, gt, retrieved)
			if err != nil {
				continue
			}
			if similar >= 0.8 { // 80% similarity threshold
				matchedCount++
				break
			}
		}
	}

	return float64(matchedCount) / float64(len(groundTruthContexts)), nil
}

// EvaluateContextRelevancy calculates context relevancy
// Context Relevancy measures the proportion of relevant sentences in retrieved contexts
func (e *RAGEvaluator) EvaluateContextRelevancy(ctx context.Context, query string, contexts []string) (float64, error) {
	if len(contexts) == 0 {
		return 0.0, nil
	}

	// Extract sentences from contexts and check relevance
	totalSentences := 0
	relevantSentences := 0

	for _, context := range contexts {
		sentences := e.extractSentences(context)
		totalSentences += len(sentences)

		for _, sentence := range sentences {
			isRelevant, err := e.checkSentenceRelevance(ctx, query, sentence)
			if err != nil {
				continue
			}
			if isRelevant {
				relevantSentences++
			}
		}
	}

	if totalSentences == 0 {
		return 0.5, nil
	}

	return float64(relevantSentences) / float64(totalSentences), nil
}

// EvaluateFaithfulness calculates faithfulness score
// Faithfulness measures if the generated answer is derived from the retrieved contexts
func (e *RAGEvaluator) EvaluateFaithfulness(ctx context.Context, answer string, contexts []string) (float64, error) {
	if answer == "" {
		return 0.0, nil
	}
	if len(contexts) == 0 {
		return 0.0, nil // No context means cannot verify faithfulness
	}

	// Extract claims from answer and verify each against contexts
	claims, err := e.extractClaims(ctx, answer)
	if err != nil {
		return 0.5, nil // Default score on error
	}

	if len(claims) == 0 {
		return 1.0, nil // No claims means nothing to verify
	}

	verifiedCount := 0
	for _, claim := range claims {
		supported, err := e.verifyClaim(ctx, claim, contexts)
		if err != nil {
			continue
		}
		if supported {
			verifiedCount++
		}
	}

	return float64(verifiedCount) / float64(len(claims)), nil
}

// EvaluateAnswerRelevancy calculates answer relevancy
// Answer Relevancy measures if the answer addresses the query
func (e *RAGEvaluator) EvaluateAnswerRelevancy(ctx context.Context, query string, answer string) (float64, error) {
	if answer == "" {
		return 0.0, nil
	}
	if query == "" {
		return 0.0, nil
	}

	// Generate questions from answer
	questions, err := e.generateQuestionsFromAnswer(ctx, answer)
	if err != nil {
		return 0.5, nil
	}

	if len(questions) == 0 {
		return 0.5, nil
	}

	// Calculate similarity between original query and generated questions
	queryEmbedding, err := e.llmClient.Embed(ctx, query)
	if err != nil {
		return 0.5, nil
	}

	totalSimilarity := 0.0
	validCount := 0

	for _, q := range questions {
		qEmbedding, err := e.llmClient.Embed(ctx, q)
		if err != nil {
			continue
		}

		similarity, err := calculateSemanticSimilarity(queryEmbedding, qEmbedding)
		if err != nil {
			continue
		}

		totalSimilarity += similarity
		validCount++
	}

	if validCount == 0 {
		return 0.5, nil
	}

	return totalSimilarity / float64(validCount), nil
}

// EvaluateAnswerCorrectness calculates answer correctness
// Answer Correctness compares the generated answer with ground truth
func (e *RAGEvaluator) EvaluateAnswerCorrectness(ctx context.Context, answer string, groundTruth string) (float64, error) {
	if answer == "" || groundTruth == "" {
		return 0.0, nil
	}

	// Calculate semantic similarity between answer and ground truth
	answerEmbedding, err := e.llmClient.Embed(ctx, answer)
	if err != nil {
		return 0.0, fmt.Errorf("embed answer: %w", err)
	}

	groundTruthEmbedding, err := e.llmClient.Embed(ctx, groundTruth)
	if err != nil {
		return 0.0, fmt.Errorf("embed ground truth: %w", err)
	}

	similarity, err := calculateSemanticSimilarity(answerEmbedding, groundTruthEmbedding)
	if err != nil {
		return 0.0, fmt.Errorf("calculate similarity: %w", err)
	}

	return similarity, nil
}

// EvaluateAnswerSimilarity calculates answer similarity using embeddings
func (e *RAGEvaluator) EvaluateAnswerSimilarity(ctx context.Context, answer1, answer2 string) (float64, error) {
	if answer1 == "" || answer2 == "" {
		return 0.0, nil
	}

	embeddings, err := e.llmClient.EmbedBatch(ctx, []string{answer1, answer2})
	if err != nil {
		return 0.0, fmt.Errorf("embed answers: %w", err)
	}

	if len(embeddings) < 2 {
		return 0.0, fmt.Errorf("insufficient embeddings")
	}

	return calculateSemanticSimilarity(embeddings[0], embeddings[1])
}

// EvaluateAll performs comprehensive RAG evaluation
func (e *RAGEvaluator) EvaluateAll(ctx context.Context, req EvaluationRequest) (*EvaluationResult, error) {
	result := &EvaluationResult{
		QueryID: generateQueryID(),
	}

	// Calculate retrieval metrics
	contextPrecision, err := e.EvaluateContextPrecision(ctx, req.Query, req.Contexts)
	if err != nil {
		contextPrecision = 0.5 // Default on error
	}
	result.ContextPrecision = contextPrecision

	// Calculate context relevancy
	contextRelevancy, err := e.EvaluateContextRelevancy(ctx, req.Query, req.Contexts)
	if err != nil {
		contextRelevancy = 0.5
	}
	result.ContextRelevancy = contextRelevancy

	// Calculate faithfulness
	faithfulness, err := e.EvaluateFaithfulness(ctx, req.Answer, req.Contexts)
	if err != nil {
		faithfulness = 0.5
	}
	result.Faithfulness = faithfulness

	// Calculate answer relevancy
	answerRelevancy, err := e.EvaluateAnswerRelevancy(ctx, req.Query, req.Answer)
	if err != nil {
		answerRelevancy = 0.5
	}
	result.AnswerRelevancy = answerRelevancy

	// Calculate answer correctness if ground truth provided
	if req.GroundTruth != "" {
		answerCorrectness, err := e.EvaluateAnswerCorrectness(ctx, req.Answer, req.GroundTruth)
		if err != nil {
			answerCorrectness = 0.5
		}
		result.AnswerCorrectness = answerCorrectness
		result.AnswerSimilarity = answerCorrectness // Same calculation
	}

	// Calculate MRR (assuming contexts are ranked)
	relevanceScores := make([]float64, len(req.Contexts))
	for i, context := range req.Contexts {
		isRelevant, err := e.checkRelevance(ctx, req.Query, context)
		if err != nil {
			relevanceScores[i] = 0.0
		} else if isRelevant {
			relevanceScores[i] = 1.0
		}
	}
	result.MRR = calculateMRR(relevanceScores)
	result.NDCG = calculateNDCG(relevanceScores, len(relevanceScores))

	// Calculate overall RAGAS score
	result.RagasScore = calculateRagasScore(
		result.ContextPrecision,
		result.ContextRecall,
		result.Faithfulness,
		result.AnswerRelevancy,
	)

	return result, nil
}

// BatchEvaluate performs batch evaluation of multiple queries
func (e *RAGEvaluator) BatchEvaluate(ctx context.Context, requests []EvaluationRequest) (*BatchEvaluationResult, error) {
	result := &BatchEvaluationResult{
		Results: make([]EvaluationResult, len(requests)),
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	totalRagasScore := 0.0
	passedCount := 0
	threshold := 0.7 // RAGAS score threshold for "passing"

	for i, req := range requests {
		wg.Add(1)
		go func(idx int, r EvaluationRequest) {
			defer wg.Done()

			evalResult, err := e.EvaluateAll(ctx, r)
			if err != nil {
				evalResult = &EvaluationResult{
					QueryID:    generateQueryID(),
					RagasScore: 0.5,
				}
			}

			mu.Lock()
			result.Results[idx] = *evalResult
			totalRagasScore += evalResult.RagasScore
			if evalResult.RagasScore >= threshold {
				passedCount++
			}
			mu.Unlock()
		}(i, req)
	}

	wg.Wait()

	result.TotalQueries = len(requests)
	result.PassedQueries = passedCount
	if result.TotalQueries > 0 {
		result.AvgRagasScore = totalRagasScore / float64(result.TotalQueries)
	}

	return result, nil
}

// Helper methods using LLM

func (e *RAGEvaluator) checkRelevance(ctx context.Context, query, context string) (bool, error) {
	systemPrompt := `You are a relevance evaluator. Determine if the given context is relevant to the query.
Answer only "yes" or "no".`

	prompt := fmt.Sprintf(`Query: %s

Context: %s

Is this context relevant to the query? Answer only "yes" or "no".`, query, context)

	resp, err := e.llmClient.Chat(ctx, &llm.ChatRequest{
		Messages:     []llm.Message{{Role: "user", Content: prompt}},
		SystemPrompt: systemPrompt,
	})
	if err != nil {
		return false, err
	}

	answer := toLower(resp.Content)
	return contains(answer, "yes") && !contains(answer, "no"), nil
}

func (e *RAGEvaluator) checkContextSimilarity(ctx context.Context, context1, context2 string) (float64, error) {
	embeddings, err := e.llmClient.EmbedBatch(ctx, []string{context1, context2})
	if err != nil {
		return 0.0, err
	}

	if len(embeddings) < 2 {
		return 0.0, fmt.Errorf("insufficient embeddings")
	}

	return calculateSemanticSimilarity(embeddings[0], embeddings[1])
}

func (e *RAGEvaluator) checkSentenceRelevance(ctx context.Context, query, sentence string) (bool, error) {
	return e.checkRelevance(ctx, query, sentence)
}

func (e *RAGEvaluator) extractSentences(text string) []string {
	// Simple sentence extraction
	// In production, use NLP library
	sentences := []string{}
	start := 0

	for i, ch := range text {
		if ch == '.' || ch == '!' || ch == '?' || ch == '\n' {
			if i-start > 10 { // Minimum sentence length
				sentence := text[start:i]
				sentences = append(sentences, sentence)
			}
			start = i + 1
		}
	}

	if len(text)-start > 10 {
		sentences = append(sentences, text[start:])
	}

	return sentences
}

func (e *RAGEvaluator) extractClaims(ctx context.Context, answer string) ([]string, error) {
	systemPrompt := `You are a claim extractor. Extract all factual claims from the given text.
Return each claim on a separate line. Be precise and include only verifiable statements.`

	resp, err := e.llmClient.Chat(ctx, &llm.ChatRequest{
		Messages:     []llm.Message{{Role: "user", Content: "Extract claims from:\n\n" + answer}},
		SystemPrompt: systemPrompt,
	})
	if err != nil {
		return nil, err
	}

	// Split response into claims
	claims := e.extractSentences(resp.Content)
	if len(claims) == 0 {
		return []string{answer}, nil
	}

	return claims, nil
}

func (e *RAGEvaluator) verifyClaim(ctx context.Context, claim string, contexts []string) (bool, error) {
	combinedContext := ""
	for _, c := range contexts {
		combinedContext += c + "\n"
	}

	systemPrompt := `You are a fact checker. Determine if the claim is supported by the provided context.
Answer only "yes" or "no".`

	prompt := fmt.Sprintf(`Claim: %s

Context: %s

Is this claim supported by the context? Answer only "yes" or "no".`, claim, combinedContext)

	resp, err := e.llmClient.Chat(ctx, &llm.ChatRequest{
		Messages:     []llm.Message{{Role: "user", Content: prompt}},
		SystemPrompt: systemPrompt,
	})
	if err != nil {
		return false, err
	}

	answer := toLower(resp.Content)
	return contains(answer, "yes") && !contains(answer, "no"), nil
}

func (e *RAGEvaluator) generateQuestionsFromAnswer(ctx context.Context, answer string) ([]string, error) {
	systemPrompt := `You are a question generator. Given an answer, generate 3 potential questions that this answer could address.
Return each question on a separate line, numbered 1-3. Be specific and relevant.`

	resp, err := e.llmClient.Chat(ctx, &llm.ChatRequest{
		Messages:     []llm.Message{{Role: "user", Content: "Generate questions for:\n\n" + answer}},
		SystemPrompt: systemPrompt,
	})
	if err != nil {
		return nil, err
	}

	// Extract questions from response
	questions := []string{}
	for _, line := range e.extractSentences(resp.Content) {
		questions = append(questions, line)
	}

	if len(questions) > 3 {
		questions = questions[:3]
	}

	return questions, nil
}

func generateQueryID() string {
	return fmt.Sprintf("query-%d", time.Now().UnixNano())
}

// SaveMetrics saves RAG metrics to repository
func (e *RAGEvaluator) SaveMetrics(ctx context.Context, metrics *RAGMetrics) error {
	return e.repo.CreateRAGMetrics(ctx, metrics)
}

// GetRAGMetrics retrieves RAG metrics by ID
func (e *RAGEvaluator) GetRAGMetrics(ctx context.Context, id string) (*RAGMetrics, error) {
	return e.repo.GetRAGMetrics(ctx, id)
}

// ListRAGMetrics lists RAG metrics with filters
func (e *RAGEvaluator) ListRAGMetrics(ctx context.Context, tenantID string, limit int) ([]*RAGMetrics, error) {
	return e.repo.ListRAGMetrics(ctx, tenantID, limit)
}