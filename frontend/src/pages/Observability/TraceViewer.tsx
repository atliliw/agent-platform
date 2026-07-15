import { useState, useEffect, useMemo } from 'react';
import {
  Card,
  Table,
  Input,
  Select,
  DatePicker,
  Button,
  Drawer,
  Descriptions,
  Tag,
  Tree,
  Timeline,
  Alert,
  Pagination,
  Space,
  Statistic,
  Row,
  Col,
  message,
  Tooltip,
} from 'antd';
import {
  SearchOutlined,
  ReloadOutlined,
  ClockCircleOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ExclamationCircleOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import dayjs from 'dayjs';
import client from '../../api/client';
import type { Trace, Span, Bottleneck } from '../../api/observability';

const { RangePicker } = DatePicker;

// 状态颜色映射
const statusColors: Record<string, string> = {
  ok: 'success',
  error: 'error',
  blocked: 'warning',
  pending: 'processing',
};

// 获取状态图标
const getStatusIcon = (status: string) => {
  switch (status) {
    case 'ok':
      return <CheckCircleOutlined style={{ color: '#52c41a' }} />;
    case 'error':
      return <CloseCircleOutlined style={{ color: '#ff4d4f' }} />;
    case 'blocked':
      return <ExclamationCircleOutlined style={{ color: '#faad14' }} />;
    default:
      return <ClockCircleOutlined style={{ color: '#1890ff' }} />;
  }
};

interface LLMMetric {
  id?: string;
  trace_id?: string;
  model?: string;
  caller?: string;
  agent_id?: string;
  input_tokens?: number;
  output_tokens?: number;
  total_tokens?: number;
  latency_ms?: number;
  cost?: number;
  success?: boolean;
  timestamp?: string | number;
}

// 生成唯一 ID。crypto.randomUUID 仅在安全上下文（HTTPS / localhost）可用，
// 部署在明文 HTTP 下时回退到 getRandomValues + 时间戳，避免 TypeError。
function generateId(prefix = 'id'): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  const rand =
    typeof crypto !== 'undefined' && typeof crypto.getRandomValues === 'function'
      ? Array.from(crypto.getRandomValues(new Uint8Array(8)), (b) => b.toString(16).padStart(2, '0')).join('')
      : Math.random().toString(16).slice(2, 18);
  return `${prefix}-${Date.now().toString(36)}-${rand}`;
}

function convertMetricsToTraces(metrics: LLMMetric[]): Trace[] {
  return metrics.map((m) => {
    const traceId = m.trace_id || m.id || generateId('trace');
    const tsMs = typeof m.timestamp === 'number'
      ? m.timestamp * 1000
      : (m.timestamp ? new Date(m.timestamp).getTime() : Date.now());
    const startTimeISO = typeof m.timestamp === 'number'
      ? new Date(m.timestamp * 1000).toISOString()
      : (m.timestamp || new Date().toISOString());
    const spanId = m.id || generateId('span');
    const tokens = (m.input_tokens || 0) + (m.output_tokens || 0) || m.total_tokens || 0;

    return {
      trace_id: traceId,
      session_id: undefined,
      agent_id: m.caller || m.agent_id,
      operation: m.model || 'llm_call',
      status: m.success ? 'ok' : 'error',
      started_at: startTimeISO,
      ended_at: m.latency_ms
        ? new Date(tsMs + m.latency_ms).toISOString()
        : undefined,
      latency_ms: m.latency_ms || 0,
      tokens,
      cost: m.cost || 0,
      spans: [
        {
          id: spanId,
          trace_id: traceId,
          operation: `${m.caller || m.agent_id || 'unknown'} → ${m.model || 'unknown'}`,
          status: m.success ? 'ok' : 'error',
          started_at: startTimeISO,
          duration_ms: m.latency_ms || 0,
          attributes: {
            model: m.model,
            tokens_in: m.input_tokens ?? m.total_tokens,
            tokens_out: m.output_tokens,
            cost: m.cost,
          },
        },
      ],
      bottlenecks: [],
    };
  });
}

