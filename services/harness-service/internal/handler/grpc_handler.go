// Package handler provides gRPC handlers for Harness service
package handler

import (
	"context"

	pb "agent-platform/pkg/pb/harness"
	commonpb "agent-platform/pkg/pb/common"
	"agent-platform/services/harness-service/internal/service"
)

// HarnessHandler implements HarnessServiceServer
type HarnessHandler struct {
	pb.UnimplementedHarnessServiceServer
	service *service.HarnessService
}

// NewHarnessHandler creates a new harness handler
func NewHarnessHandler(service *service.HarnessService) *HarnessHandler {
	return &HarnessHandler{
		service: service,
	}
}

// CreateRule creates a rule
func (h *HarnessHandler) CreateRule(ctx context.Context, req *pb.CreateRuleRequest) (*pb.Rule, error) {
	return h.service.CreateRule(ctx, req)
}

// ListRules lists rules
func (h *HarnessHandler) ListRules(ctx context.Context, req *pb.ListRulesRequest) (*pb.ListRulesResponse, error) {
	return h.service.ListRules(ctx, req)
}

// UpdateRule updates a rule
func (h *HarnessHandler) UpdateRule(ctx context.Context, req *pb.UpdateRuleRequest) (*pb.Rule, error) {
	return h.service.UpdateRule(ctx, req)
}

// DeleteRule deletes a rule
func (h *HarnessHandler) DeleteRule(ctx context.Context, req *pb.DeleteRuleRequest) (*commonpb.Empty, error) {
	return h.service.DeleteRule(ctx, req)
}

// CheckGuardrail checks guardrail
func (h *HarnessHandler) CheckGuardrail(ctx context.Context, req *pb.GuardrailCheckRequest) (*pb.GuardrailCheckResponse, error) {
	return h.service.CheckGuardrail(ctx, req)
}

// CreateEvalSuite creates an eval suite
func (h *HarnessHandler) CreateEvalSuite(ctx context.Context, req *pb.CreateEvalSuiteRequest) (*pb.EvalSuite, error) {
	return h.service.CreateEvalSuite(ctx, req)
}

// RunEval runs evaluation
func (h *HarnessHandler) RunEval(ctx context.Context, req *pb.RunEvalRequest) (*pb.RunEvalResponse, error) {
	return h.service.RunEval(ctx, req)
}

// GetEvalResults gets eval results
func (h *HarnessHandler) GetEvalResults(ctx context.Context, req *pb.GetEvalResultsRequest) (*pb.RunEvalResponse, error) {
	return h.service.GetEvalResults(ctx, req)
}

// CreateABTest creates an A/B test
func (h *HarnessHandler) CreateABTest(ctx context.Context, req *pb.CreateABTestRequest) (*pb.ABTest, error) {
	return h.service.CreateABTest(ctx, req)
}

// GetABTestResult gets A/B test result
func (h *HarnessHandler) GetABTestResult(ctx context.Context, req *pb.GetABTestResultRequest) (*pb.ABTestResult, error) {
	return h.service.GetABTestResult(ctx, req)
}

// PromoteVariant promotes variant
func (h *HarnessHandler) PromoteVariant(ctx context.Context, req *pb.PromoteVariantRequest) (*commonpb.Empty, error) {
	return h.service.PromoteVariant(ctx, req)
}

// CreateSLO creates an SLO
func (h *HarnessHandler) CreateSLO(ctx context.Context, req *pb.CreateSLORequest) (*pb.SLO, error) {
	return h.service.CreateSLO(ctx, req)
}

// GetSLOStatus gets SLO status
func (h *HarnessHandler) GetSLOStatus(ctx context.Context, req *pb.GetSLOStatusRequest) (*pb.GetSLOStatusResponse, error) {
	return h.service.GetSLOStatus(ctx, req)
}

// Chat handles harness chat
func (h *HarnessHandler) Chat(ctx context.Context, req *pb.HarnessChatRequest) (*pb.HarnessChatResponse, error) {
	return h.service.Chat(ctx, req)
}

// ChatStream handles streaming harness chat
func (h *HarnessHandler) ChatStream(req *pb.HarnessChatRequest, stream pb.HarnessService_ChatStreamServer) error {
	return h.service.ChatStream(req, stream)
}