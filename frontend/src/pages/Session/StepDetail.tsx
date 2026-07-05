/**
 * StepDetail.tsx — 回放步骤详情抽屉组件
 *
 * 按步骤类型（think/tool_call/observation/decision/action/llm_call）
 * 渲染不同风格的详情内容
 */

import {
  Descriptions, Badge, Tag, Drawer, Collapse, Typography,
} from 'antd';
import {
  BulbOutlined, ToolOutlined, EyeOutlined, CheckCircleOutlined,
  UserOutlined, RobotOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { ToolCallDisplay, AgentTraceDisplay } from '../../components/Chat/ChatMessage';
import type { SessionStep } from '../../api/session';
import { STEP_TYPE_LABELS, STEP_TYPE_COLORS, STEP_TYPE_ICONS } from './replayTransform';

const { Panel } = Collapse;
const { Text } = Typography;

interface StepDetailProps {
  step: SessionStep;
  open: boolean;
  onClose: () => void;
}

/** 步骤类型 → 状态颜色 */
function getStatusColor(status?: string): 'success' | 'processing' | 'error' | 'warning' | 'default' {
  const map: Record<string, 'success' | 'processing' | 'error' | 'warning' | 'default'> = {
    pending: 'default',
    running: 'processing',
    completed: 'success',
    failed: 'error',
  };
  return map[status || ''] || 'default';
}

/** 步骤类型 → 图标 */
function getStepIcon(stepType?: string): React.ReactNode {
  const map: Record<string, React.ReactNode> = {
    think: <BulbOutlined style={{ color: '#722ed1' }} />,
    tool_call: <ToolOutlined style={{ color: '#1677ff' }} />,
    observation: <EyeOutlined style={{ color: '#52c41a' }} />,
    decision: <CheckCircleOutlined style={{ color: '#fa8c16' }} />,
    action: <UserOutlined style={{ color: '#1890ff' }} />,
    llm_call: <RobotOutlined style={{ color: '#13c2c2' }} />,
  };
  return map[stepType || ''] || null;
}

export default function StepDetail({ step, open, onClose }: StepDetailProps) {
  const stepType = step.step_type || 'unknown';
  const stepLabel = STEP_TYPE_LABELS[stepType] || stepType;
  const stepColor = STEP_TYPE_COLORS[stepType] || '#999';
  const stepIcon = STEP_TYPE_ICONS[stepType] || '📋';

  const renderContent = () => {
    switch (stepType) {
      case 'think':
        return renderThinkStep();
      case 'tool_call':
        return renderToolCallStep();
      case 'observation':
        return renderObservationStep();
      case 'decision':
        return renderDecisionStep();
      case 'action':
        return renderActionStep();
      case 'llm_call':
        return renderLLMCallStep();
      default:
        return renderGenericStep();
    }
  };

  /** think 步骤：展示思考过程 */
  const renderThinkStep = () => (
    <div style={{ padding: 16, background: '#f9f0ff', borderRadius: 8, marginBottom: 16 }}>
      <div style={{ marginBottom: 8, color: '#722ed1', fontWeight: 600 }}>
        💡 思考过程
      </div>
      <div style={{ fontSize: 14, lineHeight: 1.8, whiteSpace: 'pre-wrap' }}>
        {step.input || '无思考内容'}
      </div>
    </div>
  );

  /** tool_call 步骤：复用 ToolCallDisplay */
  const renderToolCallStep = () => {
    let metadata: Record<string, unknown> = {};
    try {
      metadata = step.metadata ? JSON.parse(step.metadata) : {};
    } catch { /* ignore */ }

    const toolCall = {
      id: (metadata.toolCallId as string) || step.id,
      name: step.input || 'unknown',
      arguments: (typeof metadata.arguments === 'string' || typeof metadata.arguments === 'object' ? metadata.arguments : {}) as string | Record<string, unknown>,
      result: step.output || '',
      status: step.status === 'completed' ? 'completed' as const : (step.status as 'pending' | 'running' | 'completed' | 'error'),
    };

    return (
      <div style={{ marginBottom: 16 }}>
        <ToolCallDisplay toolCall={toolCall} />
      </div>
    );
  };

  /** observation 步骤：展示观察结果 */
  const renderObservationStep = () => {
    const isError = step.output?.toLowerCase().includes('error') ||
                    step.output?.toLowerCase().includes('失败');
    return (
      <div style={{
        padding: 16,
        background: isError ? '#fff2f0' : '#f6ffed',
        borderRadius: 8,
        marginBottom: 16,
        border: `1px solid ${isError ? '#ffccc7' : '#b7eb8f'}`,
      }}>
        <div style={{
          marginBottom: 8,
          color: isError ? '#ff4d4f' : '#52c41a',
          fontWeight: 600,
        }}>
          👁 观察结果
        </div>
        <pre style={{
          fontSize: 12,
          overflow: 'auto',
          maxHeight: 300,
          margin: 0,
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-word',
        }}>
          {formatOutput(step.output)}
        </pre>
      </div>
    );
  };

  /** decision 步骤：展示最终决策（Markdown 渲染） */
  const renderDecisionStep = () => (
    <div style={{ padding: 16, background: '#fff7e6', borderRadius: 8, marginBottom: 16, border: '1px solid #ffd591' }}>
      <div style={{ marginBottom: 8, color: '#fa8c16', fontWeight: 600 }}>
        ✅ 最终决策
      </div>
      <div className="markdown-body" style={{ fontSize: 14 }}>
        <ReactMarkdown remarkPlugins={[remarkGfm]}>
          {step.output || '无输出'}
        </ReactMarkdown>
      </div>
    </div>
  );

  /** action 步骤：展示用户输入 */
  const renderActionStep = () => (
    <div style={{ padding: 16, background: '#e6f7ff', borderRadius: 8, marginBottom: 16 }}>
      <div style={{ marginBottom: 8, color: '#1890ff', fontWeight: 600 }}>
        👤 用户输入
      </div>
      <div style={{ fontSize: 14, lineHeight: 1.8, whiteSpace: 'pre-wrap' }}>
        {step.input || '无输入'}
      </div>
    </div>
  );

  /** llm_call 步骤：展示 LLM 调用的输入输出 */
  const renderLLMCallStep = () => (
    <div>
      {step.input && (
        <Collapse style={{ marginBottom: 8 }}>
          <Panel header="输入" key="input">
            <pre style={{ fontSize: 12, overflow: 'auto', maxHeight: 200, margin: 0, whiteSpace: 'pre-wrap' }}>
              {formatOutput(step.input)}
            </pre>
          </Panel>
        </Collapse>
      )}
      {step.output && (
        <Collapse defaultActiveKey={['output']} style={{ marginBottom: 8 }}>
          <Panel header="输出" key="output">
            <div className="markdown-body" style={{ fontSize: 13 }}>
              <ReactMarkdown remarkPlugins={[remarkGfm]}>
                {step.output}
              </ReactMarkdown>
            </div>
          </Panel>
        </Collapse>
      )}
    </div>
  );

  /** 通用步骤渲染 */
  const renderGenericStep = () => (
    <Collapse>
      {step.input && (
        <Panel header="输入" key="input">
          <pre style={{ fontSize: 12, overflow: 'auto', margin: 0, whiteSpace: 'pre-wrap' }}>
            {formatOutput(step.input)}
          </pre>
        </Panel>
      )}
      {step.output && (
        <Panel header="输出" key="output">
          <pre style={{ fontSize: 12, overflow: 'auto', margin: 0, whiteSpace: 'pre-wrap' }}>
            {formatOutput(step.output)}
          </pre>
        </Panel>
      )}
    </Collapse>
  );

  return (
    <Drawer
      title={
        <span>
          {stepIcon} 步骤详情 — <Tag color={stepColor}>{stepLabel}</Tag>
        </span>
      }
      placement="right"
      width={520}
      open={open}
      onClose={onClose}
    >
      <Descriptions bordered column={1} size="small" style={{ marginBottom: 16 }}>
        <Descriptions.Item label="步骤编号">{step.step_number}</Descriptions.Item>
        <Descriptions.Item label="步骤类型">
          <Tag color={stepColor} icon={getStepIcon(stepType)}>{stepLabel}</Tag>
        </Descriptions.Item>
        <Descriptions.Item label="状态">
          <Badge status={getStatusColor(step.status)} text={step.status || 'completed'} />
        </Descriptions.Item>
        <Descriptions.Item label="时间">
          {step.timestamp ? dayjs(step.timestamp * 1000).format('YYYY-MM-DD HH:mm:ss') : '-'}
        </Descriptions.Item>
        {step.parent_step_id && (
          <Descriptions.Item label="父步骤">
            <Text type="secondary">{step.parent_step_id}</Text>
          </Descriptions.Item>
        )}
      </Descriptions>

      {renderContent()}
    </Drawer>
  );
}

/** 格式化输出文本 */
function formatOutput(text: string | undefined): string {
  if (!text) return '无内容';
  try {
    const parsed = JSON.parse(text);
    return JSON.stringify(parsed, null, 2);
  } catch {
    return text;
  }
}
