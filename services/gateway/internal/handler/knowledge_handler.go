// Package handler provides HTTP handlers for Gateway
package handler

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"time"

	"agent-platform/pkg/config"
	common "agent-platform/pkg/pb/common"
	pb "agent-platform/pkg/pb/knowledge"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// RealKnowledgeHandler handles knowledge requests with real gRPC calls
type RealKnowledgeHandler struct {
	cfg    *config.Config
	client pb.KnowledgeServiceClient
	conn   *grpc.ClientConn
}

// NewRealKnowledgeHandler creates a new knowledge handler with gRPC connection
func NewRealKnowledgeHandler(cfg *config.Config) *RealKnowledgeHandler {
	addr := cfg.Services.Knowledge
	if addr == "" {
		return &RealKnowledgeHandler{cfg: cfg, client: nil, conn: nil}
	}

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return &RealKnowledgeHandler{cfg: cfg, client: nil, conn: nil}
	}

	return &RealKnowledgeHandler{
		cfg:    cfg,
		client: pb.NewKnowledgeServiceClient(conn),
		conn:   conn,
	}
}

// Close closes the gRPC connection
func (h *RealKnowledgeHandler) Close() error {
	if h.conn != nil {
		return h.conn.Close()
	}
	return nil
}

// Upload handles file upload to knowledge service
func (h *RealKnowledgeHandler) Upload(c *gin.Context) {
	if h.client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": -1, "message": "knowledge service not available"})
		return
	}

	// Parse multipart form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "failed to read file: " + err.Error()})
		return
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "failed to read file content: " + err.Error()})
		return
	}

	// Build gRPC request
	tenantID := c.GetString("tenant_id")
	chunkSize, _ := strconv.Atoi(c.DefaultPostForm("chunk_size", "512"))
	chunkOverlap, _ := strconv.Atoi(c.DefaultPostForm("chunk_overlap", "50"))

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	resp, err := h.client.Upload(ctx, &pb.UploadRequest{
		Filename:      header.Filename,
		Content:       content,
		ChunkStrategy: c.DefaultPostForm("chunk_strategy", "token"),
		ChunkSize:     int32(chunkSize),
		ChunkOverlap:  int32(chunkOverlap),
		TenantId:      tenantID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": "upload failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"document_id": resp.DocumentId,
			"filename":    resp.Filename,
			"chunk_count": resp.ChunkCount,
			"status":      resp.Status,
		},
	})
}

// ListDocuments lists documents from knowledge service
func (h *RealKnowledgeHandler) ListDocuments(c *gin.Context) {
	if h.client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": -1, "message": "knowledge service not available"})
		return
	}

	tenantID := c.GetString("tenant_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := h.client.ListDocuments(ctx, &pb.ListDocumentsRequest{
		Pagination: &common.PaginationRequest{
			Page:     int32(page),
			PageSize: int32(pageSize),
		},
		TenantId: tenantID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": "list documents failed: " + err.Error()})
		return
	}

	documents := make([]gin.H, 0, len(resp.Documents))
	for _, doc := range resp.Documents {
		documents = append(documents, documentToJSON(doc))
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"documents": documents,
			"pagination": gin.H{
				"total":     resp.Pagination.GetTotal(),
				"page":      resp.Pagination.GetPage(),
				"page_size": resp.Pagination.GetPageSize(),
			},
		},
	})
}

// GetDocument gets a document from knowledge service
func (h *RealKnowledgeHandler) GetDocument(c *gin.Context) {
	if h.client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": -1, "message": "knowledge service not available"})
		return
	}

	docID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	doc, err := h.client.GetDocument(ctx, &pb.GetDocumentRequest{
		Id:       docID,
		TenantId: tenantID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": "get document failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"document": documentToJSON(doc),
		},
	})
}

// DeleteDocument deletes a document from knowledge service
func (h *RealKnowledgeHandler) DeleteDocument(c *gin.Context) {
	if h.client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": -1, "message": "knowledge service not available"})
		return
	}

	docID := c.Param("id")
	tenantID := c.GetString("tenant_id")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := h.client.DeleteDocument(ctx, &pb.DeleteDocumentRequest{
		Id:       docID,
		TenantId: tenantID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": "delete document failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}

// Search handles search requests against knowledge service
func (h *RealKnowledgeHandler) Search(c *gin.Context) {
	if h.client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": -1, "message": "knowledge service not available"})
		return
	}

	var reqBody struct {
		Query          string            `json:"query"`
		TopK           int32             `json:"top_k"`
		SearchType     string            `json:"search_type"`
		ScoreThreshold float64           `json:"score_threshold"`
		Filters        map[string]string `json:"filters"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "invalid request: " + err.Error()})
		return
	}

	tenantID := c.GetString("tenant_id")
	if reqBody.TopK == 0 {
		reqBody.TopK = 5
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := h.client.Search(ctx, &pb.SearchRequest{
		Query:          reqBody.Query,
		TopK:           reqBody.TopK,
		SearchType:     reqBody.SearchType,
		ScoreThreshold: reqBody.ScoreThreshold,
		Filters:        reqBody.Filters,
		TenantId:       tenantID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": "search failed: " + err.Error()})
		return
	}

	results := make([]gin.H, 0, len(resp.Results))
	for _, r := range resp.Results {
		results = append(results, gin.H{
			"chunk_id":    r.ChunkId,
			"document_id": r.DocumentId,
			"content":     r.Content,
			"score":       r.Score,
			"metadata":    r.Metadata,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"results":    results,
			"total":      resp.Total,
			"latency_ms": resp.LatencyMs,
		},
	})
}

// documentToJSON converts a protobuf Document to a JSON-friendly map
func documentToJSON(doc *pb.Document) gin.H {
	if doc == nil {
		return nil
	}

	return gin.H{
		"id":          doc.Id,
		"filename":    doc.Filename,
		"title":       doc.Title,
		"content":     doc.Content,
		"chunk_count": doc.ChunkCount,
		"status":      doc.Status,
		"metadata":    doc.Metadata,
		"created_at":  doc.CreatedAt,
		"updated_at":  doc.UpdatedAt,
	}
}
