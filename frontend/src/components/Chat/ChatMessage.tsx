import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Card, Tag, Collapse } from 'antd';
import { UserOutlined, RobotOutlined, ToolOutlined, BulbOutlined, CheckCircleOutlined, LoadingOutlined } from '@ant-design/icons';
import type { Message, ToolCall, AgentState } from '../../types';
import './ChatMessage.css';

interface ChatMessageProps {
  message: Message;
}

export function ToolCallDisplay({ toolCall }: { toolCall: ToolCall }) {
  const statusColor: Record<string, 'default' | 'processing' | 'success' | 'error'> = {
    pending: 'default',
    running: 'processing',
    completed: 'success',
    error: 'error',
  };

  const statusIcon: Record<string, React.ReactNode> = {
    pending: <LoadingOutlined />,
    running: <LoadingOutlined spin />,
    completed: <CheckCircleOutlined />,
    error: <ToolOutlined />,
  };

  return (
    <Card size="small" className="tool-call-card">
      <div className="tool-call-header">
        <ToolOutlined />
        <span className="tool-name">{toolCall.name}</span>
        <Tag color={statusColor[toolCall.status]} icon={statusIcon[toolCall.status]}>
          {toolCall.status}
        </Tag>
      </div>
      <Collapse
        ghost
        items={[
          {
            key: '1',
            label: '参数',
            children: (
              <pre className="tool-args">
                {typeof toolCall.arguments === 'string'
                  ? toolCall.arguments
                  : JSON.stringify(toolCall.arguments, null, 2)}
              </pre>
            ),
          },
          ...(toolCall.result ? [{
            key: '2',
            label: '结果',
            children: <div className="tool-result">{toolCall.result}</div>,
          }] : []),
        ]}
      />
    </Card>
  );
}

export function AgentTraceDisplay({ states }: { states: AgentState[] }) {
  if (!states || states.length === 0) return null;

  return (
    <div className="agent-trace">
      <div style={{ marginBottom: 12, fontWeight: 500, color: '#1890ff' }}>
        🤖 Agent 执行轨迹
      </div>
      {states.map((state, index) => (
        <div key={index} className="agent-step">
          <div className="agent-step-header">
            <span className="agent-step-number">{state.step || index + 1}</span>
            <span>步骤 {state.step || index + 1}</span>
          </div>

          {state.thought && (
            <div className="agent-thought">
              {state.thought}
            </div>
          )}

          {state.action && (
            <div className="agent-action">
              <ToolOutlined className="agent-action-icon" />
              <span className="agent-action-name">{state.action}</span>
              {state.arguments && Object.keys(state.arguments).length > 0 && (
                <Tag>{JSON.stringify(state.arguments)}</Tag>
              )}
            </div>
          )}

          {state.result && (
            <div className={`agent-result ${state.result.includes('Error') ? 'error' : ''}`}>
              <div className="agent-result-header">
                <BulbOutlined />
                <span>执行结果</span>
              </div>
              <div style={{ marginTop: 4 }}>{state.result}</div>
            </div>
          )}
        </div>
      ))}
    </div>
  );
}

export default function ChatMessage({ message }: ChatMessageProps) {
  const isUser = message.role === 'user';

  return (
    <div className={`chat-message ${isUser ? 'message-user-wrapper' : 'message-assistant-wrapper'}`}>
      <div className="message-avatar">
        {isUser ? <UserOutlined /> : <RobotOutlined />}
      </div>
      <div className={`message-bubble ${isUser ? 'message-user' : 'message-assistant'}`}>
        {isUser ? (
          <div className="message-text">{message.content}</div>
        ) : (
          <div className="markdown-body">
            <ReactMarkdown remarkPlugins={[remarkGfm]}>
              {message.content}
            </ReactMarkdown>
          </div>
        )}

        {/* Agent 执行轨迹 */}
        {message.agent_trace && message.agent_trace.length > 0 && (
          <AgentTraceDisplay states={message.agent_trace} />
        )}

        {/* 工具调用 */}
        {message.tool_calls && message.tool_calls.length > 0 && (
          <div className="tool-calls">
            {message.tool_calls.map((tc) => (
              <ToolCallDisplay key={tc.id} toolCall={tc} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}