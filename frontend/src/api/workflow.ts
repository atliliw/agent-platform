import client from './client';

export interface WorkflowNode {
  id: string;
  type: string;
  name: string;
  agent_id?: string;
  tool_name?: string;
  condition?: string;
  config?: Record<string, unknown>;
  position?: { x: number; y: number };
}

export interface WorkflowEdge {
  id: string;
  from: string;
  to: string;
  condition?: string;
  label?: string;
}

export interface Workflow {
  id: string;
  name: string;
  description?: string;
  nodes: WorkflowNode[];
  edges: WorkflowEdge[];
  entry_node_id: string;
  tenant_id?: string;
  created_at: number;
  updated_at: number;
}

export interface WorkflowExecutionResult {
  workflow_id: string;
  nodes: Array<{
    node_id: string;
    output: string;
    error?: string;
  }>;
  final_output: string;
  error?: string;
}

export const workflowApi = {
  list: () => client.get('/api/v2/harness/workflows'),
  get: (id: string) => client.get(`/api/v2/harness/workflows/${id}`),
  create: (wf: Partial<Workflow>) => client.post('/api/v2/harness/workflows', wf),
  delete: (id: string) => client.delete(`/api/v2/harness/workflows/${id}`),
  execute: (id: string, input: string) =>
    client.post(`/api/v2/harness/workflows/${id}/execute`, { input }),
};
