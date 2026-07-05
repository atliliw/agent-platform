import { useEffect, useRef, useState } from 'react';
import {
  Row, Col, Card, Select, Slider, InputNumber, Input, Button, Space,
  Tag, Tabs, Table, Badge, Statistic, Divider, Empty, Tooltip, message,
} from 'antd';
import {
  ThunderboltOutlined, ClearOutlined,
  StopOutlined, HistoryOutlined, BarChartOutlined,
  RobotOutlined, UserOutlined,
} from '@ant-design/icons';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import dayjs from 'dayjs';
import { usePlaygroundStore } from '../../stores/playgroundStore';
import { playgroundHelpers, AVAILABLE_MODELS } from '../../api/playground';
import { promptApi } from '../../api/prompt';
import type { PlaygroundMessage, PlaygroundResult } from '../../api/playground';
import './Playground.css';

const toNum = playgroundHelpers.toNumber;

// ==================== Sub-Components ====================

/** Message bubble for playground chat */
function PlaygroundMessageBubble({ msg }: { msg: PlaygroundMessage }) {
  const isUser = msg.role === 'user';

  return (
    <div className={`pg-message ${isUser ? 'pg-message-user' : 'pg-message-assistant'}`}>
      <div className="pg-message-avatar">
        {isUser ? <UserOutlined /> : <RobotOutlined />}
      </div>
      <div className={`pg-message-bubble ${isUser ? 'pg-bubble-user' : 'pg-bubble-assistant'}`}>
        {isUser ? (
          <div className="pg-message-text">{msg.content}</div>
        ) : (
          <div className="markdown-body">
            <ReactMarkdown remarkPlugins={[remarkGfm]}>
              {msg.content}
            </ReactMarkdown>
            {msg.isStreaming && <span className="pg-streaming-cursor" />}
          </div>
        )}

        {/* Metadata bar for assistant messages */}
        {!isUser && !msg.isStreaming && (msg.total_tokens || msg.cost || msg.latency) && (
          <div className="pg-message-meta">
            {msg.model && <Tag color="purple">{msg.model}</Tag>}
            {msg.total_tokens != null && <span>{toNum(msg.total_tokens).toLocaleString()} tokens</span>}
            {msg.cost != null && <span>$${Number(msg.cost).toFixed(4)}</span>}
            {msg.latency != null && <span>{toNum(msg.latency)}ms</span>}
          </div>
        )}
      </div>
    </div>
  );
}

/** History list panel */
function HistoryPanel() {
  const { history, loadHistory, loadHistorySession } = usePlaygroundStore();

  useEffect(() => {
    loadHistory();
  }, []);

  if (history.length === 0) {
    return <Empty description="暂无执行记录" />;
  }

  return (
    <div className="pg-history-list">
      {history.map((h) => (
        <div
          key={h.id}
          className="pg-history-item"
          onClick={() => {
            loadHistorySession(h);
            message.success('已恢复历史会话');
          }}
        >
          <div className="pg-history-item-header">
            <Tag color="blue">{h.model}</Tag>
            <span className="pg-history-time">
              {dayjs(Number(h.created_at) * 1000 || Date.now()).format('MM-DD HH:mm')}
            </span>
          </div>
          <div className="pg-history-item-preview">
            {h.messages?.find((m) => m.role === 'user')?.content?.slice(0, 60) || '—'}
          </div>
          {h.streamed && <Tag color="green" style={{ fontSize: 10 }}>流式</Tag>}
        </div>
      ))}
    </div>
  );
}

