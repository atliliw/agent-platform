/**
 * replayTransform.ts — 将 chat-service 的 messages 分解为细粒度的回放步骤
 *
 * 核心思想：assistant 消息可能包含 agent_trace（思考-行动-观察循环）
 * 和 tool_calls，需要将它们展开为独立的步骤以支持逐步回放。
 */

import type { SessionStep, ExecutionGraph, GraphNode, GraphEdge } from '../../api/session';

/** chat-service 返回的原始消息格式 */
interface RawMessage {
  id?: string;
  role: string;
  content: string;
  tool_calls?: Array<{
    id: string;
    name: string;
    arguments: string;
    result?: string;
    status: string;
  }>;
  agent_trace?: Array<{
    thought: string;
    action: string;
    arguments: string;
    result: string;
    step: number;
  }>;
  timestamp?: number;
}

/** 步骤类型颜色映射 */
export const STEP_TYPE_COLORS: Record<string, string> = {
  think: '#722ed1',
  tool_call: '#1677ff',
  observation: '#52c41a',
  decision: '#fa8c16',
  action: '#1890ff',
  llm_call: '#13c2c2',
};

/** 步骤类型标签 */
export const STEP_TYPE_LABELS: Record<string, string> = {
  think: '思考',
  tool_call: '工具调用',
  observation: '观察结果',
  decision: '最终决策',
  action: '用户输入',
  llm_call: 'LLM 调用',
};

/** 步骤类型图标名称 */
export const STEP_TYPE_ICONS: Record<string, string> = {
  think: '💡',
  tool_call: '🔧',
  observation: '👁',
  decision: '✅',
  action: '👤',
  llm_call: '🤖',
};

/**
 * 将 chat-service 的消息数组分解为细粒度回放步骤
 */
export function decomposeMessagesToSteps(
  messages: RawMessage[],
  sessionId: string,
  sessionCreatedAt?: number
): SessionStep[] {
  const steps: SessionStep[] = [];
  let stepNumber = 0;

  for (const msg of messages) {
    if (msg.role === 'user') {
      stepNumber++;
      steps.push({
        id: msg.id || `step-${stepNumber}`,
        session_id: sessionId,
        step_number: stepNumber,
        step_type: 'action',
        input: msg.content,
        output: '',
        status: 'completed',
        duration: 0,
        timestamp: msg.timestamp || sessionCreatedAt,
      });
    } else if (msg.role === 'assistant') {
      const hasAgentTrace = msg.agent_trace && msg.agent_trace.length > 0;
      const hasToolCalls = msg.tool_calls && msg.tool_calls.length > 0;
      const msgId = msg.id || `msg-${stepNumber + 1}`;

      if (hasAgentTrace) {
        // 有 agent_trace：分解为 think → tool_call → observation 循环
        const parentStepId = msgId;

        // 先创建一个 LLM 推理步骤作为父步骤
        stepNumber++;
        steps.push({
          id: parentStepId,
          session_id: sessionId,
          step_number: stepNumber,
          step_type: 'llm_call',
          input: '',
          output: msg.content,
          status: 'completed',
          duration: 0,
          timestamp: msg.timestamp || sessionCreatedAt,
        });

        for (const state of msg.agent_trace!) {
          // think 步骤
          if (state.thought) {
            stepNumber++;
            steps.push({
              id: `${parentStepId}-think-${state.step}`,
              session_id: sessionId,
              step_number: stepNumber,
              step_type: 'think',
              parent_step_id: parentStepId,
              input: state.thought,
              output: '',
              metadata: JSON.stringify({ step: state.step }),
              status: 'completed',
              duration: 0,
              timestamp: msg.timestamp || sessionCreatedAt,
            });
          }

          // tool_call 步骤
          if (state.action) {
            stepNumber++;
            steps.push({
              id: `${parentStepId}-action-${state.step}`,
              session_id: sessionId,
              step_number: stepNumber,
              step_type: 'tool_call',
              parent_step_id: parentStepId,
              input: state.action,
              output: state.result || '',
              metadata: JSON.stringify({
                arguments: safeParseJSON(state.arguments),
                step: state.step,
              }),
              status: 'completed',
              duration: 0,
              timestamp: msg.timestamp || sessionCreatedAt,
            });
          }

          // observation 步骤
          if (state.result) {
            stepNumber++;
            steps.push({
              id: `${parentStepId}-result-${state.step}`,
              session_id: sessionId,
              step_number: stepNumber,
              step_type: 'observation',
              parent_step_id: parentStepId,
              input: '',
              output: state.result,
              metadata: JSON.stringify({ step: state.step }),
              status: 'completed',
              duration: 0,
              timestamp: msg.timestamp || sessionCreatedAt,
            });
          }
        }

        // decision 步骤（最终输出）
        if (msg.content) {
          stepNumber++;
          steps.push({
            id: `${parentStepId}-decision`,
            session_id: sessionId,
            step_number: stepNumber,
            step_type: 'decision',
            parent_step_id: parentStepId,
            input: '',
            output: msg.content,
            status: 'completed',
            duration: 0,
            timestamp: msg.timestamp || sessionCreatedAt,
          });
        }
      } else if (hasToolCalls) {
        // 有 tool_calls 但无 agent_trace（single-agent 路径）
        const parentStepId = msgId;

        stepNumber++;
        steps.push({
          id: parentStepId,
          session_id: sessionId,
          step_number: stepNumber,
          step_type: 'llm_call',
          input: '',
          output: msg.content,
          status: 'completed',
          duration: 0,
          timestamp: msg.timestamp || sessionCreatedAt,
        });

        for (const tc of msg.tool_calls!) {
          stepNumber++;
          steps.push({
            id: tc.id || `tc-${stepNumber}`,
            session_id: sessionId,
            step_number: stepNumber,
            step_type: 'tool_call',
            parent_step_id: parentStepId,
            input: tc.name,
            output: tc.result || '',
            metadata: JSON.stringify({
              arguments: safeParseJSON(tc.arguments),
              status: tc.status,
              toolCallId: tc.id,
            }),
            status: tc.status || 'completed',
            duration: 0,
            timestamp: msg.timestamp || sessionCreatedAt,
          });

          if (tc.result) {
            stepNumber++;
            steps.push({
              id: `${tc.id || `tc-${stepNumber}`}-result`,
              session_id: sessionId,
              step_number: stepNumber,
              step_type: 'observation',
              parent_step_id: parentStepId,
              input: '',
              output: tc.result,
              status: 'completed',
              duration: 0,
              timestamp: msg.timestamp || sessionCreatedAt,
            });
          }
        }
      } else {
        // 旧数据兼容：无 agent_trace 也无 tool_calls
        stepNumber++;
        steps.push({
          id: msgId,
          session_id: sessionId,
          step_number: stepNumber,
          step_type: 'llm_call',
          input: '',
          output: msg.content,
          status: 'completed',
          duration: 0,
          timestamp: msg.timestamp || sessionCreatedAt,
        });
      }
    }
  }

  return steps;
}

