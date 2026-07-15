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

export interface WorkflowNodeResult {
  node_id: string;
  output: string;
  error?: string;
  node_name?: string;
  node_type?: string;
  duration_ms?: number;
  retries?: number;
}

export interface WorkflowExecutionResult {
  workflow_id: string;
  execution_id?: string;
  status?: string;
  nodes: WorkflowNodeResult[];
  final_output: string;
  error?: string;
}

export interface WorkflowExecution {
  id: string;
  workflow_id: string;
  status: string;
  input: string;
  final_output: string;
  error?: string;
  node_results: WorkflowNodeResult[];
  started_at: number;
  completed_at: number;
  duration_ms: number;
}

export interface ValidateResult {
  valid: boolean;
  errors?: string[];
}

export interface WorkflowStreamChunk {
  type: string;            // node_started | node_completed | node_error | final | error | done
  node_id: string;
  node_name: string;
  node_type: string;
  output: string;
  error: string;
  final_result?: WorkflowExecutionResult;
}

export interface WorkflowStreamCallbacks {
  onChunk: (chunk: WorkflowStreamChunk) => void;
  onError: (error: Error) => void;
  onDone: () => void;
}

export const workflowApi = {
  list: () => client.get('/api/v2/harness/workflows'),
  get: (id: string) => client.get(`/api/v2/harness/workflows/${id}`),
  create: (wf: Partial<Workflow>) => client.post('/api/v2/harness/workflows', wf),
  update: (id: string, wf: Partial<Workflow>) => client.put(`/api/v2/harness/workflows/${id}`, wf),
  delete: (id: string) => client.delete(`/api/v2/harness/workflows/${id}`),
  execute: (id: string, input: string, timeoutSeconds?: number) =>
    client.post(`/api/v2/harness/workflows/${id}/execute`, { input, timeout_seconds: timeoutSeconds }),
  validate: (nodes: WorkflowNode[], edges: WorkflowEdge[], entryNodeId: string) =>
    client.post('/api/v2/harness/workflows/validate', {
      nodes: JSON.stringify(nodes),
      edges: JSON.stringify(edges),
      entry_node_id: entryNodeId,
    }),
  listExecutions: (workflowId: string, limit?: number) =>
    client.get(`/api/v2/harness/workflows/${workflowId}/executions`, { params: { limit } }),
  getExecution: (executionId: string) =>
    client.get(`/api/v2/harness/workflows/executions/${executionId}`),
  cancelExecution: (executionId: string) =>
    client.post(`/api/v2/harness/workflows/executions/${executionId}/cancel`),

  // Streaming workflow execution — SSE, same pattern as chatStream
  executeStream: (id: string, input: string, callbacks: WorkflowStreamCallbacks, timeoutSeconds?: number): AbortController => {
    const controller = new AbortController();
    const baseUrl = import.meta.env.VITE_API_URL || window.location.origin;
    const url = new URL(`/api/v2/harness/workflows/${id}/execute-stream`, baseUrl);

    fetch(url.toString(), {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Tenant-ID': localStorage.getItem('tenantId') || 'default',
      },
      body: JSON.stringify({ input, timeout_seconds: timeoutSeconds }),
      signal: controller.signal,
    }).then(async (response) => {
      if (!response.ok) {
        callbacks.onError(new Error(`HTTP ${response.status}`));
        return;
      }
      const reader = response.body?.getReader();
      const decoder = new TextDecoder();
      if (!reader) { callbacks.onError(new Error('No readable stream')); return; }

      let buffer = '';
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';
        for (const line of lines) {
          if (line.startsWith('event:')) continue;
          if (line.startsWith('data:')) {
            const data = line.slice(5).trim();
            if (!data) continue;
            try {
              const parsed = JSON.parse(data) as WorkflowStreamChunk;
              if (parsed.type === 'done') { callbacks.onDone(); return; }
              callbacks.onChunk(parsed);
            } catch { /* ignore non-JSON */ }
          }
        }
      }
      callbacks.onDone();
    }).catch((error) => {
      if (error.name === 'AbortError') { callbacks.onDone(); return; }
      callbacks.onError(error);
    });

    return controller;
  },
};
