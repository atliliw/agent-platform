package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	pb "agent-platform/pkg/pb/harness"

	"agent-platform/services/harness-service/internal/rag"
)

// EvaluateRAG evaluates RAG metrics for a single query
func (s *HarnessService) EvaluateRAG(ctx context.Context, req *pb.EvaluateRAGRequest) (*pb.RAGMetrics, error) {
	result, err := s.ragEvaluator.EvaluateAll(ctx, rag.EvaluationRequest{
		Query:       req.Query,
		Contexts:    req.Contexts,
		Answer:      req.Answer,
		GroundTruth: req.GroundTruth,
	})
	if err != nil {
		return nil, err
	}
	return ragResultToPB(result), nil
}

// BatchEvaluateRAG batch evaluates RAG metrics
func (s *HarnessService) BatchEvaluateRAG(ctx context.Context, req *pb.BatchEvaluateRAGRequest) (*pb.BatchEvaluateRAGResponse, error) {
	var evalReqs []rag.EvaluationRequest
	for _, r := range req.Requests {
		evalReqs = append(evalReqs, rag.EvaluationRequest{
			Query:       r.Query,
			Contexts:    r.Contexts,
			Answer:      r.Answer,
			GroundTruth: r.GroundTruth,
		})
	}
	batchResult, err := s.ragEvaluator.BatchEvaluate(ctx, evalReqs)
	if err != nil {
		return nil, err
	}
	var metrics []*pb.RAGMetrics
	for _, r := range batchResult.Results {
		pbM := ragResultToPB(&r)
		metrics = append(metrics, pbM)
	}
	return &pb.BatchEvaluateRAGResponse{
		Metrics:       metrics,
		AvgRagasScore: batchResult.AvgRagasScore,
		TotalQueries:  int32(batchResult.TotalQueries),
		PassedQueries: int32(batchResult.PassedQueries),
	}, nil
}