/**
 * 从步骤构建执行图（支持分支）
 */
export function buildExecutionGraph(steps: SessionStep[]): ExecutionGraph {
  const nodes: GraphNode[] = [];
  const edges: GraphEdge[] = [];

  // Start 节点
  nodes.push({ id: 'start', type: 'start', label: '开始', status: 'completed' });

  // 步骤节点
  for (const step of steps) {
    const label = buildStepLabel(step);
    nodes.push({
      id: step.id,
      type: step.step_type,
      label,
      status: step.status,
      metadata: step.metadata ? safeParseToRecord(step.metadata) : undefined,
    });
  }

  // End 节点
  nodes.push({ id: 'end', type: 'end', label: '结束', status: 'completed' });

  // 构建边
  if (steps.length === 0) {
    edges.push({ from: 'start', to: 'end' });
    return { nodes, edges };
  }

  // start → 第一个步骤
  edges.push({ from: 'start', to: steps[0].id });

  // 找出所有父步骤 ID 及其子步骤
  const parentChildMap = new Map<string, SessionStep[]>();
  const topLevelSteps: SessionStep[] = [];

  for (const step of steps) {
    if (step.parent_step_id) {
      const children = parentChildMap.get(step.parent_step_id) || [];
      children.push(step);
      parentChildMap.set(step.parent_step_id, children);
    } else {
      topLevelSteps.push(step);
    }
  }

  // 顶级步骤之间的线性连接
  for (let i = 0; i < topLevelSteps.length - 1; i++) {
    edges.push({ from: topLevelSteps[i].id, to: topLevelSteps[i + 1].id });
  }

  // 父步骤到子步骤的分支连接
  for (const [parentId, children] of parentChildMap) {
    // 父 → 第一个子步骤
    if (children.length > 0) {
      edges.push({ from: parentId, to: children[0].id, label: '展开' });

      // 子步骤之间的线性连接
      for (let i = 0; i < children.length - 1; i++) {
        edges.push({ from: children[i].id, to: children[i + 1].id });
      }
    }
  }

  // 最后一个步骤 → end
  const lastStep = steps[steps.length - 1];
  edges.push({ from: lastStep.id, to: 'end' });

  return { nodes, edges };
}

/** 构建步骤的显示标签 */
function buildStepLabel(step: SessionStep): string {
  switch (step.step_type) {
    case 'think':
      return truncateText(step.input, 20) || '思考';
    case 'tool_call':
      return `🔧 ${step.input || '工具调用'}`;
    case 'observation':
      return truncateText(step.output, 20) || '观察';
    case 'decision':
      return '✅ 决策';
    case 'action':
      return `👤 ${truncateText(step.input, 16) || '用户'}`;
    case 'llm_call':
      return '🤖 LLM';
    default:
      return step.step_type || '步骤';
  }
}

/** 截断文本 */
function truncateText(text: string | undefined, maxLen: number): string {
  if (!text) return '';
  const clean = text.replace(/\n/g, ' ').trim();
  return clean.length > maxLen ? clean.slice(0, maxLen) + '...' : clean;
}

/** 安全解析 JSON */
function safeParseJSON(jsonStr: string | undefined): unknown {
  if (!jsonStr) return {};
  try {
    return JSON.parse(jsonStr);
  } catch {
    return jsonStr;
  }
}

/** 安全解析 JSON 为 Record */
function safeParseToRecord(jsonStr: string | undefined): Record<string, string> {
  if (!jsonStr) return {};
  try {
    const parsed = JSON.parse(jsonStr);
    if (typeof parsed === 'object' && parsed !== null) {
      const result: Record<string, string> = {};
      for (const [k, v] of Object.entries(parsed)) {
        result[k] = typeof v === 'string' ? v : JSON.stringify(v);
      }
      return result;
    }
    return {};
  } catch {
    return {};
  }
}
