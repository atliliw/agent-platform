// Package service provides business logic for knowledge service
package service

import (
	"context"
	"fmt"
	"time"

	"agent-platform/pkg/config"
	"agent-platform/pkg/llm"
	pb "agent-platform/pkg/pb/knowledge"
	commonpb "agent-platform/pkg/pb/common"
	"agent-platform/services/knowledge-service/internal/chunker"
	"agent-platform/services/knowledge-service/internal/parser"
	"agent-platform/services/knowledge-service/internal/repository"
)

// KnowledgeService provides knowledge management functionality
type KnowledgeService struct {
	pb.UnimplementedKnowledgeServiceServer
	llmClient llm.Client
	docRepo   *repository.DocumentRepository
	cfg       *config.Config
	chunker   *chunker.TokenChunker
}

// NewKnowledgeService creates a new knowledge service
func NewKnowledgeService(llmClient llm.Client, docRepo *repository.DocumentRepository, cfg *config.Config) *KnowledgeService {
	return &KnowledgeService{
		llmClient: llmClient,
		docRepo:   docRepo,
		cfg:       cfg,
		chunker:   chunker.NewTokenChunker(512, 50),
	}
}

// Upload handles file upload
func (s *KnowledgeService) Upload(ctx context.Context, req *pb.UploadRequest) (*pb.UploadResponse, error) {
	// Parse file content
	content, err := parser.Parse(req.Filename, req.Content)
	if err != nil {
		return nil, fmt.Errorf("parse file: %w", err)
	}

	// Chunk content
	chunkSize := int(req.ChunkSize)
	if chunkSize == 0 {
		chunkSize = 512
	}
	chunkOverlap := int(req.ChunkOverlap)
	if chunkOverlap == 0 {
		chunkOverlap = 50
	}

	s.chunker = chunker.NewTokenChunker(chunkSize, chunkOverlap)
	chunks := s.chunker.Chunk(content)

	// Generate embeddings
	var embeddings [][]float64
	for i := 0; i < len(chunks); i += 20 {
		end := i + 20
		if end > len(chunks) {
			end = len(chunks)
		}
		batch := chunks[i:end]
		batchEmbeddings, err := s.llmClient.EmbedBatch(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("generate embeddings: %w", err)
		}
		embeddings = append(embeddings, batchEmbeddings...)
	}

	// Save document and chunks
	docID, err := s.docRepo.SaveDocument(ctx, req.Filename, content, chunks, embeddings, req.TenantId)
	if err != nil {
		return nil, fmt.Errorf("save document: %w", err)
	}

	return &pb.UploadResponse{
		DocumentId:  docID,
		Filename:    req.Filename,
		ChunkCount:  int32(len(chunks)),
		Status:      "ready",
	}, nil
}

// GetDocument gets a document
func (s *KnowledgeService) GetDocument(ctx context.Context, req *pb.GetDocumentRequest) (*pb.Document, error) {
	doc, err := s.docRepo.GetDocument(ctx, req.Id, req.TenantId)
	if err != nil {
		return nil, err
	}

	return &pb.Document{
		Id:         doc.ID,
		Filename:   doc.Filename,
		Content:    doc.Content,
		ChunkCount: int32(doc.ChunkCount),
		Status:     doc.Status,
		CreatedAt:  doc.CreatedAt.Unix(),
		UpdatedAt:  doc.UpdatedAt.Unix(),
	}, nil
}

// ListDocuments lists documents
func (s *KnowledgeService) ListDocuments(ctx context.Context, req *pb.ListDocumentsRequest) (*pb.ListDocumentsResponse, error) {
	page := int(req.Pagination.GetPage())
	pageSize := int(req.Pagination.GetPageSize())
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}

	docs, total, err := s.docRepo.ListDocuments(ctx, req.TenantId, req.Status, page, pageSize)
	if err != nil {
		return nil, err
	}

	resp := &pb.ListDocumentsResponse{
		Pagination: &commonpb.PaginationResponse{
			Total:    int32(total),
			Page:     int32(page),
			PageSize: int32(pageSize),
		},
	}

	for _, doc := range docs {
		resp.Documents = append(resp.Documents, &pb.Document{
			Id:         doc.ID,
			Filename:   doc.Filename,
			ChunkCount: int32(doc.ChunkCount),
			Status:     doc.Status,
			CreatedAt:  doc.CreatedAt.Unix(),
		})
	}

	return resp, nil
}

