import client from './client';
import type { ApiResponse, Document, SearchResult, UploadConfig, SearchRequest, PaginationParams } from '../types';

export const knowledgeApi = {
  // 上传文件
  uploadFile: (
    file: File,
    config: UploadConfig,
    onProgress?: (percent: number) => void
  ): Promise<ApiResponse<{ document_id: string; filename: string; chunk_count: number; status: string }>> => {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('chunk_strategy', config.chunk_strategy);
    formData.append('chunk_size', config.chunk_size.toString());
    formData.append('chunk_overlap', config.chunk_overlap.toString());

    return client.post('/api/v2/knowledge/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      onUploadProgress: (e) => {
        if (e.total && onProgress) {
          onProgress(Math.round((e.loaded * 100) / e.total));
        }
      },
    });
  },

  // 文档列表
  listDocuments: (params?: PaginationParams & { status?: string }): Promise<ApiResponse<{ documents: Document[]; pagination: { total: number; page: number; page_size: number } }>> =>
    client.get('/api/v2/knowledge/documents', { params }),

  // 获取文档
  getDocument: (id: string): Promise<ApiResponse<Document>> =>
    client.get(`/api/v2/knowledge/documents/${id}`),

  // 删除文档
  deleteDocument: (id: string): Promise<ApiResponse<null>> =>
    client.delete(`/api/v2/knowledge/documents/${id}`),

  // 检索
  search: (params: SearchRequest): Promise<ApiResponse<{ results: SearchResult[]; total: number }>> =>
    client.post('/api/v2/knowledge/search', params),
};