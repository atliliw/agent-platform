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

// ==================== Feature Flag Methods ====================

func (h *HarnessHandler) CreateFeatureFlag(ctx context.Context, req *pb.CreateFeatureFlagRequest) (*pb.FeatureFlag, error) {
	return h.service.CreateFeatureFlag(ctx, req)
}

func (h *HarnessHandler) ListFeatureFlags(ctx context.Context, req *pb.ListFeatureFlagsRequest) (*pb.ListFeatureFlagsResponse, error) {
	return h.service.ListFeatureFlags(ctx, req)
}

func (h *HarnessHandler) GetFeatureFlag(ctx context.Context, req *pb.GetFeatureFlagRequest) (*pb.FeatureFlag, error) {
	return h.service.GetFeatureFlag(ctx, req)
}

func (h *HarnessHandler) ToggleFeatureFlag(ctx context.Context, req *pb.ToggleFeatureFlagRequest) (*pb.FeatureFlag, error) {
	return h.service.ToggleFeatureFlag(ctx, req)
}

func (h *HarnessHandler) DeleteFeatureFlag(ctx context.Context, req *pb.GetFeatureFlagRequest) (*commonpb.Empty, error) {
	return h.service.DeleteFeatureFlag(ctx, req)
}

func (h *HarnessHandler) EvaluateFeatureFlag(ctx context.Context, req *pb.EvaluateFeatureFlagRequest) (*pb.EvaluateFeatureFlagResponse, error) {
	return h.service.EvaluateFeatureFlagGRPC(ctx, req)
}

// ==================== Chaos Methods ====================

func (h *HarnessHandler) CreateChaosExperiment(ctx context.Context, req *pb.CreateChaosExperimentRequest) (*pb.ChaosExperiment, error) {
	return h.service.CreateChaosExperiment(ctx, req)
}

func (h *HarnessHandler) StartChaosExperiment(ctx context.Context, req *pb.StartChaosExperimentRequest) (*pb.ChaosExperiment, error) {
	return h.service.StartChaosExperiment(ctx, req)
}

func (h *HarnessHandler) StopChaosExperiment(ctx context.Context, req *pb.StopChaosExperimentRequest) (*pb.ChaosExperiment, error) {
	return h.service.StopChaosExperiment(ctx, req)
}

func (h *HarnessHandler) ListChaosExperiments(ctx context.Context, req *pb.ListChaosExperimentsRequest) (*pb.ListChaosExperimentsResponse, error) {
	return h.service.ListChaosExperiments(ctx, req)
}

// ==================== Rollback Methods ====================

func (h *HarnessHandler) CreateRollbackConfig(ctx context.Context, req *pb.CreateRollbackConfigRequest) (*pb.RollbackConfig, error) {
	return h.service.CreateRollbackConfig(ctx, req)
}

func (h *HarnessHandler) GetRollbackConfig(ctx context.Context, req *pb.GetFeatureFlagRequest) (*pb.RollbackConfig, error) {
	return h.service.GetRollbackConfigGRPC(ctx, req)
}

func (h *HarnessHandler) TakeSnapshot(ctx context.Context, req *pb.TakeSnapshotRequest) (*pb.ConfigSnapshot, error) {
	return h.service.TakeSnapshot(ctx, req)
}

func (h *HarnessHandler) ListSnapshots(ctx context.Context, req *pb.ListSnapshotsRequest) (*pb.ListSnapshotsResponse, error) {
	return h.service.ListSnapshots(ctx, req)
}

func (h *HarnessHandler) ExecuteRollback(ctx context.Context, req *pb.ExecuteRollbackRequest) (*pb.RollbackEvent, error) {
	return h.service.ExecuteRollback(ctx, req)
}

// ==================== RCA Methods ====================

func (h *HarnessHandler) RecordChange(ctx context.Context, req *pb.RecordChangeRequest) (*pb.ChangeEvent, error) {
	return h.service.RecordChange(ctx, req)
}

func (h *HarnessHandler) Analyze(ctx context.Context, req *pb.AnalyzeRequest) (*pb.AnalysisReport, error) {
	return h.service.Analyze(ctx, req)
}

// ==================== Cost Methods ====================

func (h *HarnessHandler) SetModelPricing(ctx context.Context, req *pb.SetModelPricingRequest) (*pb.ModelPricing, error) {
	return h.service.SetModelPricing(ctx, req)
}

func (h *HarnessHandler) ListModelPricing(ctx context.Context, req *commonpb.Empty) (*pb.ListModelPricingResponse, error) {
	return h.service.ListModelPricing(ctx, req)
}

func (h *HarnessHandler) GetCostReport(ctx context.Context, req *pb.CostReportRequest) (*pb.CostReport, error) {
	return h.service.GetCostReportGRPC(ctx, req)
}

func (h *HarnessHandler) GetCostRecommendations(ctx context.Context, req *commonpb.Empty) (*pb.ListCostRecommendationsResponse, error) {
	return h.service.GetCostRecommendations(ctx, req)
}

// ==================== Evolve Methods ====================

func (h *HarnessHandler) CreateProposal(ctx context.Context, req *pb.CreateProposalRequest) (*pb.Proposal, error) {
	return h.service.CreateProposal(ctx, req)
}

func (h *HarnessHandler) ListProposals(ctx context.Context, req *pb.ListProposalsRequest) (*pb.ListProposalsResponse, error) {
	return h.service.ListProposals(ctx, req)
}

func (h *HarnessHandler) ApproveProposal(ctx context.Context, req *pb.ApproveProposalRequest) (*pb.Proposal, error) {
	return h.service.ApproveProposal(ctx, req)
}

func (h *HarnessHandler) RejectProposal(ctx context.Context, req *pb.RejectProposalRequest) (*pb.Proposal, error) {
	return h.service.RejectProposal(ctx, req)
}

func (h *HarnessHandler) RunOptimizer(ctx context.Context, req *pb.RunOptimizerRequest) (*pb.OptimizationResult, error) {
	return h.service.RunOptimizerGRPC(ctx, req)
}

// ==================== Catalog Methods ====================

func (h *HarnessHandler) ListCatalogAgents(ctx context.Context, req *pb.ListCatalogAgentsRequest) (*pb.ListCatalogAgentsResponse, error) {
	return h.service.ListCatalogAgents(ctx, req)
}

func (h *HarnessHandler) GetCatalogAgent(ctx context.Context, req *pb.GetFeatureFlagRequest) (*pb.CatalogAgent, error) {
	return h.service.GetCatalogAgentGRPC(ctx, req)
}

// ==================== Golden Path Methods ====================

func (h *HarnessHandler) CreateGoldenPathTemplate(ctx context.Context, req *pb.CreateGoldenPathTemplateRequest) (*pb.GoldenPathTemplate, error) {
	return h.service.CreateGoldenPathTemplate(ctx, req)
}

func (h *HarnessHandler) ListGoldenPathTemplates(ctx context.Context, req *pb.ListGoldenPathTemplatesRequest) (*pb.ListGoldenPathTemplatesResponse, error) {
	return h.service.ListGoldenPathTemplates(ctx, req)
}

func (h *HarnessHandler) InstantiateTemplate(ctx context.Context, req *pb.InstantiateTemplateRequest) (*commonpb.Empty, error) {
	return h.service.InstantiateTemplate(ctx, req)
}