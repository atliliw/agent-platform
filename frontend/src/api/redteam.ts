// Red Team API - Security Testing
import client from './client';

export interface RedTeamTest {
  id: string;
  name: string;
  description: string;
  agent_id: string;
  model: string;
  category: string;
  status: string;
  config: string;
  start_time: number;
  end_time: number;
  tenant_id: string;
  created_at: number;
  updated_at: number;
}

export interface RedTeamReport {
  id: string;
  test_id: string;
  total_attacks: number;
  passed_attacks: number;
  failed_attacks: number;
  blocked_attacks: number;
  critical_count: number;
  high_count: number;
  medium_count: number;
  low_count: number;
  risk_score: number;
  security_level: string;
  vulnerabilities: string;
  recommendations: string;
  generatedAt: number;
}

export interface AttackPayload {
  id: string;
  type: string;
  name: string;
  description: string;
  payload: string;
  expected: string;
  severity: string;
  tags: string[];
}

export const redteamApi = {
  createTest: (data: { name: string; description: string; agent_id: string; model: string; category: string; config?: string }) =>
    client.post('/api/v2/harness/redteam', data),

  listTests: (params?: { agent_id?: string; status?: string }) =>
    client.get('/api/v2/harness/redteam/list', { params }),

  getTest: (id: string) =>
    client.get(`/api/v2/harness/redteam/${id}`),

  runTest: (id: string) =>
    client.post(`/api/v2/harness/redteam/${id}/run`),

  getReport: (id: string) =>
    client.get(`/api/v2/harness/redteam/${id}/report`),

  getAttacks: (id: string) =>
    client.get(`/api/v2/harness/redteam/${id}/attacks`),

  deleteTest: (id: string) =>
    client.delete(`/api/v2/harness/redteam/${id}`),

  getAttackPayloads: (params?: { category?: string }) =>
    client.get('/api/v2/harness/redteam/payloads', { params }),
};