/** Stats panel */
function StatsPanel() {
  const { stats, loadStats } = usePlaygroundStore();

  useEffect(() => {
    loadStats();
  }, []);

  if (!stats) {
    return <Empty description="暂无统计数据" />;
  }

  return (
    <div className="pg-stats-panel">
      <Row gutter={[8, 16]}>
        <Col span={12}>
          <Statistic title="总执行次数" value={toNum(stats.total_executions)} />
        </Col>
        <Col span={12}>
          <Statistic title="流式执行" value={toNum(stats.streamed_executions)} />
        </Col>
        <Col span={12}>
          <Statistic title="对比执行" value={toNum(stats.comparison_executions)} />
        </Col>
        <Col span={12}>
          <Statistic title="总 Token" value={toNum(stats.total_tokens).toLocaleString()} />
        </Col>
        <Col span={12}>
          <Statistic title="总费用" value={Number(stats.total_cost)} prefix="$" precision={2} />
        </Col>
        <Col span={12}>
          <Statistic title="平均延迟" value={Number(stats.avg_latency)} suffix="ms" />
        </Col>
      </Row>

      {stats.model_counts && Object.keys(stats.model_counts).length > 0 && (
        <div style={{ marginTop: 16 }}>
          <Divider>模型使用分布</Divider>
          <Space wrap>
            {Object.entries(stats.model_counts).map(([model, count]) => (
              <Tag key={model} color="blue">{model}: {toNum(count)}</Tag>
            ))}
          </Space>
        </div>
      )}
    </div>
  );
}

/** Compare results panel */
function ComparePanel() {
  const { compareResults, comparison } = usePlaygroundStore();

  if (compareResults.length === 0) {
    return <Empty description="选择模型并执行对比" />;
  }

  const columns = [
    {
      title: '指标',
      dataIndex: 'metric',
      key: 'metric',
      width: 100,
      fixed: 'left' as const,
    },
    ...compareResults.map((r: PlaygroundResult, i: number) => ({
      title: (
        <Tag color={i === 0 ? 'blue' : i === 1 ? 'green' : 'orange'}>
          {r.model || `模型 ${i + 1}`}
        </Tag>
      ),
      dataIndex: `model_${i}`,
      key: `model_${i}`,
      width: 180,
    })),
  ];

  const dataSource = [
    {
      key: 'content',
      metric: '内容摘要',
      ...compareResults.map((r: PlaygroundResult, i: number) => ({
        [`model_${i}`]: r.content?.slice(0, 200) + (r.content?.length > 200 ? '...' : '') || '—',
      })),
    },
    {
      key: 'tokens',
      metric: '总 Token',
      ...compareResults.map((r: PlaygroundResult, i: number) => ({
        [`model_${i}`]: toNum(r.total_tokens).toLocaleString(),
      })),
    },
    {
      key: 'cost',
      metric: '费用',
      ...compareResults.map((r: PlaygroundResult, i: number) => ({
        [`model_${i}`]: `$${Number(r.cost).toFixed(4)}`,
      })),
    },
    {
      key: 'latency',
      metric: '延迟',
      ...compareResults.map((r: PlaygroundResult, i: number) => ({
        [`model_${i}`]: `${toNum(r.latency)}ms`,
      })),
    },
    {
      key: 'finish',
      metric: '结束原因',
      ...compareResults.map((r: PlaygroundResult, i: number) => ({
        [`model_${i}`]: r.finish_reason || '—',
      })),
    },
  ];

  return (
    <div className="pg-compare-panel">
      {comparison && (
        <div className="pg-compare-summary">
          <Space wrap>
            {comparison.best_model && <Tag color="green">🏆 最佳: {comparison.best_model}</Tag>}
            {comparison.fastest_model && <Tag color="blue">⚡ 最快: {comparison.fastest_model}</Tag>}
            {comparison.cheapest_model && <Tag color="orange">💰 最省: {comparison.cheapest_model}</Tag>}
          </Space>
          <div style={{ marginTop: 8, fontSize: 12, color: '#888' }}>
            平均延迟: {Number(comparison.avg_latency).toFixed(1)}ms | 平均费用: $${Number(comparison.avg_cost).toFixed(4)} | 平均 Token: {Number(comparison.avg_tokens).toFixed(0)}
          </div>
        </div>
      )}

      <Table
        columns={columns}
        dataSource={dataSource}
        pagination={false}
        size="small"
        scroll={{ x: 400 }}
        className="pg-compare-table"
      />

      {/* Full content cards */}
      <Divider>完整输出</Divider>
      <div className="pg-compare-contents">
        {compareResults.map((r: PlaygroundResult, i: number) => (
          <Card
            key={i}
            title={<Tag color={i === 0 ? 'blue' : i === 1 ? 'green' : 'orange'}>{r.model}</Tag>}
            size="small"
            style={{ marginBottom: 8 }}
          >
            <div className="markdown-body">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>
                {r.content || 'No output'}
              </ReactMarkdown>
            </div>
          </Card>
        ))}
      </div>
    </div>
  );
}

