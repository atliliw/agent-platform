// 记忆管理 API 类型定义

// 记忆类型
export type MemoryType = 'working' | 'episodic' | 'semantic' | 'procedural';

// 工作记忆
export interface WorkingMemory {
  session_id: string;
  messages: MemoryMessage[];
  compressed_context?: string;
  token_count: number;
  max_tokens: number;
  created_at: string;
  updated_at: string;
}

export interface MemoryMessage {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  importance: number;
  timestamp: string;
  compressed?: boolean;
}

// 情节记忆
export interface EpisodicMemory {
  id: string;
  session_id: string;
  agent_id?: string;
  event_type: string;
  title: string;
  description: string;
  participants: string[];
  actions: EventAction[];
  outcome: string;
  importance: number;
  emotion?: string;
  started_at: string;
  ended_at?: string;
  created_at: string;
}

export interface EventAction {
  sequence: number;
  actor: string;
  action: string;
  target?: string;
  result: string;
  timestamp: string;
}

// 语义记忆
export interface SemanticMemory {
  id: string;
  concept: string;
  category: string;
  description: string;
  attributes: Record<string, unknown>;
  relations: ConceptRelation[];
  source_count: number;
  confidence: number;
  created_at: string;
  updated_at: string;
  last_accessed: string;
  access_count: number;
}

export interface ConceptRelation {
  relation_type: string;
  target_concept: string;
  strength: number;
}

// 程序记忆（技能）
export interface ProceduralMemory {
  id: string;
  name: string;
  description: string;
  category: string;
  steps: SkillStep[];
  preconditions: string[];
  postconditions: string[];
  success_rate: number;
  usage_count: number;
  last_used: string;
  created_at: string;
  updated_at: string;
}

export interface SkillStep {
  sequence: number;
  action: string;
  parameters: Record<string, unknown>;
  expected_outcome: string;
  error_handling?: string;
}

// 时间线事件
export interface TimelineEvent {
  id: string;
  timestamp: string;
  event_type: string;
  title: string;
  description: string;
  importance: number;
  related_memories: string[];
}

// 时间线查询参数
export interface TimelineQueryParams {
  start_time?: string;
  end_time?: string;
  event_type?: string;
  agent_id?: string;
  limit?: number;
}

// 知识图谱节点
export interface KnowledgeNode {
  id: string;
  type: 'concept' | 'entity' | 'event';
  name: string;
  properties: Record<string, unknown>;
}

// 知识图谱边
export interface KnowledgeEdge {
  id: string;
  source_id: string;
  target_id: string;
  relation: string;
  properties: Record<string, unknown>;
}

// 知识图谱
export interface KnowledgeGraph {
  nodes: KnowledgeNode[];
  edges: KnowledgeEdge[];
}

// 记忆整合请求
export interface ConsolidateRequest {
  session_id: string;
  working_memory_only?: boolean;
  force?: boolean;
}

// 记忆整合结果
export interface ConsolidateResult {
  processed_count: number;
  created_memories: string[];
  updated_memories: string[];
  deleted_memories: string[];
  duration_ms: number;
}

// 记忆搜索请求
export interface MemorySearchRequest {
  query: string;
  types?: MemoryType[];
  limit?: number;
  min_importance?: number;
  time_range?: {
    start: string;
    end: string;
  };
}

// 记忆搜索结果
export interface MemorySearchResult {
  id: string;
  type: MemoryType;
  content: string;
  relevance_score: number;
  importance: number;
  created_at: string;
  metadata?: Record<string, unknown>;
}

// 遗忘曲线配置
export interface ForgettingConfig {
  enabled: boolean;
  retention_days: number;
  importance_threshold: number;
  cleanup_interval_hours: number;
}

// 记忆统计
export interface MemoryStats {
  working_memory_count: number;
  episodic_memory_count: number;
  semantic_memory_count: number;
  procedural_memory_count: number;
  total_size_bytes: number;
  avg_importance: number;
  oldest_memory: string;
  newest_memory: string;
}
