// Session API - Session Replay Feature
import client from './client';

export interface Session {
  id: string;
  agent_id?: string;
  trace_id?: string;
  status?: string;
  start_time?: number;
  end_time?: number;
  duration?: number;
  total_tokens?: number;
  total_cost?: number;
  model?: string;
  metadata?: Record<string, string>;
  created_at?: number;
}

export interface SessionStep {
  id: string;
  session_id: string;
  step_number: number;
  step_type?: string;
  parent_step_id?: string;
  input?: string;
  output?: string;
  metadata?: string;
  duration?: number;
  status?: string;
  timestamp?: number;
}

export interface ExecutionGraph {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

export interface ReplayState {
  session: Session;
  steps: SessionStep[];
  graph: ExecutionGraph;
  current_step_index?: number;
  is_playing?: boolean;
  playback_speed?: number;
}

export interface SessionStats {
  total_sessions?: number;
  running_sessions?: number;
  completed_sessions?: number;
  failed_sessions?: number;
  total_tokens?: number;
  total_cost?: number;
  avg_duration?: number;
}

export interface GraphNode {
  id: string;
  type?: string;
  label?: string;
  duration?: number;
  status?: string;
  metadata?: Record<string, string>;
}

export interface GraphEdge {
  from?: string;
  to?: string;
  label?: string;
}

export const sessionApi = {
  createSession: (data: { agent_id: string; model?: string; metadata?: Record<string, string> }) =>
    client.post('/api/v2/harness/session', data),

  getSession: (session_id: string) =>
    client.get(`/api/v2/harness/session/${session_id}`),

  listSessions: (params?: { agent_id?: string; limit?: number }) =>
    client.get('/api/v2/harness/session/list', { params }),

  deleteSession: (session_id: string) =>
    client.delete(`/api/v2/harness/session/${session_id}`),

  replaySession: (session_id: string) =>
    client.post(`/api/v2/harness/session/${session_id}/replay`),

  getSessionGraph: (session_id: string) =>
    client.get(`/api/v2/harness/session/${session_id}/graph`),

  exportSession: (session_id: string, format = 'json') => {
    const baseUrl = import.meta.env.VITE_API_URL || '';
    return `${baseUrl}/api/v2/harness/session/${session_id}/export?format=${format}`;
  },

  getSessionStats: (params?: { agent_id?: string }) =>
    client.get('/api/v2/harness/session/stats', { params }),
};
