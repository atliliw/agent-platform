// Package handler provides gRPC handlers for knowledge service
package handler

import (
	"context"

	pb "agent-platform/pkg/pb/knowledge"
	commonpb "agent-platform/pkg/pb/common"
	"agent-platform/services/knowledge-service/internal/service"
)

// KnowledgeHandler implements KnowledgeServiceServer
type KnowledgeHandler struct {
	pb.UnimplementedKnowledgeServiceServer
	service *service.KnowledgeService
}

// NewKnowledgeHandler creates a new knowledge handler
func NewKnowledgeHandler(service *service.KnowledgeService) *KnowledgeHandler {
	return &KnowledgeHandler{
		service: service,
	}
}

// Upload handles file upload
func (h *KnowledgeHandler) Upload(ctx context.Context, req *pb.UploadRequest) (*pb.UploadResponse, error) {
	return h.service.Upload(ctx, req)
}

// GetDocument gets a document
func (h *KnowledgeHandler) GetDocument(ctx context.Context, req *pb.GetDocumentRequest) (*pb.Document, error) {
	return h.service.GetDocument(ctx, req)
}

// ListDocuments lists documents
func (h *KnowledgeHandler) ListDocuments(ctx context.Context, req *pb.ListDocumentsRequest) (*pb.ListDocumentsResponse, error) {
	return h.service.ListDocuments(ctx, req)
}

// DeleteDocument deletes a document
func (h *KnowledgeHandler) DeleteDocument(ctx context.Context, req *pb.DeleteDocumentRequest) (*commonpb.Empty, error) {
	return h.service.DeleteDocument(ctx, req)
}

// Search performs search
func (h *KnowledgeHandler) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	return h.service.Search(ctx, req)
}

// SearchStream performs streaming search
func (h *KnowledgeHandler) SearchStream(req *pb.SearchRequest, stream pb.KnowledgeService_SearchStreamServer) error {
	return h.service.SearchStream(req, stream)
}