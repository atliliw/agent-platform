// Package handler provides gRPC handlers for Harness service
package handler

import (
	"context"

	commonpb "agent-platform/pkg/pb/common"
	pb "agent-platform/pkg/pb/harness"
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

// ListABTests lists A/B tests
func (h *HarnessHandler) ListABTests(ctx context.Context, req *pb.ListABTestsRequest) (*pb.ListABTestsResponse, error) {
	return h.service.ListABTests(ctx, req)
}

// GetABTestResult gets A/B test result
func (h *HarnessHandler) GetABTestResult(ctx context.Context, req *pb.GetABTestResultRequest) (*pb.ABTestResult, error) {
	return h.service.GetABTestResult(ctx, req)
}

// ShouldUseVariant determines if a request should use variant
func (h *HarnessHandler) ShouldUseVariant(ctx context.Context, req *pb.ShouldUseVariantRequest) (*pb.ShouldUseVariantResponse, error) {
	return h.service.ShouldUseVariant(ctx, req)
}

// RecordABTestResult records A/B test result
func (h *HarnessHandler) RecordABTestResult(ctx context.Context, req *pb.RecordABTestResultRequest) (*commonpb.Empty, error) {
	return h.service.RecordABTestResult(ctx, req)
}

// DeleteABTest deletes an A/B test
func (h *HarnessHandler) DeleteABTest(ctx context.Context, req *pb.PromoteVariantRequest) (*commonpb.Empty, error) {
	return h.service.DeleteABTest(ctx, req)
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
	return h.service.EvaluateFeatureFlag(ctx, req)
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
	return h.service.GetRollbackConfig(ctx, req)
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
	return h.service.GetCostReport(ctx, req)
}

func (h *HarnessHandler) GetCostRecommendations(ctx context.Context, req *commonpb.Empty) (*pb.ListCostRecommendationsResponse, error) {
	return h.service.GetCostRecommendations(ctx, req)
}

func (h *HarnessHandler) RecordCostUsage(ctx context.Context, req *pb.RecordCostUsageRequest) (*commonpb.Empty, error) {
	return h.service.RecordCostUsage(ctx, req)
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

func (h *HarnessHandler) ExecuteProposal(ctx context.Context, req *pb.ApproveProposalRequest) (*pb.Proposal, error) {
	return h.service.ExecuteProposal(ctx, req)
}

func (h *HarnessHandler) RunOptimizer(ctx context.Context, req *pb.RunOptimizerRequest) (*pb.OptimizationResult, error) {
	return h.service.RunOptimizer(ctx, req)
}

// ==================== Catalog Methods ====================

func (h *HarnessHandler) ListCatalogAgents(ctx context.Context, req *pb.ListCatalogAgentsRequest) (*pb.ListCatalogAgentsResponse, error) {
	return h.service.ListCatalogAgents(ctx, req)
}

func (h *HarnessHandler) GetCatalogAgent(ctx context.Context, req *pb.GetFeatureFlagRequest) (*pb.CatalogAgent, error) {
	return h.service.GetCatalogAgent(ctx, req)
}

func (h *HarnessHandler) RegisterCatalogAgent(ctx context.Context, req *pb.RegisterCatalogAgentRequest) (*pb.CatalogAgent, error) {
	return h.service.RegisterCatalogAgent(ctx, req)
}

func (h *HarnessHandler) RecordCatalogUsage(ctx context.Context, req *pb.RecordCatalogUsageRequest) (*commonpb.Empty, error) {
	return h.service.RecordCatalogUsage(ctx, req)
}

func (h *HarnessHandler) RateCatalogAgent(ctx context.Context, req *pb.RateCatalogAgentRequest) (*commonpb.Empty, error) {
	return h.service.RateCatalogAgent(ctx, req)
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

// ==================== Scheduler Methods ====================

func (h *HarnessHandler) SetEvalSchedule(ctx context.Context, req *pb.SetEvalScheduleRequest) (*pb.EvalSchedule, error) {
	return h.service.SetEvalSchedule(ctx, req)
}

func (h *HarnessHandler) GetEvalSchedule(ctx context.Context, req *pb.GetEvalScheduleRequest) (*pb.EvalSchedule, error) {
	return h.service.GetEvalSchedule(ctx, req)
}

func (h *HarnessHandler) ListEvalSchedules(ctx context.Context, req *pb.ListEvalSchedulesRequest) (*pb.ListEvalSchedulesResponse, error) {
	return h.service.ListEvalSchedules(ctx, req)
}

func (h *HarnessHandler) PauseEvalSchedule(ctx context.Context, req *pb.PauseScheduleRequest) (*pb.EvalSchedule, error) {
	return h.service.PauseEvalSchedule(ctx, req)
}

func (h *HarnessHandler) ResumeEvalSchedule(ctx context.Context, req *pb.ResumeScheduleRequest) (*pb.EvalSchedule, error) {
	return h.service.ResumeEvalSchedule(ctx, req)
}

func (h *HarnessHandler) DeleteEvalSchedule(ctx context.Context, req *pb.GetEvalScheduleRequest) (*commonpb.Empty, error) {
	return h.service.DeleteEvalSchedule(ctx, req)
}

func (h *HarnessHandler) RunEvalScheduleNow(ctx context.Context, req *pb.RunScheduleNowRequest) (*pb.ScheduledEvalResult, error) {
	return h.service.RunEvalScheduleNow(ctx, req)
}

func (h *HarnessHandler) GetEvalScheduleResults(ctx context.Context, req *pb.GetScheduleResultsRequest) (*pb.GetScheduleResultsResponse, error) {
	return h.service.GetEvalScheduleResults(ctx, req)
}

func (h *HarnessHandler) GetSchedulerStatus(ctx context.Context, req *commonpb.Empty) (*pb.SchedulerStatus, error) {
	return h.service.GetSchedulerStatus(ctx, req)
}

func (h *HarnessHandler) SchedulerControl(ctx context.Context, req *pb.SchedulerControlRequest) (*pb.SchedulerStatus, error) {
	return h.service.SchedulerControl(ctx, req)
}

func (h *HarnessHandler) GetSchedulerStats(ctx context.Context, req *commonpb.Empty) (*pb.SchedulerStatsResponse, error) {
	return h.service.GetSchedulerStats(ctx, req)
}

// RecordLLMMetrics records LLM call metrics from external services
func (h *HarnessHandler) RecordLLMMetrics(ctx context.Context, req *pb.RecordLLMMetricsRequest) (*commonpb.Empty, error) {
	return h.service.RecordLLMMetrics(ctx, req)
}

// GetLLMMetrics gets LLM metrics summary
func (h *HarnessHandler) GetLLMMetrics(ctx context.Context, req *pb.GetLLMMetricsRequest) (*pb.LLMMetricsSummary, error) {
	return h.service.GetLLMMetricsPB(ctx, req)
}

// AnalyzeAndPropose analyzes cost/SLO data and generates proposals
func (h *HarnessHandler) AnalyzeAndPropose(ctx context.Context, req *pb.AnalyzeAndProposeRequest) (*pb.AnalyzeAndProposeResponse, error) {
	return h.service.AnalyzeAndPropose(ctx, req)
}

// ==================== Playground Methods ====================

func (h *HarnessHandler) ExecutePlayground(ctx context.Context, req *pb.PlaygroundRequest) (*pb.PlaygroundResult, error) {
	return h.service.ExecutePlayground(ctx, req)
}

func (h *HarnessHandler) CompareModels(ctx context.Context, req *pb.CompareModelsRequest) (*pb.CompareModelsResponse, error) {
	return h.service.CompareModels(ctx, req)
}

func (h *HarnessHandler) StreamPlayground(req *pb.PlaygroundRequest, stream pb.HarnessService_StreamPlaygroundServer) error {
	return h.service.StreamPlayground(req, stream)
}

func (h *HarnessHandler) GetPlaygroundHistory(ctx context.Context, req *pb.GetPlaygroundHistoryRequest) (*pb.GetPlaygroundHistoryResponse, error) {
	return h.service.GetPlaygroundHistory(ctx, req)
}

func (h *HarnessHandler) DeletePlaygroundHistory(ctx context.Context, req *pb.DeletePlaygroundHistoryRequest) (*commonpb.Empty, error) {
	return h.service.DeletePlaygroundHistory(ctx, req)
}

func (h *HarnessHandler) GetPlaygroundStats(ctx context.Context, req *pb.GetPlaygroundStatsRequest) (*pb.PlaygroundStats, error) {
	return h.service.GetPlaygroundStats(ctx, req)
}

// ==================== Session Replay Methods ====================

func (h *HarnessHandler) CreateSession(ctx context.Context, req *pb.CreateSessionRequest) (*pb.CreateSessionResponse, error) {
	return h.service.CreateSession(ctx, req)
}

func (h *HarnessHandler) GetSession(ctx context.Context, req *pb.GetSessionRequest) (*pb.SessionDetail, error) {
	return h.service.GetSession(ctx, req)
}

func (h *HarnessHandler) ListSessions(ctx context.Context, req *pb.ListSessionsRequest) (*pb.ListSessionsResponse, error) {
	return h.service.ListSessions(ctx, req)
}

func (h *HarnessHandler) RecordStep(ctx context.Context, req *pb.RecordStepRequest) (*pb.RecordStepResponse, error) {
	return h.service.RecordStep(ctx, req)
}

func (h *HarnessHandler) EndSession(ctx context.Context, req *pb.EndSessionRequest) (*pb.EndSessionResponse, error) {
	return h.service.EndSession(ctx, req)
}

func (h *HarnessHandler) ReplaySession(ctx context.Context, req *pb.ReplaySessionRequest) (*pb.ReplaySessionResponse, error) {
	return h.service.ReplaySession(ctx, req)
}

func (h *HarnessHandler) GetSessionGraph(ctx context.Context, req *pb.GetSessionGraphRequest) (*pb.SessionGraph, error) {
	return h.service.GetSessionGraph(ctx, req)
}

func (h *HarnessHandler) ExportSession(ctx context.Context, req *pb.ExportSessionRequest) (*pb.ExportSessionResponse, error) {
	return h.service.ExportSession(ctx, req)
}

func (h *HarnessHandler) DeleteSession(ctx context.Context, req *pb.GetSessionRequest) (*commonpb.Empty, error) {
	return h.service.DeleteSession(ctx, req)
}

// ==================== Prompt Management Methods ====================

func (h *HarnessHandler) CreatePrompt(ctx context.Context, req *pb.CreatePromptRequest) (*pb.Prompt, error) {
	return h.service.CreatePrompt(ctx, req)
}

func (h *HarnessHandler) GetPrompt(ctx context.Context, req *pb.GetPromptRequest) (*pb.Prompt, error) {
	return h.service.GetPrompt(ctx, req)
}

func (h *HarnessHandler) ListPrompts(ctx context.Context, req *pb.ListPromptsRequest) (*pb.ListPromptsResponse, error) {
	return h.service.ListPrompts(ctx, req)
}

func (h *HarnessHandler) DeletePrompt(ctx context.Context, req *pb.GetPromptRequest) (*commonpb.Empty, error) {
	return h.service.DeletePrompt(ctx, req)
}

func (h *HarnessHandler) CreatePromptVersion(ctx context.Context, req *pb.CreatePromptVersionRequest) (*pb.PromptVersion, error) {
	return h.service.CreatePromptVersion(ctx, req)
}

func (h *HarnessHandler) GetPromptVersion(ctx context.Context, req *pb.GetPromptVersionRequest) (*pb.PromptVersion, error) {
	return h.service.GetPromptVersion(ctx, req)
}

func (h *HarnessHandler) GetActivePromptVersion(ctx context.Context, req *pb.GetActivePromptVersionRequest) (*pb.PromptVersion, error) {
	return h.service.GetActivePromptVersion(ctx, req)
}

func (h *HarnessHandler) ListPromptVersions(ctx context.Context, req *pb.ListPromptVersionsRequest) (*pb.ListPromptVersionsResponse, error) {
	return h.service.ListPromptVersions(ctx, req)
}

func (h *HarnessHandler) ActivatePromptVersion(ctx context.Context, req *pb.ActivatePromptVersionRequest) (*pb.PromptVersion, error) {
	return h.service.ActivatePromptVersion(ctx, req)
}

func (h *HarnessHandler) ArchivePromptVersion(ctx context.Context, req *pb.ArchivePromptVersionRequest) (*pb.PromptVersion, error) {
	return h.service.ArchivePromptVersion(ctx, req)
}

func (h *HarnessHandler) RollbackPromptVersion(ctx context.Context, req *pb.ActivatePromptVersionRequest) (*pb.PromptVersion, error) {
	return h.service.RollbackPromptVersion(ctx, req)
}

func (h *HarnessHandler) ComparePromptVersions(ctx context.Context, req *pb.ComparePromptVersionsRequest) (*pb.PromptVersionDiff, error) {
	return h.service.ComparePromptVersions(ctx, req)
}

func (h *HarnessHandler) RenderPrompt(ctx context.Context, req *pb.RenderPromptRequest) (*pb.RenderPromptResponse, error) {
	return h.service.RenderPrompt(ctx, req)
}

func (h *HarnessHandler) RecordPromptUsage(ctx context.Context, req *pb.RecordPromptUsageRequest) (*commonpb.Empty, error) {
	return h.service.RecordPromptUsage(ctx, req)
}

func (h *HarnessHandler) GetPromptPerformance(ctx context.Context, req *pb.GetPromptPerformanceRequest) (*pb.PromptPerformance, error) {
	return h.service.GetPromptPerformance(ctx, req)
}

func (h *HarnessHandler) GetPromptPerformanceTrend(ctx context.Context, req *pb.GetPromptPerformanceTrendRequest) (*pb.PromptPerformanceTrend, error) {
	return h.service.GetPromptPerformanceTrend(ctx, req)
}

// ==================== RAG Metrics ====================

func (h *HarnessHandler) EvaluateRAG(ctx context.Context, req *pb.EvaluateRAGRequest) (*pb.RAGMetrics, error) {
	return h.service.EvaluateRAG(ctx, req)
}

func (h *HarnessHandler) BatchEvaluateRAG(ctx context.Context, req *pb.BatchEvaluateRAGRequest) (*pb.BatchEvaluateRAGResponse, error) {
	return h.service.BatchEvaluateRAG(ctx, req)
}

func (h *HarnessHandler) GetRAGMetrics(ctx context.Context, req *pb.GetRAGMetricsRequest) (*pb.RAGMetrics, error) {
	return h.service.GetRAGMetrics(ctx, req)
}

func (h *HarnessHandler) ListRAGMetrics(ctx context.Context, req *pb.ListRAGMetricsRequest) (*pb.ListRAGMetricsResponse, error) {
	return h.service.ListRAGMetrics(ctx, req)
}

func (h *HarnessHandler) CreateRAGEvaluation(ctx context.Context, req *pb.CreateRAGEvaluationRequest) (*pb.RAGEvaluation, error) {
	return h.service.CreateRAGEvaluation(ctx, req)
}

func (h *HarnessHandler) GetRAGEvaluation(ctx context.Context, req *pb.GetRAGEvaluationRequest) (*pb.RAGEvaluation, error) {
	return h.service.GetRAGEvaluation(ctx, req)
}

func (h *HarnessHandler) ListRAGEvaluations(ctx context.Context, req *pb.ListRAGEvaluationsRequest) (*pb.ListRAGEvaluationsResponse, error) {
	return h.service.ListRAGEvaluations(ctx, req)
}

func (h *HarnessHandler) RunRAGEvaluation(ctx context.Context, req *pb.RunRAGEvaluationRequest) (*pb.RunRAGEvaluationResponse, error) {
	return h.service.RunRAGEvaluation(ctx, req)
}

// ==================== Checkpoint ====================

// ListCheckpoints lists checkpoints for a session
func (h *HarnessHandler) ListCheckpoints(ctx context.Context, req *pb.ListCheckpointsRequest) (*pb.ListCheckpointsResponse, error) {
	return h.service.ListCheckpoints(ctx, req)
}

// GetCheckpoint gets a specific checkpoint
func (h *HarnessHandler) GetCheckpoint(ctx context.Context, req *pb.GetCheckpointRequest) (*pb.GetCheckpointResponse, error) {
	return h.service.GetCheckpoint(ctx, req)
}

// ResumeFromCheckpoint resumes execution from a checkpoint
func (h *HarnessHandler) ResumeFromCheckpoint(ctx context.Context, req *pb.ResumeFromCheckpointRequest) (*pb.ResumeFromCheckpointResponse, error) {
	return h.service.ResumeFromCheckpoint(ctx, req)
}

// CreateWorkflow creates a workflow
func (h *HarnessHandler) CreateWorkflow(ctx context.Context, req *pb.CreateWorkflowRequest) (*pb.Workflow, error) {
	return h.service.CreateWorkflow(ctx, req)
}

// GetWorkflow gets a workflow
func (h *HarnessHandler) GetWorkflow(ctx context.Context, req *pb.GetWorkflowRequest) (*pb.Workflow, error) {
	return h.service.GetWorkflow(ctx, req)
}

// UpdateWorkflow updates a workflow
func (h *HarnessHandler) UpdateWorkflow(ctx context.Context, req *pb.UpdateWorkflowRequest) (*pb.Workflow, error) {
	return h.service.UpdateWorkflow(ctx, req)
}

// ListWorkflows lists workflows
func (h *HarnessHandler) ListWorkflows(ctx context.Context, req *pb.ListWorkflowsRequest) (*pb.ListWorkflowsResponse, error) {
	return h.service.ListWorkflows(ctx, req)
}

// DeleteWorkflow deletes a workflow
func (h *HarnessHandler) DeleteWorkflow(ctx context.Context, req *pb.DeleteWorkflowRequest) (*commonpb.Empty, error) {
	return h.service.DeleteWorkflow(ctx, req)
}

// ExecuteWorkflow executes a workflow
func (h *HarnessHandler) ExecuteWorkflow(ctx context.Context, req *pb.ExecuteWorkflowRequest) (*pb.ExecuteWorkflowResponse, error) {
	return h.service.ExecuteWorkflow(ctx, req)
}

// ValidateWorkflow validates a workflow
func (h *HarnessHandler) ValidateWorkflow(ctx context.Context, req *pb.ValidateWorkflowRequest) (*pb.ValidateWorkflowResponse, error) {
	return h.service.ValidateWorkflow(ctx, req)
}

// GetWorkflowExecution gets a workflow execution
func (h *HarnessHandler) GetWorkflowExecution(ctx context.Context, req *pb.GetWorkflowExecutionRequest) (*pb.WorkflowExecution, error) {
	return h.service.GetWorkflowExecution(ctx, req)
}

// ListWorkflowExecutions lists workflow executions
func (h *HarnessHandler) ListWorkflowExecutions(ctx context.Context, req *pb.ListWorkflowExecutionsRequest) (*pb.ListWorkflowExecutionsResponse, error) {
	return h.service.ListWorkflowExecutions(ctx, req)
}

// CancelWorkflowExecution cancels a workflow execution
func (h *HarnessHandler) CancelWorkflowExecution(ctx context.Context, req *pb.CancelWorkflowExecutionRequest) (*commonpb.Empty, error) {
	return h.service.CancelWorkflowExecution(ctx, req)
}
