// Package rag provides RAG metrics evaluation functionality
package rag

import (
	"encoding/json"
	"time"
)

// RAGMetrics represents comprehensive RAG evaluation metrics
type RAGMetrics struct {
	ID              string    `json:"id"`
	QueryID         string    `json:"query_id"`
	Query           string    `json:"query"`
	RetrievedDocs   string    `json:"retrieved_docs"`   // JSON: []string
	GeneratedAnswer string    `json:"generated_answer"`
	GroundTruth     string    `json:"ground_truth"`

	// Retrieval Quality Metrics (检索质量指标)
	ContextPrecision  float64 `json:"context_precision"`  // 上下文精确率
	ContextRecall     float64 `json:"context_recall"`     // 上下文召回率
	ContextRelevancy  float64 `json:"context_relevancy"`  // 上下文相关性
	MRR               float64 `json:"mrr"`                // Mean Reciprocal Rank
	NDCG              float64 `json:"ndcg"`               // Normalized Discounted Cumulative Gain

	// Generation Quality Metrics (生成质量指标)
	Faithfulness      float64 `json:"faithfulness"`      // 答案忠实度
	AnswerRelevancy   float64 `json:"answer_relevancy"`  // 答案相关性
	AnswerCorrectness float64 `json:"answer_correctness"` // 答案正确性
	AnswerSimilarity  float64 `json:"answer_similarity"` // 答案相似度

	// Comprehensive Metrics (综合指标)
	RagasScore        float64 `json:"ragas_score"` // 综合评分 (RAGAS)

	Timestamp         time.Time `json:"timestamp"`
	TenantID          string    `json:"tenant_id"`
}

// RAGEvaluation represents a RAG evaluation batch
type RAGEvaluation struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Queries     string    `json:"queries"` // JSON: []RAGQuery
	Status      string    `json:"status"`  // pending, running, completed, failed
	StartTime   *time.Time `json:"start_time,omitempty"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	TenantID    string    `json:"tenant_id"`
	CreatedAt   time.Time `json:"created_at"`
}

// RAGQuery represents a single RAG evaluation query
type RAGQuery struct {
	QueryID       string   `json:"query_id"`
	Query         string   `json:"query"`
	Contexts      []string `json:"contexts"`
	GeneratedAnswer string `json:"generated_answer"`
	GroundTruth   string   `json:"ground_truth"`
}

// EvaluationRequest represents a single evaluation request
type EvaluationRequest struct {
	Query       string   `json:"query"`
	Contexts    []string `json:"contexts"`
	Answer      string   `json:"answer"`
	GroundTruth string   `json:"ground_truth"`
}

// EvaluationResult represents the result of a RAG evaluation
type EvaluationResult struct {
	QueryID           string  `json:"query_id"`
	ContextPrecision  float64 `json:"context_precision"`
	ContextRecall     float64 `json:"context_recall"`
	ContextRelevancy  float64 `json:"context_relevancy"`
	MRR               float64 `json:"mrr"`
	NDCG              float64 `json:"ndcg"`
	Faithfulness      float64 `json:"faithfulness"`
	AnswerRelevancy   float64 `json:"answer_relevancy"`
	AnswerCorrectness float64 `json:"answer_correctness"`
	AnswerSimilarity  float64 `json:"answer_similarity"`
	RagasScore        float64 `json:"ragas_score"`
}

// BatchEvaluationResult represents results of batch evaluation
type BatchEvaluationResult struct {
	Results       []EvaluationResult `json:"results"`
	AvgRagasScore float64            `json:"avg_ragas_score"`
	TotalQueries  int                `json:"total_queries"`
	PassedQueries int                `json:"passed_queries"` // queries with RagasScore >= threshold
}

// GetRetrievedDocs parses the JSON stored RetrievedDocs field
func (m *RAGMetrics) GetRetrievedDocs() ([]string, error) {
	if m.RetrievedDocs == "" {
		return nil, nil
	}
	var docs []string
	if err := json.Unmarshal([]byte(m.RetrievedDocs), &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// SetRetrievedDocs stores the RetrievedDocs as JSON
func (m *RAGMetrics) SetRetrievedDocs(docs []string) error {
	if docs == nil {
		m.RetrievedDocs = ""
		return nil
	}
	data, err := json.Marshal(docs)
	if err != nil {
		return err
	}
	m.RetrievedDocs = string(data)
	return nil
}

// GetQueries parses the JSON stored Queries field
func (e *RAGEvaluation) GetQueries() ([]RAGQuery, error) {
	if e.Queries == "" {
		return nil, nil
	}
	var queries []RAGQuery
	if err := json.Unmarshal([]byte(e.Queries), &queries); err != nil {
		return nil, err
	}
	return queries, nil
}

// SetQueries stores the Queries as JSON
func (e *RAGEvaluation) SetQueries(queries []RAGQuery) error {
	if queries == nil {
		e.Queries = ""
		return nil
	}
	data, err := json.Marshal(queries)
	if err != nil {
		return err
	}
	e.Queries = string(data)
	return nil
}