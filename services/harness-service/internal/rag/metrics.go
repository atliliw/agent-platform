// Package rag provides RAG metrics calculation utilities
package rag

import (
	"context"
	"fmt"
	"math"
)

// calculateMRR computes Mean Reciprocal Rank
// MRR = 1/|Q| * sum(1/rank_i) where rank_i is the position of the first relevant document
func calculateMRR(relevanceScores []float64) float64 {
	if len(relevanceScores) == 0 {
		return 0.0
	}

	// Find the rank of the first relevant document (score > threshold)
	threshold := 0.5
	for i, score := range relevanceScores {
		if score >= threshold {
			return 1.0 / float64(i+1)
		}
	}
	return 0.0
}

// calculateNDCG computes Normalized Discounted Cumulative Gain at k
// NDCG@k = DCG@k / IDCG@k
// DCG@k = sum(rel_i / log2(i+1)) for i = 1 to k
// IDCG@k = DCG of ideal ranking (sorted by relevance)
func calculateNDCG(relevanceScores []float64, k int) float64 {
	if len(relevanceScores) == 0 {
		return 0.0
	}

	if k <= 0 || k > len(relevanceScores) {
		k = len(relevanceScores)
	}

	// Calculate DCG@k
	dcg := 0.0
	for i := 0; i < k; i++ {
		if i < len(relevanceScores) {
			// Using log2(i+2) because i is 0-indexed
			dcg += relevanceScores[i] / math.Log2(float64(i+2))
		}
	}

	// Calculate IDCG@k (ideal DCG with sorted relevance scores)
	sortedScores := make([]float64, len(relevanceScores))
	copy(sortedScores, relevanceScores)
	sortDescending(sortedScores)

	idcg := 0.0
	for i := 0; i < k; i++ {
		if i < len(sortedScores) {
			idcg += sortedScores[i] / math.Log2(float64(i+2))
		}
	}

	if idcg == 0 {
		return 0.0
	}

	return dcg / idcg
}

// sortDescending sorts a slice of float64 in descending order
func sortDescending(scores []float64) {
	for i := 0; i < len(scores)-1; i++ {
		for j := i + 1; j < len(scores); j++ {
			if scores[j] > scores[i] {
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}
}

// calculateSemanticSimilarity computes cosine similarity between two embeddings
// This is used for AnswerSimilarity metric
func calculateSemanticSimilarity(embedding1, embedding2 []float64) (float64, error) {
	if len(embedding1) != len(embedding2) {
		return 0, fmt.Errorf("embedding dimensions don't match: %d vs %d", len(embedding1), len(embedding2))
	}

	if len(embedding1) == 0 {
		return 0, nil
	}

	// Cosine similarity = dot(A, B) / (||A|| * ||B||)
	dotProduct := 0.0
	norm1 := 0.0
	norm2 := 0.0

	for i := range embedding1 {
		dotProduct += embedding1[i] * embedding2[i]
		norm1 += embedding1[i] * embedding1[i]
		norm2 += embedding2[i] * embedding2[i]
	}

	if norm1 == 0 || norm2 == 0 {
		return 0, nil
	}

	return dotProduct / (math.Sqrt(norm1) * math.Sqrt(norm2)), nil
}

// calculateF1Score computes F1 score between predicted and ground truth
func calculateF1Score(predicted, groundTruth []string) float64 {
	if len(predicted) == 0 && len(groundTruth) == 0 {
		return 1.0
	}
	if len(predicted) == 0 || len(groundTruth) == 0 {
		return 0.0
	}

	// Convert to sets for efficient lookup
	predSet := make(map[string]bool)
	for _, p := range predicted {
		predSet[p] = true
	}

	gtSet := make(map[string]bool)
	for _, g := range groundTruth {
		gtSet[g] = true
	}

	// Calculate precision and recall
	truePositives := 0
	for p := range predSet {
		if gtSet[p] {
			truePositives++
		}
	}

	precision := float64(truePositives) / float64(len(predSet))
	recall := float64(truePositives) / float64(len(gtSet))

	if precision+recall == 0 {
		return 0.0
	}

	return 2 * precision * recall / (precision + recall)
}

// calculateRagasScore computes the overall RAGAS score
// RAGAS = harmonic mean of context_precision, context_recall, faithfulness, answer_relevancy
func calculateRagasScore(contextPrecision, contextRecall, faithfulness, answerRelevancy float64) float64 {
	// Ensure all values are non-negative
	values := []float64{contextPrecision, contextRecall, faithfulness, answerRelevancy}

	// Check if any value is 0 (harmonic mean would be 0)
	for _, v := range values {
		if v <= 0 {
			return 0.0
		}
	}

	// Calculate harmonic mean
	n := float64(len(values))
	sumInverses := 0.0
	for _, v := range values {
		sumInverses += 1.0 / v
	}

	return n / sumInverses
}

// calculateAnswerRelevancyScore computes answer relevancy using LLM-based evaluation
// This is a simplified version that uses string matching
// In production, this should use LLM to generate questions from answer and measure similarity
func calculateAnswerRelevancyScore(answer, query string) float64 {
	if answer == "" || query == "" {
		return 0.0
	}

	// Simple heuristic: check if key terms from query appear in answer
	queryTerms := extractKeyTerms(query)
	answerLower := toLower(answer)

	matches := 0
	for _, term := range queryTerms {
		if contains(answerLower, toLower(term)) {
			matches++
		}
	}

	if len(queryTerms) == 0 {
		return 0.5 // Default score if no terms extracted
	}

	return float64(matches) / float64(len(queryTerms))
}

// extractKeyTerms extracts key terms from a query string
func extractKeyTerms(query string) []string {
	// Simple implementation: split by spaces and remove common words
	// In production, use NLP techniques
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "must": true, "shall": true, "can": true,
		"need": true, "dare": true, "ought": true, "used": true, "to": true,
		"of": true, "in": true, "for": true, "on": true, "with": true,
		"at": true, "by": true, "from": true, "as": true, "into": true,
		"through": true, "during": true, "before": true, "after": true,
		"above": true, "below": true, "between": true, "under": true,
		"again": true, "further": true, "then": true, "once": true,
		"what": true, "which": true, "who": true, "whom": true, "how": true,
		"why": true, "where": true, "when": true, "and": true, "or": true,
		"but": true, "if": true, "because": true, "until": true, "while": true,
	}

	// Simple tokenization by splitting on spaces
	// In production, use proper tokenization
	terms := []string{}
	word := ""
	for _, ch := range query {
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' {
			word += string(ch)
		} else {
			if word != "" && !stopWords[toLower(word)] {
				terms = append(terms, word)
			}
			word = ""
		}
	}
	if word != "" && !stopWords[toLower(word)] {
		terms = append(terms, word)
	}

	return terms
}