/** Prompt template selector dropdown */
function PromptTemplateSelector({ onSelect, currentPrompt }: { onSelect: (content: string) => void; currentPrompt: string }) {
  const [templates, setTemplates] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedKey, setSelectedKey] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    promptApi.listPrompts()
      .then((res: any) => {
        const list = res?.prompts || [];
        setTemplates(list);
      })
      .catch(() => setTemplates([]))
      .finally(() => setLoading(false));
  }, []);

  const handleSelect = async (key: string) => {
    if (!key) {
      setSelectedKey(null);
      return;
    }
    setSelectedKey(key);
    try {
      const res: any = await promptApi.getActiveVersion(key);
      const content = res?.content || '';
      if (content) {
        onSelect(content);
      }
    } catch {
      // Template has no active version, ignore
    }
  };

  // Group templates by category
  const grouped: Record<string, any[]> = templates.reduce((acc, t: any) => {
    const cat = t.category || 'other';
    if (!acc[cat]) acc[cat] = [];
    acc[cat].push(t);
    return acc;
  }, {} as Record<string, any[]>);

  const categoryLabels: Record<string, string> = {
    system: '系统指令',
    agent: 'Agent',
    rag: 'RAG',
    template: '模板',
    user: '用户',
    other: '其他',
  };

  return (
    <Select
      style={{ width: '100%' }}
      value={selectedKey || undefined}
      onChange={handleSelect}
      loading={loading}
      allowClear
      placeholder="选择 Prompt 模板..."
      options={Object.entries(grouped).map(([cat, items]) => ({
        label: categoryLabels[cat] || cat,
        options: items.map((t: any) => ({
          value: t.key,
          label: `${t.name}${t.description ? ' - ' + t.description.slice(0, 30) : ''}`,
        })),
      }))}
    />
  );
}

// ==================== Main Page ====================

