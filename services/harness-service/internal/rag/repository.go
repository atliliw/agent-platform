// Package rag provides RAG metrics data access
package rag

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RAGMetricsRecord is the database model for RAG metrics
type RAGMetricsRecord struct {
	ID                string    `gorm:"primaryKey"`
	QueryID           string    `gorm:"index"`
	Query             string    `gorm:"type:text"`
	RetrievedDocs     string    `gorm:"type:text"` // JSON: []string
	GeneratedAnswer   string    `gorm:"type:text"`
	GroundTruth       string    `gorm:"type:text"`

	// Retrieval Quality Metrics
	ContextPrecision    float64
	ContextRecall       float64
	ContextRelevancy    float64
	ContextEntityRecall float64
	NoiseSensitivity    float64
	MRR                 float64
	NDCG                float64

	// Generation Quality Metrics
	Faithfulness      float64
	AnswerRelevancy   float64
	AnswerCorrectness float64
	AnswerSimilarity  float64
	Hallucination     float64
	Comprehensiveness float64
	Coherence         float64

	// Comprehensive Metrics
	RagasScore        float64

	Timestamp         time.Time `gorm:"index"`
	TenantID          string    `gorm:"index"`
}

// RAGEvaluationRecord is the database model for RAG evaluations
type RAGEvaluationRecord struct {
	ID          string    `gorm:"primaryKey"`
	Name        string
	Description string
	Queries     string    `gorm:"type:text"` // JSON: []RAGQuery
	Status      string    `gorm:"index"`
	StartTime   *time.Time
	EndTime     *time.Time
	TenantID    string    `gorm:"index"`
	CreatedAt   time.Time
}

// Repository provides data access for RAG metrics
type Repository struct {
	db   *gorm.DB
	mu   sync.RWMutex
	// In-memory storage for memory mode
	metricsRecords    map[string]*RAGMetricsRecord
	evaluationRecords map[string]*RAGEvaluationRecord
	memoryMode        bool
}

// NewRepository creates a new RAG repository
func NewRepository(db *gorm.DB) *Repository {
	repo := &Repository{
		db:                db,
		metricsRecords:    make(map[string]*RAGMetricsRecord),
		evaluationRecords: make(map[string]*RAGEvaluationRecord),
		memoryMode:        db == nil,
	}

	// Auto-migrate if database is provided
	if db != nil {
		db.AutoMigrate(&RAGMetricsRecord{}, &RAGEvaluationRecord{})
	}

	return repo
}

// CreateRAGMetrics creates a new RAG metrics record
func (r *Repository) CreateRAGMetrics(ctx context.Context, metrics *RAGMetrics) error {
	if metrics.ID == "" {
		metrics.ID = uuid.New().String()
	}
	if metrics.Timestamp.IsZero() {
		metrics.Timestamp = time.Now()
	}

	record := r.metricsToRecord(metrics)

	if r.memoryMode {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.metricsRecords[record.ID] = record
		return nil
	}

	return r.db.WithContext(ctx).Create(record).Error
}

// GetRAGMetrics retrieves RAG metrics by ID
func (r *Repository) GetRAGMetrics(ctx context.Context, id string) (*RAGMetrics, error) {
	if r.memoryMode {
		r.mu.RLock()
		defer r.mu.RUnlock()
		record, ok := r.metricsRecords[id]
		if !ok {
			return nil, fmt.Errorf("RAG metrics not found: %s", id)
		}
		return r.recordToMetrics(record), nil
	}

	var record RAGMetricsRecord
	if err := r.db.WithContext(ctx).First(&record, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return r.recordToMetrics(&record), nil
}

// ListRAGMetrics lists RAG metrics with filters
func (r *Repository) ListRAGMetrics(ctx context.Context, tenantID string, limit int) ([]*RAGMetrics, error) {
	if r.memoryMode {
		r.mu.RLock()
		defer r.mu.RUnlock()

		var results []*RAGMetrics
		for _, record := range r.metricsRecords {
			if tenantID == "" || record.TenantID == tenantID {
				results = append(results, r.recordToMetrics(record))
			}
			if limit > 0 && len(results) >= limit {
				break
			}
		}
		return results, nil
	}

	var records []RAGMetricsRecord
	query := r.db.WithContext(ctx).Order("timestamp DESC")
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&records).Error; err != nil {
		return nil, err
	}

	results := make([]*RAGMetrics, len(records))
	for i, record := range records {
		results[i] = r.recordToMetrics(&record)
	}

	return results, nil
}

// DeleteRAGMetrics deletes RAG metrics by ID
func (r *Repository) DeleteRAGMetrics(ctx context.Context, id string) error {
	if r.memoryMode {
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.metricsRecords, id)
		return nil
	}

	return r.db.WithContext(ctx).Delete(&RAGMetricsRecord{}, "id = ?", id).Error
}

// CreateRAGEvaluation creates a new RAG evaluation
func (r *Repository) CreateRAGEvaluation(ctx context.Context, evaluation *RAGEvaluation) error {
	if evaluation.ID == "" {
		evaluation.ID = uuid.New().String()
	}
	if evaluation.CreatedAt.IsZero() {
		evaluation.CreatedAt = time.Now()
	}

	record := r.evaluationToRecord(evaluation)

	if r.memoryMode {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.evaluationRecords[record.ID] = record
		return nil
	}

	return r.db.WithContext(ctx).Create(record).Error
}

