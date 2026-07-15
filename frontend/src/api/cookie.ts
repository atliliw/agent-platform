import client from './client';
import type { ApiResponse } from '../types';

export interface Cookie {
  name: string;
  value: string;
  domain?: string;
  path?: string;
  expires?: number;
  httpOnly?: boolean;
  secure?: boolean;
}

export interface StoredCookie extends Cookie {
  id?: string;
  user_id?: string;
  tenant_id?: string;
  created_at?: string;
  updated_at?: string;
}

export const cookieApi = {
  // 保存 Cookie
  save: (params: {
    user_id?: string;
    tenant_id?: string;
    domain: string;
    cookies: Cookie[];
  }): Promise<ApiResponse<{ count: number; domain: string; user_id: string; tenant_id: string }>> =>
    client.post('/api/v2/cookies', params),

  // 获取指定域名的 Cookie
  get: (params: {
    domain: string;
    user_id?: string;
    tenant_id?: string;
  }): Promise<ApiResponse<{ cookies: Cookie[]; domain: string; user_id: string; tenant_id: string }>> =>
    client.get('/api/v2/cookies', { params }),

  // 获取所有 Cookie
  getAll: (params?: {
    user_id?: string;
    tenant_id?: string;
  }): Promise<ApiResponse<{ cookies: StoredCookie[]; user_id: string; tenant_id: string }>> =>
    client.get('/api/v2/cookies/all', { params }),

  // 删除 Cookie
  delete: (params: {
    domain: string;
    user_id?: string;
    tenant_id?: string;
  }): Promise<ApiResponse<null>> =>
    client.delete('/api/v2/cookies', { params }),
};