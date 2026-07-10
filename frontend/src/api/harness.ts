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
  toggleFlag: (key: string, enabled: boolean) => client.put('/api/v2/harness/flags/toggle', { key, enabled }),
  evaluateFlag: (key: string, context: any) => client.post('/api/v2/harness/flags/evaluate', { key, ...context }),
  deleteFlag: (id: string) => client.delete(`/api/v2/harness/flags/${id}`),

  // Scheduler
  listSchedules: () => client.get('/api/v2/harness/scheduler/schedules'),
  createSchedule: (schedule: any) => client.post('/api/v2/harness/scheduler/schedules', schedule),
  deleteSchedule: (id: string) => client.delete(`/api/v2/harness/scheduler/schedules/${id}`),
  pauseSchedule: (id: string) => client.put(`/api/v2/harness/scheduler/schedules/${id}/pause`),
  resumeSchedule: (id: string) => client.put(`/api/v2/harness/scheduler/schedules/${id}/resume`),
  runScheduleNow: (id: string) => client.post(`/api/v2/harness/scheduler/schedules/${id}/run`),

  // Approval
  listApprovalRules: () => client.get('/api/v2/harness/approval/rules'),
  listPendingApprovals: () => client.get('/api/v2/harness/approval/pending'),
  approveRequest: (id: string) => client.post('/api/v2/harness/approval/approve', { request_id: id }),
  rejectRequest: (id: string, reason: string) => client.post('/api/v2/harness/approval/reject', { request_id: id, reason }),
};