// GetRAGMetrics gets specific RAG metrics by ID
func (s *HarnessService) GetRAGMetrics(ctx context.Context, req *pb.GetRAGMetricsRequest) (*pb.RAGMetrics, error) {
	m, err := s.ragEvaluator.GetRAGMetrics(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return ragMetricsToPB(m), nil
}

// ListRAGMetrics lists RAG metrics
func (s *HarnessService) ListRAGMetrics(ctx context.Context, req *pb.ListRAGMetricsRequest) (*pb.ListRAGMetricsResponse, error) {
	metrics, err := s.ragEvaluator.ListRAGMetrics(ctx, req.TenantId, int(req.Limit))
	if err != nil {
		return nil, err
	}
	var pbMetrics []*pb.RAGMetrics
	for _, m := range metrics {
		pbMetrics = append(pbMetrics, ragMetricsToPB(m))
	}
	return &pb.ListRAGMetricsResponse{Metrics: pbMetrics, Total: int32(len(pbMetrics))}, nil
}

// CreateRAGEvaluation creates a RAG evaluation
func (s *HarnessService) CreateRAGEvaluation(ctx context.Context, req *pb.CreateRAGEvaluationRequest) (*pb.RAGEvaluation, error) {
	evaluation := &rag.RAGEvaluation{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Status:      "pending",
		TenantID:    req.TenantId,
		CreatedAt:   time.Now(),
	}

	// Serialize queries from protobuf to JSON for storage
	if len(req.Queries) > 0 {
		var queries []rag.RAGQuery
		for _, q := range req.Queries {
			queries = append(queries, rag.RAGQuery{
				QueryID:         q.QueryId,
				Query:           q.Query,
				Contexts:        q.Contexts,
				GeneratedAnswer: q.GeneratedAnswer,
				GroundTruth:     q.GroundTruth,
			})
		}
		if err := evaluation.SetQueries(queries); err != nil {
			return nil, fmt.Errorf("serialize queries: %w", err)
		}
	}

	if err := s.ragRepo.CreateRAGEvaluation(ctx, evaluation); err != nil {
		return nil, fmt.Errorf("create RAG evaluation: %w", err)
	}
	return ragEvaluationToPB(evaluation), nil
}

// GetRAGEvaluation gets a RAG evaluation by ID
func (s *HarnessService) GetRAGEvaluation(ctx context.Context, req *pb.GetRAGEvaluationRequest) (*pb.RAGEvaluation, error) {
	evaluation, err := s.ragRepo.GetRAGEvaluation(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("get RAG evaluation: %w", err)
	}
	return ragEvaluationToPB(evaluation), nil
}

// ListRAGEvaluations lists RAG evaluations
func (s *HarnessService) ListRAGEvaluations(ctx context.Context, req *pb.ListRAGEvaluationsRequest) (*pb.ListRAGEvaluationsResponse, error) {
	evaluations, err := s.ragRepo.ListRAGEvaluations(ctx, req.TenantId, req.Status)
	if err != nil {
		return nil, fmt.Errorf("list RAG evaluations: %w", err)
	}
	var pbEvals []*pb.RAGEvaluation
	for _, e := range evaluations {
		pbEvals = append(pbEvals, ragEvaluationToPB(e))
	}
	return &pb.ListRAGEvaluationsResponse{Evaluations: pbEvals}, nil
}

// RunRAGEvaluation runs a RAG evaluation
func (s *HarnessService) RunRAGEvaluation(ctx context.Context, req *pb.RunRAGEvaluationRequest) (*pb.RunRAGEvaluationResponse, error) {
	evaluation, err := s.ragRepo.GetRAGEvaluation(ctx, req.EvaluationId)
	if err != nil {
		return nil, fmt.Errorf("get RAG evaluation: %w", err)
	}

	// Parse queries from the evaluation
	queries, err := evaluation.GetQueries()
	if err != nil {
		return nil, fmt.Errorf("parse queries: %w", err)
	}
	if len(queries) == 0 {
		return nil, fmt.Errorf("no queries in evaluation")
	}

	// Update status to running
	evaluation.Status = "running"
	now := time.Now()
	evaluation.StartTime = &now
	if err := s.ragRepo.UpdateRAGEvaluation(ctx, evaluation); err != nil {
		return nil, fmt.Errorf("update evaluation status to running: %w", err)
	}

	// Convert to evaluation requests
	var evalReqs []rag.EvaluationRequest
	for _, q := range queries {
		evalReqs = append(evalReqs, rag.EvaluationRequest{
			Query:       q.Query,
			Contexts:    q.Contexts,
			Answer:      q.GeneratedAnswer,
			GroundTruth: q.GroundTruth,
		})
	}

	// Run real batch evaluation
	batchResult, err := s.ragEvaluator.BatchEvaluate(ctx, evalReqs)
	if err != nil {
		evaluation.Status = "failed"
		s.ragRepo.UpdateRAGEvaluation(ctx, evaluation)
		return nil, fmt.Errorf("batch evaluate: %w", err)
	}

	// Save individual metrics
	for i, result := range batchResult.Results {
		metrics := &rag.RAGMetrics{
			QueryID:           result.QueryID,
			Query:             queries[i].Query,
			GeneratedAnswer:   queries[i].GeneratedAnswer,
			GroundTruth:       queries[i].GroundTruth,
			ContextPrecision:  result.ContextPrecision,
			ContextRecall:     result.ContextRecall,
			ContextRelevancy:  result.ContextRelevancy,
			Faithfulness:      result.Faithfulness,
			AnswerRelevancy:   result.AnswerRelevancy,
			AnswerCorrectness: result.AnswerCorrectness,
			RagasScore:        result.RagasScore,
			TenantID:          evaluation.TenantID,
		}
		if err := s.ragEvaluator.SaveMetrics(ctx, metrics); err != nil {
			fmt.Printf("Warning: failed to save metrics for query %s: %v\n", result.QueryID, err)
		}
	}

	// Update evaluation status to completed
	endTime := time.Now()
	evaluation.Status = "completed"
	evaluation.EndTime = &endTime
	if err := s.ragRepo.UpdateRAGEvaluation(ctx, evaluation); err != nil {
		fmt.Printf("Warning: failed to update evaluation status: %v\n", err)
	}

	// Build response with results
	var pbResults []*pb.RAGMetrics
	for i, result := range batchResult.Results {
		pbResults = append(pbResults, &pb.RAGMetrics{
			QueryId:           result.QueryID,
			Query:             queries[i].Query,
			ContextPrecision:  result.ContextPrecision,
			ContextRecall:     result.ContextRecall,
			ContextRelevancy:  result.ContextRelevancy,
			Faithfulness:      result.Faithfulness,
			AnswerRelevancy:   result.AnswerRelevancy,
			AnswerCorrectness: result.AnswerCorrectness,
			RagasScore:        result.RagasScore,
		})
	}

	return &pb.RunRAGEvaluationResponse{
		EvaluationId:  req.EvaluationId,
		Results:       pbResults,
		AvgRagasScore: batchResult.AvgRagasScore,
		Status:        "completed",
		CompletedAt:   endTime.Unix(),
	}, nil
}

func ragResultToPB(r *rag.EvaluationResult) *pb.RAGMetrics {
	return &pb.RAGMetrics{
		ContextPrecision: r.ContextPrecision,
		ContextRecall:    r.ContextRecall,
		ContextRelevancy: r.ContextRelevancy,
		Faithfulness:     r.Faithfulness,
		AnswerRelevancy:  r.AnswerRelevancy,
		RagasScore:       r.RagasScore,
	}
}

func ragMetricsToPB(m *rag.RAGMetrics) *pb.RAGMetrics {
	return &pb.RAGMetrics{
		Id:               m.ID,
		QueryId:          m.QueryID,
		Query:            m.Query,
		ContextPrecision: m.ContextPrecision,
		ContextRecall:    m.ContextRecall,
		ContextRelevancy: m.ContextRelevancy,
		Faithfulness:     m.Faithfulness,
		AnswerRelevancy:  m.AnswerRelevancy,
		RagasScore:       m.RagasScore,
		Timestamp:        m.Timestamp.Unix(),
		TenantId:         m.TenantID,
	}
}

// ragEvaluationToPB converts a rag.RAGEvaluation domain type to pb.RAGEvaluation protobuf
func ragEvaluationToPB(e *rag.RAGEvaluation) *pb.RAGEvaluation {
	if e == nil {
		return nil
	}

	var startTime, endTime int64
	if e.StartTime != nil {
		startTime = e.StartTime.Unix()
	}
	if e.EndTime != nil {
		endTime = e.EndTime.Unix()
	}

	// Deserialize queries from JSON and convert to protobuf
	var pbQueries []*pb.RAGQuery
	if e.Queries != "" {
		var queries []rag.RAGQuery
		if err := json.Unmarshal([]byte(e.Queries), &queries); err == nil {
			for _, q := range queries {
				pbQueries = append(pbQueries, &pb.RAGQuery{
					QueryId:         q.QueryID,
					Query:           q.Query,
					Contexts:        q.Contexts,
					GeneratedAnswer: q.GeneratedAnswer,
					GroundTruth:     q.GroundTruth,
				})
			}
		}
	}

	return &pb.RAGEvaluation{
		Id:          e.ID,
		Name:        e.Name,
		Description: e.Description,
		Queries:     pbQueries,
		Status:      e.Status,
		StartTime:   startTime,
		EndTime:     endTime,
		TenantId:    e.TenantID,
		CreatedAt:   e.CreatedAt.Unix(),
	}
}

