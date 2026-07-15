import { useEffect, useRef, useState } from 'react';
import { Row, Col, Card, Select, Input, Button, Tag, Collapse, Space } from 'antd';
import { SettingOutlined, CloseOutlined, StopOutlined, LoadingOutlined, PauseCircleOutlined, PlayCircleOutlined, MessageOutlined } from '@ant-design/icons';
import { ChatMessage, ChatInput, ChatHistory } from '../../components/Chat';
import { EmptyState } from '../../components/Common';
import { useChatStore } from '../../stores';
import { promptApi } from '../../api/prompt';
import './ChatPage.css';

const categoryLabels: Record<string, string> = {
  system: '系统指令',
  agent: 'Agent',
  rag: 'RAG',
  template: '模板',
  user: '用户',
  other: '其他',
};

export default function ChatPage() {
  const {
    sessions,
    currentSessionId,
    messages,
    isLoading,
    isStreaming,
    isPaused,
    isRunning,
    loadSessions,
    selectSession,
    deleteSession,
    createSession,
    sendMessage,
    stopStreaming,
    pauseAgent,
    resumeAgent,
    stopAgent,
    injectMessage,
    systemPrompt,
    showSystemPrompt,
    setSystemPrompt,
    setShowSystemPrompt,
  } = useChatStore();

  const [templates, setTemplates] = useState<any[]>([]);
  const [injectVisible, setInjectVisible] = useState(false);
  const [injectValue, setInjectValue] = useState('');

  const messagesEndRef = useRef<HTMLDivElement>(null);

  // 加载会话列表
  useEffect(() => {
    loadSessions();
  }, []);

  // 加载模板列表
  useEffect(() => {
    promptApi.listPrompts()
      .then((res: any) => setTemplates(res?.prompts || []))
      .catch(() => setTemplates([]));
  }, []);

  // 滚动到底部
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  // 选择模板后加载内容
  const handleTemplateSelect = async (key: string) => {
    if (!key) return;
    try {
      const res: any = await promptApi.getActiveVersion(key);
      if (res?.content) {
        setSystemPrompt(res.content);
      }
    } catch { /* ignore */ }
  };

  // 按分类分组模板
  const grouped: Record<string, any[]> = templates.reduce((acc, t: any) => {
    const cat = t.category || 'other';
    if (!acc[cat]) acc[cat] = [];
    acc[cat].push(t);
    return acc;
  }, {} as Record<string, any[]>);

  return (
    <div className="chat-page">
      <Row gutter={16} style={{ height: '100%' }}>
        <Col span={6}>
          <Card className="chat-history-card" title="会话列表">
            <ChatHistory
              sessions={sessions}
              currentSessionId={currentSessionId}
              onSelect={selectSession}
              onDelete={deleteSession}
              onNew={createSession}
            />
          </Card>
        </Col>

        <Col span={18}>
          <Card className="chat-main-card">
            {/* 系统提示词折叠区 */}
            <div style={{ marginBottom: showSystemPrompt ? 12 : 0 }}>
              <Button
                type="text"
                size="small"
                icon={<SettingOutlined />}
                onClick={() => setShowSystemPrompt(!showSystemPrompt)}
                style={{ marginBottom: showSystemPrompt ? 8 : 0, color: '#888' }}
              >
                系统提示词 {systemPrompt ? '(已设置)' : ''}
              </Button>
              {showSystemPrompt && (
                <div style={{ background: '#fafafa', borderRadius: 8, padding: 12 }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                    <span style={{ fontSize: 12, color: '#888' }}>选择模板或手动输入</span>
                    <Button
                      type="text"
                      size="small"
                      icon={<CloseOutlined />}
                      onClick={() => { setSystemPrompt(''); setShowSystemPrompt(false); }}
                    />
                  </div>
                  <Select
                    style={{ width: '100%', marginBottom: 8 }}
                    allowClear
                    placeholder="选择 Prompt 模板..."
                    onChange={handleTemplateSelect}
                    options={Object.entries(grouped).map(([cat, items]) => ({
                      label: categoryLabels[cat] || cat,
                      options: items.map((t: any) => ({
                        value: t.key,
                        label: t.name,
                      })),
                    }))}
                  />
                  <Input.TextArea
                    rows={3}
                    value={systemPrompt}
                    onChange={(e) => setSystemPrompt(e.target.value)}
                    placeholder="手动输入系统提示词..."
                  />
                </div>
              )}
            </div>

            {/* Intervention toolbar */}
            {isStreaming && (
              <div style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                padding: '6px 12px',
                background: isPaused ? '#fff7e6' : '#e6f7ff',
                borderRadius: 6,
                marginBottom: 12,
                border: `1px solid ${isPaused ? '#ffd591' : '#91d5ff'}`,
              }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <LoadingOutlined style={{ color: isPaused ? '#fa8c16' : '#1890ff' }} />
                  <span style={{ fontSize: 13, color: isPaused ? '#fa8c16' : '#1890ff' }}>
                    {isPaused ? 'Agent 已暂停' : 'Agent 正在响应...'}
                  </span>
                </div>
                <Space size={4}>
                  {!isPaused ? (
                    <Button
                      type="text"
                      size="small"
                      icon={<PauseCircleOutlined />}
                      onClick={pauseAgent}
                      style={{ color: '#fa8c16', borderColor: '#ffd591' }}
                    >
                      暂停
                    </Button>
                  ) : (
                    <Button
                      type="text"
                      size="small"
                      icon={<PlayCircleOutlined />}
                      onClick={resumeAgent}
                      style={{ color: '#52c41a', borderColor: '#b7eb8f' }}
                    >
                      恢复
                    </Button>
                  )}
                  <Button
                    type="text"
                    size="small"
                    icon={<StopOutlined />}
                    onClick={stopAgent}
                    style={{ color: '#ff4d4f' }}
                  >
                    停止
                  </Button>
                  <Button
                    type="text"
                    size="small"
                    icon={<MessageOutlined />}
                    onClick={() => setInjectVisible(!injectVisible)}
                    style={{ color: '#1890ff' }}
                  >
                    注入
                  </Button>
                </Space>
              </div>
            )}

            {/* Inject message input */}
            {isStreaming && injectVisible && (
              <div style={{
                display: 'flex',
                gap: 8,
                padding: '8px 12px',
                background: '#f9f0ff',
                borderRadius: 6,
                marginBottom: 12,
                border: '1px solid #d3adf7',
              }}>
                <Input
                  size="small"
                  value={injectValue}
                  onChange={(e) => setInjectValue(e.target.value)}
                  onPressEnter={() => {
                    if (injectValue.trim()) {
                      injectMessage(injectValue.trim());
                      setInjectValue('');
                      setInjectVisible(false);
                    }
                  }}
                  placeholder="输入要注入的消息..."
                  style={{ flex: 1 }}
                />
                <Button
                  size="small"
                  type="primary"
                  disabled={!injectValue.trim()}
                  onClick={() => {
                    if (injectValue.trim()) {
                      injectMessage(injectValue.trim());
                      setInjectValue('');
                      setInjectVisible(false);
                    }
                  }}
                >
                  发送
                </Button>
              </div>
            )}

            <div className="chat-messages">
              {messages.length === 0 ? (
                <EmptyState
                  description="开始一段新的对话"
                  action={{
                    text: '发送消息',
                    onClick: () => {},
                  }}
                />
              ) : (
                <>
                  {messages.map((msg) => (
                    <ChatMessage key={msg.id} message={msg} />
                  ))}
                  <div ref={messagesEndRef} />
                </>
              )}
            </div>

            <div className="chat-input-area">
              <ChatInput
                onSend={sendMessage}
                loading={isLoading}
              />
            </div>
          </Card>
        </Col>
      </Row>
    </div>
  );
}
