import client from './client';

export const harnessApi = {
  // Rules
  listRules: () => client.get('/api/v2/harness/rules'),
  createRule: (rule: any) => client.post('/api/v2/harness/rules', rule),
  deleteRule: (id: string) => client.delete(`/api/v2/harness/rules/${id}`),
  checkGuardrail: (message: string) => client.post('/api/v2/harness/guardrail/check', { message }),

  // AB Tests
  listABTests: () => client.post('/api/v2/harness/abtest/list', {}),
  createABTest: (test: any) => client.post('/api/v2/harness/abtest', test),
  deleteABTest: (id: string) => client.delete(`/api/v2/harness/abtest/${id}`),
  getABTestResult: (id: string) => client.get(`/api/v2/harness/abtest/${id}/result`),

  // SLO
  getSLOStatus: () => client.get('/api/v2/harness/slo/status'),
  createSLO: (slo: any) => client.post('/api/v2/harness/slo', slo),

  // Cost
  getCostReport: (params?: { start?: string; end?: string }) => client.get('/api/v2/harness/cost/report', { params }),
  getPricing: () => client.get('/api/v2/harness/cost/pricing'),
  updatePricing: (pricing: any) => client.post('/api/v2/harness/cost/pricing', pricing),
  getRecommendations: () => client.get('/api/v2/harness/cost/recommendations'),
  recordUsage: (usage: any) => client.post('/api/v2/harness/cost/usage', usage),

  // Proposals
  listProposals: () => client.get('/api/v2/harness/proposals'),
  createProposal: (proposal: any) => client.post('/api/v2/harness/proposals', proposal),
  approveProposal: (id: string) => client.post(`/api/v2/harness/proposals/${id}/approve`, { approved_by: 'admin' }),
  rejectProposal: (id: string, reason: string) => client.post(`/api/v2/harness/proposals/${id}/reject`, { reason }),
  executeProposal: (id: string) => client.post(`/api/v2/harness/proposals/${id}/execute`),
  analyzeProposals: () => client.post('/api/v2/harness/proposals/analyze'),

  // LLM Metrics
  getLLMMetrics: (params?: any) => client.get('/api/v2/harness/llm/metrics', { params }),

  // Feature Flags
  listFlags: () => client.get('/api/v2/harness/flags'),
  createFlag: (flag: any) => client.post('/api/v2/harness/flags', flag),
  toggleFlag: (id: string, enabled: boolean) => client.post(`/api/v2/harness/flags/${id}/toggle`, { enabled }),
  evaluateFlag: (id: string, context: any) => client.post(`/api/v2/harness/flags/${id}/evaluate`, context),
  deleteFlag: (id: string) => client.delete(`/api/v2/harness/flags/${id}`),

  // Scheduler
  listSchedules: () => client.get('/api/v2/harness/scheduler/schedules'),
  createSchedule: (schedule: any) => client.post('/api/v2/harness/scheduler/schedules', schedule),
  deleteSchedule: (id: string) => client.delete(`/api/v2/harness/scheduler/schedules/${id}`),
  pauseSchedule: (id: string) => client.post(`/api/v2/harness/scheduler/schedules/${id}/pause`),
  resumeSchedule: (id: string) => client.post(`/api/v2/harness/scheduler/schedules/${id}/resume`),
  runScheduleNow: (id: string) => client.post(`/api/v2/harness/scheduler/schedules/${id}/run`),

  // Catalog
  listCatalog: () => client.get('/api/v2/harness/catalog'),

  // Rollback
  listSnapshots: (params?: any) => client.get('/api/v2/harness/rollback/snapshots', { params }),
  createSnapshot: (snapshot: any) => client.post('/api/v2/harness/rollback/snapshots', snapshot),
  rollbackSnapshot: (id: string) => client.post(`/api/v2/harness/rollback/snapshots/${id}/rollback`),

  // Chaos
  listChaosExperiments: () => client.get('/api/v2/harness/chaos/experiments'),
  createChaosExperiment: (exp: any) => client.post('/api/v2/harness/chaos/experiments', exp),
  startChaosExperiment: (id: string) => client.post(`/api/v2/harness/chaos/experiments/${id}/start`),
  stopChaosExperiment: (id: string) => client.post(`/api/v2/harness/chaos/experiments/${id}/stop`),
  deleteChaosExperiment: (id: string) => client.delete(`/api/v2/harness/chaos/experiments/${id}`),

  // Approval
  listApprovalRules: () => client.get('/api/v2/harness/approval/rules'),
  listPendingApprovals: () => client.get('/api/v2/harness/approval/pending'),
  approveRequest: (id: string) => client.post(`/api/v2/harness/approval/${id}/approve`),
  rejectRequest: (id: string, reason: string) => client.post(`/api/v2/harness/approval/${id}/reject`, { reason }),
};
