import client from './client';
import type { MCPConnection } from '../types';

// Connect to an external MCP server
export function connectMCP(params: {
  name: string;
  type: 'stdio' | 'streamable-http';
  command?: string;
  url?: string;
  env?: Record<string, string>;
}): Promise<{ connection: MCPConnection }> {
  return client.post('/api/v2/mcp/connect', params);
}

// Disconnect from an MCP server
export function disconnectMCP(id: string): Promise<void> {
  return client.delete(`/api/v2/mcp/connections/${id}`);
}

// List all MCP connections
export function listMCPConnections(): Promise<{ connections: MCPConnection[] }> {
  return client.get('/api/v2/mcp/connections');
}

// List all MCP tools (built-in + remote)
export function listMCPTools(connectionId?: string): Promise<{ tools: MCPToolItem[] }> {
  const params = connectionId ? { connection_id: connectionId } : {};
  return client.get('/api/v2/mcp/tools', { params });
}

// Call an MCP tool
export function callMCPTool(params: {
  name: string;
  arguments: Record<string, unknown>;
  tool_config?: Record<string, unknown>;
}): Promise<{ is_error: boolean; content: string }> {
  return client.post('/api/v2/mcp/call', params);
}

export interface MCPToolItem {
  name: string;
  description: string;
  input_schema: string;
}