// GetRAGEvaluation retrieves a RAG evaluation by ID
func (r *Repository) GetRAGEvaluation(ctx context.Context, id string) (*RAGEvaluation, error) {
	if r.memoryMode {
		r.mu.RLock()
		defer r.mu.RUnlock()
		record, ok := r.evaluationRecords[id]
		if !ok {
			return nil, fmt.Errorf("RAG evaluation not found: %s", id)
		}
		return r.recordToEvaluation(record), nil
	}

	var record RAGEvaluationRecord
	if err := r.db.WithContext(ctx).First(&record, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return r.recordToEvaluation(&record), nil
}

// ListRAGEvaluations lists RAG evaluations
func (r *Repository) ListRAGEvaluations(ctx context.Context, tenantID string, status string) ([]*RAGEvaluation, error) {
	if r.memoryMode {
		r.mu.RLock()
		defer r.mu.RUnlock()

		var results []*RAGEvaluation
		for _, record := range r.evaluationRecords {
			if (tenantID == "" || record.TenantID == tenantID) &&
				(status == "" || record.Status == status) {
				results = append(results, r.recordToEvaluation(record))
			}
		}
		return results, nil
	}

	var records []RAGEvaluationRecord
	query := r.db.WithContext(ctx).Order("created_at DESC")
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Find(&records).Error; err != nil {
		return nil, err
	}

	results := make([]*RAGEvaluation, len(records))
	for i, record := range records {
		results[i] = r.recordToEvaluation(&record)
	}

	return results, nil
}

// UpdateRAGEvaluation updates a RAG evaluation
func (r *Repository) UpdateRAGEvaluation(ctx context.Context, evaluation *RAGEvaluation) error {
	record := r.evaluationToRecord(evaluation)

	if r.memoryMode {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.evaluationRecords[record.ID] = record
		return nil
	}

	return r.db.WithContext(ctx).Save(record).Error
}

// DeleteRAGEvaluation deletes a RAG evaluation
func (r *Repository) DeleteRAGEvaluation(ctx context.Context, id string) error {
	if r.memoryMode {
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.evaluationRecords, id)
		return nil
	}

	return r.db.WithContext(ctx).Delete(&RAGEvaluationRecord{}, "id = ?", id).Error
}

// Helper methods for type conversion

func (r *Repository) metricsToRecord(m *RAGMetrics) *RAGMetricsRecord {
	return &RAGMetricsRecord{
		ID:                  m.ID,
		QueryID:             m.QueryID,
		Query:               m.Query,
		RetrievedDocs:       m.RetrievedDocs,
		GeneratedAnswer:     m.GeneratedAnswer,
		GroundTruth:         m.GroundTruth,
		ContextPrecision:    m.ContextPrecision,
		ContextRecall:       m.ContextRecall,
		ContextRelevancy:    m.ContextRelevancy,
		ContextEntityRecall: m.ContextEntityRecall,
		NoiseSensitivity:    m.NoiseSensitivity,
		MRR:                 m.MRR,
		NDCG:                m.NDCG,
		Faithfulness:        m.Faithfulness,
		AnswerRelevancy:     m.AnswerRelevancy,
		AnswerCorrectness:   m.AnswerCorrectness,
		AnswerSimilarity:    m.AnswerSimilarity,
		Hallucination:       m.Hallucination,
		Comprehensiveness:   m.Comprehensiveness,
		Coherence:           m.Coherence,
		RagasScore:          m.RagasScore,
		Timestamp:           m.Timestamp,
		TenantID:            m.TenantID,
	}
}

func (r *Repository) recordToMetrics(rec *RAGMetricsRecord) *RAGMetrics {
	return &RAGMetrics{
		ID:                  rec.ID,
		QueryID:             rec.QueryID,
		Query:               rec.Query,
		RetrievedDocs:       rec.RetrievedDocs,
		GeneratedAnswer:     rec.GeneratedAnswer,
		GroundTruth:         rec.GroundTruth,
		ContextPrecision:    rec.ContextPrecision,
		ContextRecall:       rec.ContextRecall,
		ContextRelevancy:    rec.ContextRelevancy,
		ContextEntityRecall: rec.ContextEntityRecall,
		NoiseSensitivity:    rec.NoiseSensitivity,
		MRR:                 rec.MRR,
		NDCG:                rec.NDCG,
		Faithfulness:        rec.Faithfulness,
		AnswerRelevancy:     rec.AnswerRelevancy,
		AnswerCorrectness:   rec.AnswerCorrectness,
		AnswerSimilarity:    rec.AnswerSimilarity,
		Hallucination:       rec.Hallucination,
		Comprehensiveness:   rec.Comprehensiveness,
		Coherence:           rec.Coherence,
		RagasScore:          rec.RagasScore,
		Timestamp:           rec.Timestamp,
		TenantID:            rec.TenantID,
	}
}

func (r *Repository) evaluationToRecord(e *RAGEvaluation) *RAGEvaluationRecord {
	return &RAGEvaluationRecord{
		ID:          e.ID,
		Name:        e.Name,
		Description: e.Description,
		Queries:     e.Queries,
		Status:      e.Status,
		StartTime:   e.StartTime,
		EndTime:     e.EndTime,
		TenantID:    e.TenantID,
		CreatedAt:   e.CreatedAt,
	}
}

func (r *Repository) recordToEvaluation(rec *RAGEvaluationRecord) *RAGEvaluation {
	return &RAGEvaluation{
		ID:          rec.ID,
		Name:        rec.Name,
		Description: rec.Description,
		Queries:     rec.Queries,
		Status:      rec.Status,
		StartTime:   rec.StartTime,
		EndTime:     rec.EndTime,
		TenantID:    rec.TenantID,
		CreatedAt:   rec.CreatedAt,
	}
}

// GetDB returns the underlying database connection
func (r *Repository) GetDB() *gorm.DB {
	return r.db
}