export default function PlaygroundPage() {
  const store = usePlaygroundStore();
  const {
    model, temperature, maxTokens, topP, systemPrompt,
    messages, isLoading, isStreaming,
    compareModels, showComparePanel,
  } = store;

  const messagesEndRef = useRef<HTMLDivElement>(null);

  // Auto-scroll on new messages
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  // Load history and stats on mount
  useEffect(() => {
    store.loadHistory();
    store.loadStats();
  }, []);

  const handleSend = (content: string) => {
    if (!content.trim()) return;
    store.sendMessage(content);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      const textarea = e.target as HTMLTextAreaElement;
      if (textarea.value.trim()) {
        handleSend(textarea.value.trim());
        textarea.value = '';
      }
    }
  };

  const handleCompare = async () => {
    if (compareModels.length < 2) {
      message.warning('请选择至少 2 个模型进行对比');
      return;
    }
    await store.executeCompare();
  };

  const sidebarTabs = [
    {
      key: 'history',
      label: <span><HistoryOutlined /> 历史</span>,
      children: <HistoryPanel />,
    },
    {
      key: 'stats',
      label: <span><BarChartOutlined /> 统计</span>,
      children: <StatsPanel />,
    },
  ];

  return (
    <div className="playground-page">
      <Row gutter={16} style={{ height: '100%' }}>
        {/* ===== Left Sidebar: Config + History/Stats ===== */}
        <Col span={6}>
          <Card title="模型配置" size="small" className="pg-config-card">
            <div className="pg-config-section">
              <label>模型</label>
              <Select
                value={model}
                onChange={store.setModel}
                style={{ width: '100%' }}
                options={AVAILABLE_MODELS.map((m) => ({
                  value: m.id,
                  label: `${m.name} (${m.provider})`,
                }))}
              />
            </div>
            <div className="pg-config-section">
              <label>Temperature: {temperature}</label>
              <Slider min={0} max={2} step={0.1} value={temperature} onChange={store.setTemperature} />
            </div>
            <div className="pg-config-section">
              <label>Max Tokens</label>
              <InputNumber min={1} max={32768} value={maxTokens} onChange={(v) => store.setMaxTokens(v || 2048)} style={{ width: '100%' }} />
            </div>
            <div className="pg-config-section">
              <label>Top P: {topP}</label>
              <Slider min={0} max={1} step={0.05} value={topP} onChange={store.setTopP} />
            </div>
            <div className="pg-config-section">
              <label>System Prompt</label>
              <PromptTemplateSelector
                onSelect={(content) => store.setSystemPrompt(content)}
                currentPrompt={systemPrompt}
              />
              <Input.TextArea
                rows={3}
                value={systemPrompt}
                onChange={(e) => store.setSystemPrompt(e.target.value)}
                placeholder="选择模板或手动输入 System Prompt..."
                style={{ marginTop: 4 }}
              />
            </div>

            <Divider style={{ margin: '8px 0' }} />

            {/* Compare model selection */}
            <div className="pg-config-section">
              <label>对比模型 (≥2)</label>
              <Select
                mode="multiple"
                value={compareModels}
                onChange={store.setCompareModels}
                style={{ width: '100%' }}
                placeholder="选择模型"
                options={AVAILABLE_MODELS.map((m) => ({
                  value: m.id,
                  label: m.name,
                }))}
              />
              <Button
                type="primary"
                icon={<ThunderboltOutlined />}
                onClick={handleCompare}
                loading={store.isComparing}
                disabled={compareModels.length < 2}
                style={{ width: '100%', marginTop: 8 }}
              >
                对比执行
              </Button>
            </div>
          </Card>

          <Tabs
            items={sidebarTabs}
            className="pg-sidebar-tabs"
            style={{ marginTop: 8 }}
          />
        </Col>

        {/* ===== Center: Chat Area ===== */}
        <Col span={showComparePanel ? 12 : 18}>
          <Card className="pg-chat-card">
            <div className="pg-chat-messages">
              {messages.length === 0 ? (
                <Empty
                  description="选择模型，输入消息开始测试"
                  className="pg-empty-state"
                />
              ) : (
                <>
                  {messages.map((msg) => (
                    <PlaygroundMessageBubble key={msg.id} msg={msg} />
                  ))}
                  <div ref={messagesEndRef} />
                </>
              )}
            </div>

            <div className="pg-chat-input-area">
              <Input.TextArea
                placeholder="输入消息，按 Enter 发送，Shift+Enter 换行"
                autoSize={{ minRows: 1, maxRows: 6 }}
                onKeyDown={handleKeyDown}
                disabled={isLoading}
                className="pg-chat-input"
              />
              <div className="pg-chat-actions">
                <Space>
                  <Tooltip title="清空对话">
                    <Button icon={<ClearOutlined />} onClick={store.clearMessages} disabled={isLoading} />
                  </Tooltip>
                  {isStreaming && (
                    <Button
                      danger
                      icon={<StopOutlined />}
                      onClick={store.stopStreaming}
                    >
                      Stop
                    </Button>
                  )}
                  <Badge status={isLoading ? 'processing' : 'success'} text={isLoading ? '执行中...' : '就绪'} />
                </Space>
              </div>
            </div>
          </Card>
        </Col>

        {/* ===== Right: Compare Panel ===== */}
        {showComparePanel && (
          <Col span={6}>
            <Card
              title="模型对比"
              size="small"
              className="pg-compare-card"
              extra={
                <Button size="small" onClick={() => store.setShowComparePanel(false)}>
                  收起
                </Button>
              }
            >
              <ComparePanel />
            </Card>
          </Col>
        )}
      </Row>
    </div>
  );
}