// DeleteDocument deletes a document
func (s *KnowledgeService) DeleteDocument(ctx context.Context, req *pb.DeleteDocumentRequest) (*commonpb.Empty, error) {
	if err := s.docRepo.DeleteDocument(ctx, req.Id, req.TenantId); err != nil {
		return nil, err
	}
	return &commonpb.Empty{}, nil
}

// Search performs search
func (s *KnowledgeService) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	startTime := time.Now()

	// Generate query embedding
	queryEmbedding, err := s.llmClient.Embed(ctx, req.Query)
	if err != nil {
		return nil, fmt.Errorf("generate query embedding: %w", err)
	}

	topK := int(req.TopK)
	if topK == 0 {
		topK = 10
	}

	var results []*pb.SearchResult

	switch req.SearchType {
	case "bm25":
		// BM25 search
		chunks, err := s.docRepo.SearchBM25(ctx, req.Query, topK, req.TenantId)
		if err != nil {
			return nil, err
		}
		for i, c := range chunks {
			results = append(results, &pb.SearchResult{
				ChunkId:    c.ID,
				DocumentId: c.DocID,
				Content:    c.Content,
				Score:      float64(topK - i), // Simple ranking
			})
		}

	case "hybrid":
		// Hybrid search (RRF fusion)
		results, err = s.hybridSearch(ctx, queryEmbedding, req.Query, topK, req.TenantId)
		if err != nil {
			return nil, err
		}

	default:
		// Vector search
		searchResults, err := s.docRepo.SearchVector(ctx, queryEmbedding, topK, req.TenantId, req.ScoreThreshold)
		if err != nil {
			return nil, err
		}
		for _, r := range searchResults {
			content, _ := r.Payload["content"].(string)
			docID, _ := r.Payload["doc_id"].(string)
			results = append(results, &pb.SearchResult{
				ChunkId:    r.ID,
				DocumentId: docID,
				Content:    content,
				Score:      r.Score,
			})
		}
	}

	return &pb.SearchResponse{
		Results:    results,
		Total:      int64(len(results)),
		LatencyMs:  time.Since(startTime).Milliseconds(),
	}, nil
}

// SearchStream performs streaming search
func (s *KnowledgeService) SearchStream(req *pb.SearchRequest, stream pb.KnowledgeService_SearchStreamServer) error {
	resp, err := s.Search(stream.Context(), req)
	if err != nil {
		return err
	}

	for _, result := range resp.Results {
		if err := stream.Send(result); err != nil {
			return err
		}
	}

	return nil
}

func (s *KnowledgeService) hybridSearch(ctx context.Context, queryEmbedding []float64, query string, topK int, tenantID string) ([]*pb.SearchResult, error) {
	// Vector search
	vectorResults, err := s.docRepo.SearchVector(ctx, queryEmbedding, topK*2, tenantID, 0)
	if err != nil {
		return nil, err
	}

	// BM25 search
	bm25Chunks, err := s.docRepo.SearchBM25(ctx, query, topK*2, tenantID)
	if err != nil {
		return nil, err
	}

	// RRF fusion
	scores := make(map[string]float64)
	contentMap := make(map[string]string)
	docIDMap := make(map[string]string)

	k := 60.0
	for i, r := range vectorResults {
		content, _ := r.Payload["content"].(string)
		docID, _ := r.Payload["doc_id"].(string)
		scores[r.ID] += 1.0 / (k + float64(i+1))
		contentMap[r.ID] = content
		docIDMap[r.ID] = docID
	}

	for i, c := range bm25Chunks {
		scores[c.ID] += 1.0 / (k + float64(i+1))
		contentMap[c.ID] = c.Content
		docIDMap[c.ID] = c.DocID
	}

	// Sort by score
	type result struct {
		id    string
		score float64
	}
	var sorted []result
	for id, score := range scores {
		sorted = append(sorted, result{id: id, score: score})
	}

	// Simple sort
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].score > sorted[i].score {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Build results
	var results []*pb.SearchResult
	for i := 0; i < len(sorted) && i < topK; i++ {
		r := sorted[i]
		results = append(results, &pb.SearchResult{
			ChunkId:    r.id,
			DocumentId: docIDMap[r.id],
			Content:    contentMap[r.id],
			Score:      r.score,
		})
	}

	return results, nil
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}