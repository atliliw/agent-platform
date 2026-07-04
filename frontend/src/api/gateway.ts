import client from './client';

// ========================= Types =========================

export interface GatewayProvider {
  id: string;
  name: string;
  provider: string;
  api_key: string;
  base_url: string;
  models: string;          // JSON string from backend
  rate_limit: number;
  timeout: number;
  retry_count: number;
  priority: number;
  enabled: boolean;
  description: string;
  tenant_id: string;
  created_at: number;
  updated_at: number;
}

export interface GatewayConfigRequest {
  name: string;
  provider: string;
  api_key: string;
  base_url?: string;
  models?: string;         // JSON string
  rate_limit?: number;
  timeout?: number;
  retry_count?: number;
  priority?: number;
  enabled?: boolean;
  description?: string;
  tenant_id?: string;
}

export interface GatewayChatRequest {
  provider?: string;
  model: string;
  messages: Array<{
    role: string;
    content: string;
  }>;
  temperature?: number;
  max_tokens?: number;
  parameters?: Record<string, string>;
  tenant_id?: string;
}

export interface GatewayChatResponse {
  content: string;
  model: string;
  provider: string;
  total_tokens: number;
  cost: number;
  latency: number;
  usedFallback: boolean;
  originalModel: string;
  error: string;
}

export interface GatewayStats {
  provider: string;
  total_requests: number;
  success_count: number;
  error_count: number;
  avg_latency: number;
  total_tokens: number;
  total_cost: number;
  lastActiveTime: number;
}

export interface GatewayRoute {
  id: string;
  name: string;
  pattern: string;
  model_id: string;
  fallbacks: string;       // JSON string
  tenant_id: string;
  enabled: boolean;
  created_at: number;
  updated_at: number;
}

export interface RoutingRuleRequest {
  name: string;
  pattern: string;
  model_id: string;
  fallbacks?: string;      // JSON string
  enabled?: boolean;
  tenant_id?: string;
}

// ========================= API =========================

export const gatewayApi = {
  // Chat through gateway
  chat: (params: GatewayChatRequest): Promise<GatewayChatResponse> =>
    client.post('/api/v2/harness/gateway/chat', params),

  // Provider config CRUD - matches backend routes
  createGatewayConfig: (params: GatewayConfigRequest): Promise<GatewayProvider> =>
    client.post('/api/v2/harness/gateway/config', params),

  listGatewayConfigs: (): Promise<{ configs: GatewayProvider[] }> =>
    client.get('/api/v2/harness/gateway/config'),

  getGatewayConfig: (id: string): Promise<GatewayProvider> =>
    client.get(`/api/v2/harness/gateway/config/${id}`),

  updateGatewayConfig: (id: string, params: Partial<GatewayConfigRequest>): Promise<GatewayProvider> =>
    client.put(`/api/v2/harness/gateway/config/${id}`, params),

  deleteGatewayConfig: (id: string): Promise<void> =>
    client.delete(`/api/v2/harness/gateway/config/${id}`),

  toggleProvider: (id: string, enabled: boolean, fullConfig: GatewayConfigRequest): Promise<void> =>
    client.put(`/api/v2/harness/gateway/config/${id}`, { ...fullConfig, enabled }),

  // Gateway stats
  getGatewayStats: (): Promise<{ stats: GatewayStats[] }> =>
    client.get('/api/v2/harness/gateway/stats'),

  // Routing rules - uses dedicated route endpoint
  listRoutingRules: (): Promise<{ routes: GatewayRoute[] }> =>
    client.get('/api/v2/harness/gateway/routes'),

  createRoutingRule: (params: RoutingRuleRequest): Promise<GatewayRoute> =>
    client.post('/api/v2/harness/gateway/route', params),

  deleteRoutingRule: (id: string): Promise<void> =>
    client.delete(`/api/v2/harness/gateway/route/${id}`),

  // Load balance strategy
  setLoadBalanceStrategy: (strategy: string): Promise<void> =>
    client.post('/api/v2/harness/gateway/strategy', { strategy }),
};
