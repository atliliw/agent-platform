// RAG Metrics API
import client from './client';

export interface RAGMetrics {
  id: string;
  query_id: string;
  query: string;
  retrieved_docs: string[];
  generated_answer: string;
  ground_truth: string;
  context_precision: number;
  context_recall: number;
  context_relevancy: number;
  context_entity_recall: number;
  noise_sensitivity: number;
  mrr: number;
  ndcg: number;
  faithfulness: number;
  answer_relevancy: number;
  answer_correctness: number;
  answer_similarity: number;
  hallucination: number;
  comprehensiveness: number;
  coherence: number;
  ragas_score: number;
  timestamp: number;
  tenant_id: string;
}

export interface RAGQuery {
  query_id: string;
  query: string;
  contexts: string[];
  generated_answer: string;
  ground_truth: string;
}

export interface RAGEvaluation {
  id: string;
  name: string;
  description: string;
  queries: RAGQuery[];
  status: string;
  start_time: number;
  end_time: number;
  tenant_id: string;
  created_at: number;
}

export const ragApi = {
  evaluate: (data: { query: string; contexts: string[]; answer: string; ground_truth: string }) =>
    client.post('/api/v2/harness/rag/evaluate', data),

  batchEvaluate: (data: { requests: Array<{ query: string; contexts: string[]; answer: string; ground_truth: string }>; tenant_id?: string }) =>
    client.post('/api/v2/harness/rag/batch-evaluate', data),

  listMetrics: (params?: { limit?: number }) =>
    client.get('/api/v2/harness/rag/metrics', { params }),

  getMetrics: (id: string) =>
    client.get(`/api/v2/harness/rag/metrics/${id}`),

  createEvaluation: (data: { name: string; description: string; queries: unknown[] }) =>
    client.post('/api/v2/harness/rag/evaluation', data),

  listEvaluations: (params?: { status?: string }) =>
    client.get('/api/v2/harness/rag/evaluations', { params }),

  runEvaluation: (id: string) =>
    client.post(`/api/v2/harness/rag/evaluation/${id}/run`),
};