export default function TraceViewer() {
  // 搜索和筛选状态
  const [searchQuery, setSearchQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState<string | undefined>();
  const [dateRange, setDateRange] = useState<[dayjs.Dayjs, dayjs.Dayjs] | null>(null);
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [total, setTotal] = useState(0);

  // 数据状态
  const [traces, setTraces] = useState<Trace[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedTrace, setSelectedTrace] = useState<Trace | null>(null);
  const [detailVisible, setDetailVisible] = useState(false);

  // 统计数据
  const [stats, setStats] = useState({
    totalTraces: 0,
    successRate: 0,
    avgLatency: 0,
    totalTokens: 0,
    totalCost: 0,
  });

  // 加载追踪数据
  const loadTraces = async () => {
    setLoading(true);
    try {
      const params: Record<string, unknown> = {};
      if (searchQuery) params.search = searchQuery;
      if (statusFilter) params.status = statusFilter;
      if (dateRange) {
        params.start_time = dateRange[0].toISOString();
        params.end_time = dateRange[1].toISOString();
      }

      const response = await client.get('/api/v2/harness/llm/metrics', { params });
      const responseData = response?.data || response;
      const metrics: LLMMetric[] = responseData?.metrics || responseData || [];
      const converted = convertMetricsToTraces(Array.isArray(metrics) ? metrics : []);
      setTraces(converted);
      setTotal(converted.length);

      const okCount = converted.filter((t) => t.status === 'ok').length;
      const totalLatency = converted.reduce((sum, t) => sum + t.latency_ms, 0);
      const totalTokens = converted.reduce((sum, t) => sum + t.tokens, 0);
      const totalCost = converted.reduce((sum, t) => sum + t.cost, 0);

      setStats({
        totalTraces: converted.length,
        successRate: converted.length > 0 ? (okCount / converted.length) * 100 : 0,
        avgLatency: converted.length > 0 ? totalLatency / converted.length : 0,
        totalTokens,
        totalCost,
      });
    } catch (error) {
      message.error('加载追踪数据失败');
      console.error(error);
      setTraces([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  };

  // 查看追踪详情
  const viewTrace = (trace: Trace) => {
    setSelectedTrace(trace);
    setDetailVisible(true);
  };

  // 导出追踪
  const exportTrace = (trace: Trace) => {
    const data = JSON.stringify(trace, null, 2);
    const blob = new Blob([data], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `trace-${trace.trace_id}.json`;
    a.click();
    URL.revokeObjectURL(url);
  };

  // Tree 节点类型
  interface TreeNodeData {
    key: string;
    title: React.ReactNode;
    children?: TreeNodeData[];
  }

  // 构建 Span 树形数据
  const buildSpanTree = (spans: Span[]): TreeNodeData[] => {
    const spanMap = new Map<string, Span & { children: Span[] }>();
    const roots: (Span & { children: Span[] })[] = [];

    spans.forEach((span) => {
      spanMap.set(span.id, { ...span, children: [] });
    });

    spans.forEach((span) => {
      const node = spanMap.get(span.id)!;
      if (span.parent_id) {
        const parent = spanMap.get(span.parent_id);
        if (parent) {
          parent.children.push(node);
        }
      } else {
        roots.push(node);
      }
    });

    return roots.map((node) => convertToTreeNode(node));
  };

  // 转换为 Tree 节点
  const convertToTreeNode = (span: Span & { children: Span[] }): TreeNodeData => ({
    key: span.id,
    title: (
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <span style={{ fontWeight: 500 }}>{span.operation}</span>
        <span style={{ color: '#999' }}>{span.duration_ms}ms</span>
        <Tag color={statusColors[span.status]} style={{ marginLeft: 4 }}>
          {span.status}
        </Tag>
      </div>
    ),
    children: span.children?.map((child) => convertToTreeNode(child as Span & { children: Span[] })),
  });

  // 过滤后的追踪列表
  const filteredTraces = useMemo(() => {
    return traces;
  }, [traces]);

  // Span 树形数据
  const spanTreeData = useMemo(() => {
    if (!selectedTrace?.spans) return [];
    return buildSpanTree(selectedTrace.spans);
  }, [selectedTrace]);

  // 瓶颈数据
  const bottlenecks = useMemo(() => {
    return selectedTrace?.bottlenecks || [];
  }, [selectedTrace]);

  // 表格列定义
  const columns: ColumnsType<Trace> = [
    {
      title: '追踪ID',
      dataIndex: 'trace_id',
      key: 'trace_id',
      width: 200,
      render: (id: string) => (
        <Tooltip title={id}>
          <Tag>{id.substring(0, 16)}...</Tag>
        </Tooltip>
      ),
    },
    {
      title: '操作',
      dataIndex: 'operation',
      key: 'operation',
      width: 180,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: string) => (
        <Tag color={statusColors[status]} icon={getStatusIcon(status)}>
          {status}
        </Tag>
      ),
    },
    {
      title: '延迟(ms)',
      dataIndex: 'latency_ms',
      key: 'latency_ms',
      width: 120,
      sorter: true,
      render: (ms: number) => (
        <span style={{ color: ms > 1000 ? '#ff4d4f' : undefined, fontWeight: ms > 1000 ? 'bold' : undefined }}>
          {ms}
        </span>
      ),
    },
    {
      title: 'Token',
      dataIndex: 'tokens',
      key: 'tokens',
      width: 100,
      sorter: true,
    },
    {
      title: '成本',
      dataIndex: 'cost',
      key: 'cost',
      width: 100,
      sorter: true,
      render: (cost: number) => `$${cost.toFixed(4)}`,
    },
    {
      title: '时间',
      dataIndex: 'started_at',
      key: 'started_at',
      width: 180,
      render: (time: string) => dayjs(time).format('YYYY-MM-DD HH:mm:ss'),
    },
    {
      title: '操作',
      key: 'action',
      fixed: 'right',
      width: 150,
      render: (_: unknown, record: Trace) => (
        <Space>
          <Button size="small" onClick={() => viewTrace(record)}>
            查看
          </Button>
          <Button size="small" type="primary" ghost onClick={() => exportTrace(record)}>
            导出
          </Button>
        </Space>
      ),
    },
  ];

  // 初始化加载
  useEffect(() => {
    loadTraces();
  }, [currentPage, pageSize]);

  return (
    <div className="trace-viewer">
      {/* 头部 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <h2 style={{ margin: 0 }}>执行追踪查看器</h2>
        <div style={{ display: 'flex', gap: 12 }}>
          <Input
            placeholder="搜索追踪..."
            prefix={<SearchOutlined />}
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            style={{ width: 240 }}
            onPressEnter={loadTraces}
          />
          <Select
            placeholder="状态筛选"
            value={statusFilter}
            onChange={setStatusFilter}
            style={{ width: 120 }}
            allowClear
            options={[
              { value: 'ok', label: '成功' },
              { value: 'error', label: '错误' },
              { value: 'blocked', label: '阻塞' },
            ]}
          />
          <RangePicker
            showTime
            value={dateRange}
            onChange={(dates) => setDateRange(dates as [dayjs.Dayjs, dayjs.Dayjs] | null)}
          />
          <Button icon={<ReloadOutlined />} onClick={loadTraces}>
            刷新
          </Button>
        </div>
      </div>

      {/* 统计卡片 */}
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={4}>
          <Card>
            <Statistic title="总追踪数" value={stats.totalTraces} />
          </Card>
        </Col>
        <Col span={4}>
          <Card>
            <Statistic
              title="成功率"
              value={stats.successRate}
              precision={1}
              suffix="%"
              valueStyle={{ color: '#3f8600' }}
            />
          </Card>
        </Col>
        <Col span={4}>
          <Card>
            <Statistic title="平均延迟" value={stats.avgLatency} suffix="ms" />
          </Card>
        </Col>
        <Col span={4}>
          <Card>
            <Statistic title="总Token" value={stats.totalTokens} />
          </Card>
        </Col>
        <Col span={4}>
          <Card>
            <Statistic title="总成本" value={stats.totalCost} prefix="$" precision={4} />
          </Card>
        </Col>
      </Row>

      {/* 追踪列表 */}
      <Card>
        <Table
          columns={columns}
          dataSource={filteredTraces}
          rowKey="trace_id"
          loading={loading}
          pagination={false}
          scroll={{ x: 1200 }}
          onRow={(record) => ({
            onClick: () => viewTrace(record),
            style: { cursor: 'pointer' },
          })}
        />
        <div style={{ marginTop: 16, textAlign: 'right' }}>
          <Pagination
            current={currentPage}
            pageSize={pageSize}
            total={total}
            showSizeChanger
            showQuickJumper
            showTotal={(total) => `共 ${total} 条`}
            pageSizeOptions={['10', '20', '50', '100']}
            onChange={(page, size) => {
              setCurrentPage(page);
              setPageSize(size);
            }}
          />
        </div>
      </Card>

      {/* 追踪详情抽屉 */}
      <Drawer
        title="追踪详情"
        placement="right"
        width="60%"
        open={detailVisible}
        onClose={() => setDetailVisible(false)}
      >
        {selectedTrace && (
          <div>
            {/* 基本信息 */}
            <Descriptions title="基本信息" bordered column={2} style={{ marginBottom: 24 }}>
              <Descriptions.Item label="追踪ID">{selectedTrace.trace_id}</Descriptions.Item>
              <Descriptions.Item label="操作">{selectedTrace.operation}</Descriptions.Item>
              <Descriptions.Item label="状态">
                <Tag color={statusColors[selectedTrace.status]}>{selectedTrace.status}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="延迟">{selectedTrace.latency_ms}ms</Descriptions.Item>
              <Descriptions.Item label="Token">{selectedTrace.tokens}</Descriptions.Item>
              <Descriptions.Item label="成本">${selectedTrace.cost?.toFixed(4)}</Descriptions.Item>
            </Descriptions>

            {/* 执行步骤 */}
            <Card title="执行步骤" style={{ marginBottom: 24 }}>
              {spanTreeData.length > 0 ? (
                <Tree
                  showLine
                  defaultExpandAll
                  treeData={spanTreeData}
                />
              ) : (
                <div style={{ color: '#999', textAlign: 'center', padding: 24 }}>暂无执行步骤</div>
              )}
            </Card>

            {/* 时间线视图 */}
            <Card title="时间线" style={{ marginBottom: 24 }}>
              <Timeline
                items={selectedTrace.spans?.map((span) => ({
                  color: span.status === 'ok' ? 'green' : span.status === 'error' ? 'red' : 'orange',
                  children: (
                    <div>
                      <div style={{ fontWeight: 500 }}>{span.operation}</div>
                      <div style={{ color: '#999', fontSize: 12 }}>
                        {dayjs(span.started_at).format('HH:mm:ss')} - {span.duration_ms}ms
                      </div>
                      {span.error_msg && (
                        <div style={{ color: '#ff4d4f', fontSize: 12, marginTop: 4 }}>
                          {span.error_msg}
                        </div>
                      )}
                    </div>
                  ),
                }))}
              />
            </Card>

            {/* 性能瓶颈 */}
            {bottlenecks.length > 0 && (
              <Card title="性能瓶颈">
                {bottlenecks.map((bn: Bottleneck, idx: number) => (
                  <Alert
                    key={idx}
                    message={bn.operation}
                    description={`耗时 ${bn.duration}ms，占总时间 ${bn.percent}%`}
                    type={bn.severity === 'high' ? 'error' : bn.severity === 'medium' ? 'warning' : 'info'}
                    showIcon
                    style={{ marginBottom: 12 }}
                  />
                ))}
              </Card>
            )}
          </div>
        )}
      </Drawer>
    </div>
  );
}