// toLower converts a string to lowercase
func toLower(s string) string {
	result := make([]byte, len(s))
	for i, ch := range s {
		if ch >= 'A' && ch <= 'Z' {
			result[i] = byte(ch + 32)
		} else {
			result[i] = byte(ch)
		}
	}
	return string(result)
}

// contains checks if substr is in s
func contains(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// ContextPrecisionCalculator calculates context precision using relevance judgments
type ContextPrecisionCalculator struct {
	llmClient LLMClient
}

// NewContextPrecisionCalculator creates a new context precision calculator
func NewContextPrecisionCalculator(llmClient LLMClient) *ContextPrecisionCalculator {
	return &ContextPrecisionCalculator{llmClient: llmClient}
}

// Calculate computes context precision for each context
// Context Precision = (Number of relevant contexts) / (Total number of contexts)
func (c *ContextPrecisionCalculator) Calculate(ctx context.Context, query string, contexts []string) (float64, error) {
	if len(contexts) == 0 {
		return 0.0, nil
	}

	// Use LLM to determine relevance of each context
	relevantCount := 0
	for _, context := range contexts {
		isRelevant, err := c.llmClient.CheckRelevance(ctx, query, context)
		if err != nil {
			// On error, assume context is relevant (optimistic approach)
			isRelevant = true
		}
		if isRelevant {
			relevantCount++
		}
	}

	return float64(relevantCount) / float64(len(contexts)), nil
}

// LLMClient interface for LLM-based evaluations
type LLMClient interface {
	CheckRelevance(ctx context.Context, query, context string) (bool, error)
	CheckFaithfulness(ctx context.Context, answer, context string) (float64, error)
	GenerateQuestions(ctx context.Context, answer string) ([]string, error)
	GenerateQuestionsWithVerbose(ctx context.Context, answer string) ([]string, map[string]interface{}, error)
	CompareAnswer(ctx context.Context, predicted, groundTruth string) (float64, error)
	Embed(ctx context.Context, text string) ([]float64, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float64, error)
}

// AnswerRelevancyCalculator calculates answer relevancy using LLM
type AnswerRelevancyCalculator struct {
	llmClient LLMClient
}

// NewAnswerRelevancyCalculator creates a new answer relevancy calculator
func NewAnswerRelevancyCalculator(llmClient LLMClient) *AnswerRelevancyCalculator {
	return &AnswerRelevancyCalculator{llmClient: llmClient}
}

// Calculate computes answer relevancy by generating questions from the answer
// and measuring similarity with the original query
func (a *AnswerRelevancyCalculator) Calculate(ctx context.Context, query, answer string) (float64, error) {
	if answer == "" {
		return 0.0, nil
	}
	if query == "" {
		return 0.0, nil
	}

	// Generate potential questions from the answer
	questions, err := a.llmClient.GenerateQuestions(ctx, answer)
	if err != nil {
		return 0.0, fmt.Errorf("generate questions: %w", err)
	}

	if len(questions) == 0 {
		return 0.5, nil // Default score if no questions generated
	}

	// Calculate mean similarity between original query and generated questions
	queryEmbedding, err := a.llmClient.Embed(ctx, query)
	if err != nil {
		return 0.0, fmt.Errorf("embed query: %w", err)
	}

	totalSimilarity := 0.0
	validCount := 0

	for _, q := range questions {
		qEmbedding, err := a.llmClient.Embed(ctx, q)
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
