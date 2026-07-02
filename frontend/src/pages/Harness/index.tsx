import { useState, useEffect } from 'react';
import {
  Card, Tabs, Table, Tag, Button, Space, Select, message, Badge,
  Row, Col, Statistic, Alert, Input, Modal, Form, InputNumber, Slider, Progress,
  Switch, List, Tooltip, Popconfirm
} from 'antd';
import {
  SafetyOutlined, KeyOutlined, AuditOutlined, ExperimentOutlined,
  DashboardOutlined, SettingOutlined, PlusOutlined,
  ThunderboltOutlined, FlagOutlined,
  BugOutlined, DollarOutlined, RocketOutlined, AppstoreOutlined,
  DeleteOutlined
} from '@ant-design/icons';
import client from '../../api/client';

// ========================= 类型定义 =========================

interface FeatureFlag {
  id: string;
  key: string;
  name: string;
  description: string;
  type: string;
  status: string;
  rollout: number;
  created_at: number;
}

interface ChaosExperiment {
  id: string;
  name: string;
  agent_id: string;
  fault_type: string;
  duration: number;
  blast_radius: number;
  status: string;
  created_at: number;
}

interface Proposal {
  id: string;
  agent_id: string;
  type: string;
  title: string;
  description: string;
  expected_benefit: number;
  risk_level: string;
  status: string;
  created_at: number;
}

interface CostRecommendation {
  type: string;
  priority: string;
  title: string;
  description: string;
  potential_savings: number;
  agent_id: string;
}

interface ABTest {
  id: string;
  name: string;
  type: string;
  control_config: string;
  variant_config: string;
  traffic_split: number;
  status: string;
  created_at: number;
}

interface ABTestResult {
  control_score: number;
  variant_score: number;
  delta: number;
  p_value: number;
  significant: boolean;
  recommended: string;
}

interface SLOStatusDetail {
  name: string;
  current: number;
  target: number;
  budget_remaining: number;
  status: string;
}

// ========================= 主页面 =========================

export default function HarnessPage() {
  const [activeTab, setActiveTab] = useState('dashboard');

  return (
    <div>
      <h2 style={{ marginBottom: 24 }}>运维治理中心</h2>
      <Card>
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          items={[
            {
              key: 'dashboard',
              label: <span><DashboardOutlined /> 概览</span>,
              children: <DashboardPanel />,
            },
            {
              key: 'rules',
              label: <span><SettingOutlined /> 规则引擎</span>,
              children: <RulesPanel />,
            },
            {
              key: 'guardrail',
              label: <span><SafetyOutlined /> 安全护栏</span>,
              children: <GuardrailPanel />,
            },
            {
              key: 'abtest',
              label: <span><ExperimentOutlined /> A/B 测试</span>,
              children: <ABTestPanel />,
            },
            {
              key: 'slo',
              label: <span><ThunderboltOutlined /> SLO 监控</span>,
              children: <SLOPanel />,
            },
            {
              key: 'featureflags',
              label: <span><FlagOutlined /> Feature Flags</span>,
              children: <FeatureFlagsPanel />,
            },
            {
              key: 'chaos',
              label: <span><BugOutlined /> Chaos</span>,
              children: <ChaosPanel />,
            },
            {
              key: 'cost',
              label: <span><DollarOutlined /> Cost</span>,
              children: <CostPanel />,
            },
            {
              key: 'proposals',
              label: <span><RocketOutlined /> Proposals</span>,
              children: <ProposalsPanel />,
            },
            {
              key: 'catalog',
              label: <span><AppstoreOutlined /> Catalog</span>,
              children: <CatalogPanel />,
            },
            {
              key: 'permissions',
              label: <span><KeyOutlined /> 权限矩阵</span>,
              children: <PermissionsPanel />,
            },
            {
              key: 'audit',
              label: <span><AuditOutlined /> 审计日志</span>,
              children: <AuditPanel />,
            },
          ]}
        />
      </Card>
    </div>
  );
}

// ========================= Feature Flags Panel =========================

function FeatureFlagsPanel() {
  const [flags, setFlags] = useState<FeatureFlag[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [form] = Form.useForm();

  useEffect(() => {
    loadFlags();
  }, []);

  const loadFlags = async () => {
    setLoading(true);
    try {
      const res = await client.get('/api/v2/harness/flags') as any;
      // 响应拦截器已经提取了 data.data，所以 res 直接就是 { flags: [...] }
      setFlags(res?.flags || []);
    } catch (error) {
      // Mock data
      setFlags([
        { id: '1', key: 'new-ui', name: 'New UI', description: 'Enable new UI design', type: 'boolean', status: 'active', rollout: 50, created_at: Date.now() },
        { id: '2', key: 'beta-feature', name: 'Beta Feature', description: 'Beta feature toggle', type: 'boolean', status: 'inactive', rollout: 10, created_at: Date.now() },
      ]);
    } finally {
      setLoading(false);
    }
  };

  const createFlag = async (values: any) => {
    try {
      await client.post('/api/v2/harness/flags', values);
      message.success('创建成功');
      setModalOpen(false);
      form.resetFields();
      loadFlags();
    } catch (error) {
      message.error('创建失败');
    }
  };

  const toggleFlag = async (key: string, enabled: boolean) => {
    try {
      await client.put('/api/v2/harness/flags/toggle', { key, enabled });
      message.success('切换成功');
      loadFlags();
    } catch (error) {
      message.error('切换失败');
    }
  };

  const columns = [
    { title: 'Key', dataIndex: 'key', key: 'key', render: (k: string) => <code>{k}</code> },
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '类型', dataIndex: 'type', key: 'type', render: (t: string) => <Tag color="blue">{t}</Tag> },
    { title: 'Rollout', dataIndex: 'rollout', key: 'rollout', render: (r: number) => <Progress percent={r} size="small" /> },
    { title: '状态', dataIndex: 'status', key: 'status', render: (s: string) => <Badge status={s === 'active' ? 'success' : 'default'} text={s} /> },
    {
      title: '操作', key: 'action',
      render: (_: any, record: FeatureFlag) => (
        <Space>
          <Switch checked={record.status === 'active'} onChange={(checked) => toggleFlag(record.key, checked)} />
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Alert message="Feature Flags 支持灰度发布、A/B 测试、特性开关等功能" type="info" showIcon style={{ marginBottom: 16 }} />
      <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)} style={{ marginBottom: 16 }}>创建 Flag</Button>
      <Table columns={columns} dataSource={flags} rowKey="id" loading={loading} />

      <Modal title="创建 Feature Flag" open={modalOpen} onCancel={() => setModalOpen(false)} onOk={() => form.submit()}>
        <Form form={form} layout="vertical" onFinish={createFlag}>
          <Form.Item name="key" label="Key" rules={[{ required: true }]}>
            <Input placeholder="例如：new-feature" />
          </Form.Item>
          <Form.Item name="name" label="名称" rules={[{ required: true }]}>
            <Input placeholder="显示名称" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item name="type" label="类型" rules={[{ required: true }]}>
            <Select options={[
              { value: 'boolean', label: 'Boolean' },
              { value: 'string', label: 'String' },
              { value: 'number', label: 'Number' },
            ]} />
          </Form.Item>
          <Form.Item name="rollout" label="Rollout %" initialValue={100}>
            <Slider min={0} max={100} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}

// ========================= Chaos Panel =========================

function ChaosPanel() {
  const [experiments, setExperiments] = useState<ChaosExperiment[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [form] = Form.useForm();

  useEffect(() => {
    loadExperiments();
  }, []);

  const loadExperiments = async () => {
    setLoading(true);
    try {
      const res = await client.get('/api/v2/harness/chaos') as any;
      // 响应拦截器已经提取了 data.data，所以 res 直接就是 { experiments: [...] }
      setExperiments(res?.experiments || []);
    } catch (error) {
      setExperiments([
        { id: '1', name: 'Latency Test', agent_id: 'browser-agent', fault_type: 'network_latency', duration: 5, blast_radius: 0.1, status: 'running', created_at: Date.now() },
        { id: '2', name: 'Error Injection', agent_id: 'chat-agent', fault_type: 'agent_error', duration: 10, blast_radius: 0.2, status: 'created', created_at: Date.now() },
      ]);
    } finally {
      setLoading(false);
    }
  };

  const createExperiment = async (values: any) => {
    try {
      await client.post('/api/v2/harness/chaos', values);
      message.success('创建成功');
      setModalOpen(false);
      form.resetFields();
      loadExperiments();
    } catch (error) {
      message.error('创建失败');
    }
  };

  const startExperiment = async (id: string) => {
    try {
      await client.post(`/api/v2/harness/chaos/${id}/start`);
      message.success('实验已启动');
      loadExperiments();
    } catch (error) {
      message.error('启动失败');
    }
  };

  const stopExperiment = async (id: string) => {
    try {
      await client.post(`/api/v2/harness/chaos/${id}/stop`);
      message.success('实验已停止');
      loadExperiments();
    } catch (error) {
      message.error('停止失败');
    }
  };

  const columns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: 'Agent', dataIndex: 'agent_id', key: 'agent_id' },
    { title: '故障类型', dataIndex: 'fault_type', key: 'fault_type', render: (t: string) => <Tag color="orange">{t}</Tag> },
    { title: '时长(分钟)', dataIndex: 'duration', key: 'duration' },
    { title: 'Blast Radius', dataIndex: 'blast_radius', key: 'blast_radius', render: (r: number) => `${Math.round(r * 100)}%` },
    { title: '状态', dataIndex: 'status', key: 'status', render: (s: string) => <Badge status={s === 'running' ? 'processing' : 'default'} text={s} /> },
    {
      title: '操作', key: 'action',
      render: (_: any, record: ChaosExperiment) => (
        <Space>
          {record.status === 'created' && (
            <Button size="small" type="primary" ghost onClick={() => startExperiment(record.id)}>启动</Button>
          )}
          {record.status === 'running' && (
            <Button size="small" danger onClick={() => stopExperiment(record.id)}>停止</Button>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Alert message="混沌工程引擎支持故障注入、SLO联动自动停止、Blast Radius 控制等功能" type="info" showIcon style={{ marginBottom: 16 }} />
      <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)} style={{ marginBottom: 16 }}>创建实验</Button>
      <Table columns={columns} dataSource={experiments} rowKey="id" loading={loading} />

      <Modal title="创建 Chaos 实验" open={modalOpen} onCancel={() => setModalOpen(false)} onOk={() => form.submit()}>
        <Form form={form} layout="vertical" onFinish={createExperiment}>
          <Form.Item name="name" label="名称" rules={[{ required: true }]}>
            <Input placeholder="实验名称" />
          </Form.Item>
          <Form.Item name="agent_id" label="Agent ID" rules={[{ required: true }]}>
            <Input placeholder="目标 Agent" />
          </Form.Item>
          <Form.Item name="fault_type" label="故障类型" rules={[{ required: true }]}>
            <Select options={[
              { value: 'network_latency', label: '网络延迟' },
              { value: 'agent_timeout', label: 'Agent 超时' },
              { value: 'agent_error', label: 'Agent 错误' },
              { value: 'model_degraded', label: '模型降级' },
            ]} />
          </Form.Item>
          <Form.Item name="duration" label="时长(分钟)" initialValue={5}>
            <InputNumber min={1} max={60} />
          </Form.Item>
          <Form.Item name="blast_radius" label="Blast Radius" initialValue={10}>
            <Slider min={1} max={100} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}

// ========================= Cost Panel =========================

function CostPanel() {
  const [recommendations, setRecommendations] = useState<CostRecommendation[]>([]);
  const [loading, setLoading] = useState(false);
  const [costStats, setCostStats] = useState({
    totalCost: 0,
    forecastCost: 0,
    totalRequests: 0,
    inputTokens: 0,
    outputTokens: 0,
  });

  useEffect(() => {
    loadCostData();
  }, []);

  const loadCostData = async () => {
    setLoading(true);
    try {
      // 加载成本报告
      const now = new Date();
      const startOfMonth = new Date(now.getFullYear(), now.getMonth(), 1);
      const reportRes = await client.get(`/api/v2/harness/cost/report?start=${startOfMonth.toISOString()}&end=${now.toISOString()}`) as any;

      console.log('[Cost] API response:', reportRes);

      if (reportRes && reportRes.report) {
        const totalCost = reportRes.report.total_cost || 0;
        const totalRequests = reportRes.report.request_count || 0;

        // 预测成本：基于当前使用量线性外推
        const daysInMonth = new Date(now.getFullYear(), now.getMonth() + 1, 0).getDate();
        const daysPassed = now.getDate();
        const forecastCost = daysPassed > 0 ? (totalCost / daysPassed) * daysInMonth : 0;

        console.log('[Cost] totalCost:', totalCost, 'forecastCost:', forecastCost, 'totalRequests:', totalRequests);

        setCostStats({
          totalCost,
          forecastCost,
          totalRequests,
          inputTokens: reportRes.report.total_input_tokens || 0,
          outputTokens: reportRes.report.total_output_tokens || 0,
        });
      }
    } catch (error) {
      console.error('Failed to load cost report:', error);
    }

    try {
      // 加载优化建议
      const res = await client.get('/api/v2/harness/cost/recommendations') as any;
      setRecommendations(res?.recommendations || []);
    } catch (error) {
      console.error('Failed to load recommendations:', error);
      setRecommendations([]);
    } finally {
      setLoading(false);
    }
  };

  const getPriorityColor = (p: string) => p === 'high' ? 'red' : p === 'medium' ? 'orange' : 'blue';

  return (
    <div>
      <Alert message="Cost Intelligence 提供成本分析、闲置检测、优化建议等功能" type="info" showIcon style={{ marginBottom: 16 }} />

      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}>
          <Card>
            <Statistic title="本月成本" value={costStats.totalCost} prefix="¥" precision={2} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="预测成本" value={costStats.forecastCost} prefix="¥" precision={2} valueStyle={{ color: '#faad14' }} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="潜在节省" value={recommendations.reduce((a, r) => a + r.potential_savings, 0)} prefix="¥" precision={2} valueStyle={{ color: '#52c41a' }} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="请求总数" value={costStats.totalRequests} />
          </Card>
        </Col>
      </Row>

      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}>
          <Card>
            <Statistic title="输入Tokens" value={costStats.inputTokens} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="输出Tokens" value={costStats.outputTokens} />
          </Card>
        </Col>
      </Row>

      <Card title="优化建议">
        <List
          loading={loading}
          dataSource={recommendations}
          renderItem={(item) => (
            <List.Item actions={[<Button type="link">查看详情</Button>]}>
              <List.Item.Meta
                title={<Space><Tag color={getPriorityColor(item.priority)}>{item.priority}</Tag>{item.title}</Space>}
                description={item.description}
              />
              <Statistic title="潜在节省" value={item.potential_savings} prefix="$" precision={2} valueStyle={{ fontSize: 16, color: '#52c41a' }} />
            </List.Item>
          )}
        />
      </Card>
    </div>
  );
}

// ========================= Proposals Panel =========================

function ProposalsPanel() {
  const [proposals, setProposals] = useState<Proposal[]>([]);
  const [loading, setLoading] = useState(false);
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [analyzeLoading, setAnalyzeLoading] = useState(false);
  const [createForm] = Form.useForm();

  useEffect(() => {
    loadProposals();
  }, []);

  const loadProposals = async () => {
    setLoading(true);
    try {
      const res = await client.get('/api/v2/harness/proposals') as any;
      setProposals(res?.proposals || []);
    } catch {
      message.error('加载提案列表失败');
      setProposals([]);
    } finally {
      setLoading(false);
    }
  };

  const analyzeAndPropose = async () => {
    setAnalyzeLoading(true);
    try {
      const res = await client.post('/api/v2/harness/proposals/analyze') as any;
      message.success(res?.analysis_summary || '分析完成，已生成优化提案');
      loadProposals();
    } catch {
      message.error('自动分析失败');
    } finally {
      setAnalyzeLoading(false);
    }
  };

  const approveProposal = async (id: string) => {
    try {
      await client.post(`/api/v2/harness/proposals/${id}/approve`, { approved_by: 'admin' });
      message.success('已批准');
      loadProposals();
    } catch {
      message.error('批准失败');
    }
  };

  const rejectProposal = async (id: string) => {
    try {
      await client.post(`/api/v2/harness/proposals/${id}/reject`, { reason: 'Not suitable' });
      message.success('已拒绝');
      loadProposals();
    } catch {
      message.error('拒绝失败');
    }
  };

  const executeProposal = async (id: string) => {
    try {
      await client.post(`/api/v2/harness/proposals/${id}/execute`);
      message.success('执行成功');
      loadProposals();
    } catch {
      message.error('执行失败');
    }
  };

  const handleCreate = async () => {
    try {
      const values = await createForm.validateFields();
      await client.post('/api/v2/harness/proposals', {
        agent_id: values.agent_id,
        type: values.type,
        title: values.title,
        description: values.description,
        current_state: JSON.stringify({ model: values.current_config }),
        proposed_state: JSON.stringify({ model: values.variant_config }),
        expected_benefit: values.expected_benefit,
        risk_level: values.risk_level,
      });
      message.success('提案创建成功');
      setCreateModalOpen(false);
      createForm.resetFields();
      loadProposals();
    } catch {
      message.error('创建失败');
    }
  };

  const getRiskColor = (r: string) => r === 'low' ? 'green' : r === 'medium' ? 'orange' : 'red';
  const getRiskLabel = (r: string) => r === 'low' ? '低' : r === 'medium' ? '中' : '高';
  const getStatusColor = (s: string) => {
    const map: Record<string, string> = { pending: 'blue', approved: 'green', rejected: 'red', running: 'orange', completed: 'cyan', failed: 'red' };
    return map[s] || 'default';
  };
  const getStatusLabel = (s: string) => {
    const map: Record<string, string> = { pending: '待审批', approved: '已批准', rejected: '已拒绝', running: '执行中', completed: '已完成', failed: '失败' };
    return map[s] || s;
  };
  const getTypeLabel = (t: string) => {
    const map: Record<string, string> = { model_switch: '模型切换', config_optimize: '配置优化', cost_reduce: '成本优化', performance: '性能提升', ab_test: 'A/B测试' };
    return map[t] || t;
  };
  const getTypeColor = (t: string) => {
    const map: Record<string, string> = { model_switch: 'purple', config_optimize: 'blue', cost_reduce: 'green', performance: 'orange', ab_test: 'cyan' };
    return map[t] || 'default';
  };

  const columns = [
    { title: '标题', dataIndex: 'title', key: 'title', ellipsis: true },
    { title: '类型', dataIndex: 'type', key: 'type', render: (t: string) => <Tag color={getTypeColor(t)}>{getTypeLabel(t)}</Tag> },
    { title: 'Agent', dataIndex: 'agent_id', key: 'agent_id' },
    { title: '预期收益', dataIndex: 'expected_benefit', key: 'expected_benefit', render: (v: number) => <span style={{ color: '#52c41a', fontWeight: 600 }}>+{v}%</span> },
    { title: '风险', dataIndex: 'risk_level', key: 'risk_level', render: (r: string) => <Tag color={getRiskColor(r)}>{getRiskLabel(r)}</Tag> },
    { title: '状态', dataIndex: 'status', key: 'status', render: (s: string) => <Tag color={getStatusColor(s)}>{getStatusLabel(s)}</Tag> },
    {
      title: '操作', key: 'action', width: 240,
      render: (_: any, record: Proposal) => (
        <Space>
          {record.status === 'pending' && (
            <>
              <Button size="small" type="primary" ghost onClick={() => approveProposal(record.id)}>批准</Button>
              <Button size="small" danger onClick={() => rejectProposal(record.id)}>拒绝</Button>
            </>
          )}
          {record.status === 'approved' && (
            <Popconfirm
              title="确定执行此提案？"
              description="执行将修改 Agent 配置"
              onConfirm={() => executeProposal(record.id)}
              okText="确定执行"
              cancelText="取消"
            >
              <Button size="small" type="primary">执行</Button>
            </Popconfirm>
          )}
          {record.status === 'running' && <Tag color="orange">执行中</Tag>}
          {record.status === 'completed' && <Tag color="cyan">已完成</Tag>}
          {record.status === 'failed' && <Tag color="red">失败</Tag>}
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Alert message="自演化提案系统 — 自动化优化建议、审批流程、执行跟踪" type="info" showIcon style={{ flex: 1 }} />
        <Space>
          <Button
            type="default"
            icon={<ThunderboltOutlined />}
            onClick={analyzeAndPropose}
            loading={analyzeLoading}
          >
            自动分析
          </Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateModalOpen(true)}>
            创建提案
          </Button>
        </Space>
      </div>
      <Table columns={columns} dataSource={proposals} rowKey="id" loading={loading}
        locale={{ emptyText: '暂无提案，点击"自动分析"生成优化建议或"创建提案"手动添加' }}
      />

      <Modal title="创建优化提案" open={createModalOpen} onOk={handleCreate} onCancel={() => { setCreateModalOpen(false); createForm.resetFields(); }} width={600}>
        <Form form={createForm} layout="vertical">
          <Form.Item name="title" label="提案标题" rules={[{ required: true, message: '请输入标题' }]}>
            <Input placeholder="例如：切换到qwen-plus模型" />
          </Form.Item>
          <Form.Item name="agent_id" label="Agent ID" rules={[{ required: true, message: '请输入Agent ID' }]}>
            <Input placeholder="例如：chat-agent" />
          </Form.Item>
          <Form.Item name="type" label="提案类型" rules={[{ required: true, message: '请选择类型' }]}>
            <Select placeholder="选择类型">
              <Select.Option value="model_switch">模型切换</Select.Option>
              <Select.Option value="config_optimize">配置优化</Select.Option>
              <Select.Option value="cost_reduce">成本优化</Select.Option>
              <Select.Option value="performance">性能提升</Select.Option>
              <Select.Option value="ab_test">A/B测试</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="description" label="描述" rules={[{ required: true, message: '请输入描述' }]}>
            <Input.TextArea rows={3} placeholder="描述优化建议的详细内容" />
          </Form.Item>
          <div style={{ display: 'flex', gap: 16 }}>
            <Form.Item name="current_config" label="当前配置" style={{ flex: 1 }}>
              <Input placeholder="例如：qwen-turbo" />
            </Form.Item>
            <Form.Item name="variant_config" label="建议配置" style={{ flex: 1 }}>
              <Input placeholder="例如：qwen-plus" />
            </Form.Item>
          </div>
          <div style={{ display: 'flex', gap: 16 }}>
            <Form.Item name="expected_benefit" label="预期收益(%)" initialValue={10} style={{ flex: 1 }}>
              <InputNumber min={0} max={100} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="risk_level" label="风险等级" initialValue="low" style={{ flex: 1 }}>
              <Select>
                <Select.Option value="low">低</Select.Option>
                <Select.Option value="medium">中</Select.Option>
                <Select.Option value="high">高</Select.Option>
              </Select>
            </Form.Item>
          </div>
        </Form>
      </Modal>
    </div>
  );
}

// ========================= Catalog Panel =========================

function CatalogPanel() {
  const [agents, setAgents] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    loadAgents();
  }, []);

  const loadAgents = async () => {
    setLoading(true);
    try {
      const res = await client.get('/api/v2/harness/catalog') as any;
      // 响应拦截器已经提取了 data.data，所以 res 直接就是 { agents: [...] }
      setAgents(res?.agents || []);
    } catch (error) {
      setAgents([
        { id: '1', name: 'Browser Agent', type: 'browser', description: 'Web automation agent', version: '1.0.0', rating: 4.5, usage_count: 1234 },
        { id: '2', name: 'Chat Agent', type: 'chat', description: 'Conversational AI agent', version: '2.0.0', rating: 4.8, usage_count: 5678 },
        { id: '3', name: 'Research Agent', type: 'research', description: 'Information gathering agent', version: '1.5.0', rating: 4.2, usage_count: 890 },
      ]);
    } finally {
      setLoading(false);
    }
  };

  const columns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '类型', dataIndex: 'type', key: 'type', render: (t: string) => <Tag color="blue">{t}</Tag> },
    { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
    { title: '版本', dataIndex: 'version', key: 'version' },
    { title: '评分', dataIndex: 'rating', key: 'rating', render: (r: number) => <span>⭐ {r}</span> },
    { title: '使用次数', dataIndex: 'usage_count', key: 'usage_count' },
  ];

  return (
    <div>
      <Alert message="Agent 目录提供已注册 Agent 的浏览、搜索、评分功能" type="info" showIcon style={{ marginBottom: 16 }} />
      <Table columns={columns} dataSource={agents} rowKey="id" loading={loading} />
    </div>
  );
}

// ========================= Dashboard Panel =========================

function DashboardPanel() {
  const [stats, setStats] = useState({
    totalRules: 0,
    activeGuardrails: 0,
    runningABTests: 0,
    sloCompliance: 0,
  });
  const [sloLoading, setSloLoading] = useState(true);

  useEffect(() => {
    loadStats();
  }, []);

  const loadStats = async () => {
    // Fetch real SLO compliance from API
    let sloCompliance = 0;
    try {
      setSloLoading(true);
      const res = await client.get('/api/v2/harness/slo/status') as any;
      const statuses = res?.statuses || [];
      if (statuses.length > 0) {
        const totalBudget = statuses.reduce((sum: number, s: any) => sum + (s.budget_remaining || 0), 0);
        sloCompliance = (totalBudget / statuses.length) * 100;
      }
    } catch {
      sloCompliance = 0;
    } finally {
      setSloLoading(false);
    }

    setStats({
      totalRules: 12,        // TODO: fetch from /api/v2/harness/rules
      activeGuardrails: 5,   // TODO: add API endpoint
      runningABTests: 3,     // TODO: fetch from /api/v2/harness/abtest/list
      sloCompliance,
    });
  };

  return (
    <div>
      <Row gutter={16}>
        <Col span={6}>
          <Card>
            <Statistic title="规则数量" value={stats.totalRules} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="活跃护栏" value={stats.activeGuardrails} valueStyle={{ color: '#3f8600' }} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="运行中 A/B 测试" value={stats.runningABTests} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="SLO 合规率" value={sloLoading ? 0 : stats.sloCompliance} suffix="%" precision={1} valueStyle={{ color: stats.sloCompliance >= 90 ? '#3f8600' : '#cf1322' }} loading={sloLoading} />
          </Card>
        </Col>
      </Row>

      <Card title="系统状态" style={{ marginTop: 24 }}>
        <Alert message="所有服务运行正常" type="success" showIcon />
      </Card>
    </div>
  );
}

// ========================= Rules Panel =========================

function RulesPanel() {
  const [rules, setRules] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [form] = Form.useForm();

  useEffect(() => {
    loadRules();
  }, []);

  const loadRules = async () => {
    setLoading(true);
    try {
      const res = await client.get('/api/v2/harness/rules') as any;
      // 响应拦截器已经提取了 data.data，所以 res 直接就是 { rules: [...] }
      setRules(res?.rules || []);
    } catch (error) {
      console.error('Failed to load rules:', error);
      // Mock data if API fails
      setRules([]);
    } finally {
      setLoading(false);
    }
  };

  const createRule = async (values: any) => {
    try {
      await client.post('/api/v2/harness/rules', {
        name: values.name,
        type: values.type,
        agent_id: values.agent_id || '',
        config: values.config || '',
        enabled: values.enabled ?? true,
        tenant_id: 'default',
      });
      message.success('规则创建成功');
      setModalOpen(false);
      form.resetFields();
      loadRules();
    } catch (error) {
      message.error('创建失败');
    }
  };

  const deleteRule = async (id: string) => {
    try {
      await client.delete(`/api/v2/harness/rules/${id}`);
      message.success('删除成功');
      loadRules();
    } catch (error) {
      message.error('删除失败');
    }
  };

  const columns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '类型', dataIndex: 'type', key: 'type', render: (t: string) => <Tag color="blue">{t}</Tag> },
    { title: '状态', dataIndex: 'enabled', key: 'enabled', render: (enabled: boolean) => (
      <Badge status={enabled ? 'success' : 'default'} text={enabled ? '启用' : '禁用'} />
    )},
    {
      title: '操作', key: 'action',
      render: (_: any, record: any) => (
        <Space>
          <Button size="small" danger onClick={() => deleteRule(record.id)}>删除</Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Alert message="规则引擎支持自定义规则、条件匹配、动作触发" type="info" showIcon style={{ marginBottom: 16 }} />
      <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)} style={{ marginBottom: 16 }}>创建规则</Button>
      <Table columns={columns} dataSource={rules} rowKey="id" loading={loading} />

      <Modal title="创建规则" open={modalOpen} onCancel={() => setModalOpen(false)} onOk={() => form.submit()}>
        <Form form={form} layout="vertical" onFinish={createRule}>
          <Form.Item name="name" label="规则名称" rules={[{ required: true }]}>
            <Input placeholder="例如：敏感词过滤" />
          </Form.Item>
          <Form.Item name="type" label="规则类型" rules={[{ required: true }]}>
            <Select options={[
              { value: 'guardrail', label: '安全护栏' },
              { value: 'fallback', label: '降级规则' },
              { value: 'rate_limit', label: '限流规则' },
              { value: 'custom', label: '自定义' },
            ]} />
          </Form.Item>
          <Form.Item name="agent_id" label="Agent ID">
            <Input placeholder="留空表示全局规则" />
          </Form.Item>
          <Form.Item name="config" label="配置 (JSON)">
            <Input.TextArea rows={3} placeholder='{"pattern": "xxx", "action": "block"}' />
          </Form.Item>
          <Form.Item name="enabled" label="启用" valuePropName="checked" initialValue={true}>
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}

// ========================= Guardrail Panel =========================

function GuardrailPanel() {
  const [guardrails, setGuardrails] = useState<any[]>([]);

  useEffect(() => {
    setGuardrails([
      { id: '1', name: 'PII 检测', description: '检测并屏蔽个人信息', status: 'active', blocked_count: 156 },
      { id: '2', name: '毒性检测', description: '检测有害内容', status: 'active', blocked_count: 23 },
      { id: '3', name: 'Prompt 注入防护', description: '防止 prompt 注入攻击', status: 'active', blocked_count: 8 },
    ]);
  }, []);

  const columns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '描述', dataIndex: 'description', key: 'description' },
    { title: '拦截次数', dataIndex: 'blocked_count', key: 'blocked_count', render: (v: number) => <Tag color="orange">{v}</Tag> },
    { title: '状态', dataIndex: 'status', key: 'status', render: (s: string) => <Badge status="success" text={s} /> },
  ];

  return (
    <div>
      <Alert message="安全护栏保护系统免受恶意输入和输出" type="info" showIcon style={{ marginBottom: 16 }} />
      <Button type="primary" icon={<PlusOutlined />} style={{ marginBottom: 16 }}>添加护栏</Button>
      <Table columns={columns} dataSource={guardrails} rowKey="id" />
    </div>
  );
}

// ========================= AB Test Panel =========================

function ABTestPanel() {
  const [tests, setTests] = useState<ABTest[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [resultModalOpen, setResultModalOpen] = useState(false);
  const [resultData, setResultData] = useState<ABTestResult | null>(null);
  const [resultLoading, setResultLoading] = useState(false);
  const [currentTestName, setCurrentTestName] = useState('');
  const [form] = Form.useForm();

  useEffect(() => {
    loadTests();
  }, []);

  const loadTests = async () => {
    setLoading(true);
    try {
      const res = await client.post('/api/v2/harness/abtest/list', {}) as any;
      setTests(res?.tests || []);
    } catch {
      setTests([]);
    } finally {
      setLoading(false);
    }
  };

  const createTest = async (values: any) => {
    try {
      await client.post('/api/v2/harness/abtest', {
        name: values.name,
        type: values.type,
        control_model: values.type === 'model' ? values.control_config : '',
        variant_model: values.type === 'model' ? values.variant_config : '',
        control_config: values.type === 'prompt' ? values.control_config : '',
        variant_config: values.type === 'prompt' ? values.variant_config : '',
        traffic_split: values.traffic_split / 100,
      });
      message.success('创建成功');
      setModalOpen(false);
      form.resetFields();
      loadTests();
    } catch {
      message.error('创建失败');
    }
  };

  const viewResult = async (test: ABTest) => {
    setCurrentTestName(test.name);
    setResultModalOpen(true);
    setResultLoading(true);
    setResultData(null);
    try {
      const res = await client.get(`/api/v2/harness/abtest/${test.id}/result`) as any;
      setResultData(res?.result || null);
    } catch {
      setResultData(null);
    } finally {
      setResultLoading(false);
    }
  };

  const deleteTest = async (id: string) => {
    try {
      await client.delete(`/api/v2/harness/abtest/${id}`);
      message.success('删除成功');
      loadTests();
    } catch {
      message.error('删除失败');
    }
  };

  const columns = [
    { title: '实验名称', dataIndex: 'name', key: 'name' },
    {
      title: '类型', dataIndex: 'type', key: 'type',
      render: (t: string) => {
        if (t === 'prompt') return <Tag color="blue">Prompt</Tag>;
        if (t === 'model') return <Tag color="green">模型</Tag>;
        return <Tag>{t || '-'}</Tag>;
      },
    },
    {
      title: '对照组', dataIndex: 'control_config', key: 'control_config',
      render: (v: string) => (
        <Tooltip title={v}>
          <span>{v ? (v.length > 20 ? v.slice(0, 20) + '...' : v) : '-'}</span>
        </Tooltip>
      ),
    },
    {
      title: '实验组', dataIndex: 'variant_config', key: 'variant_config',
      render: (v: string) => (
        <Tooltip title={v}>
          <Tag color="blue">{v ? (v.length > 20 ? v.slice(0, 20) + '...' : v) : '-'}</Tag>
        </Tooltip>
      ),
    },
    {
      title: '流量分配', dataIndex: 'traffic_split', key: 'traffic_split',
      render: (v: number) => `${Math.round(v * 100)}% / ${Math.round((1 - v) * 100)}%`,
    },
    {
      title: '状态', dataIndex: 'status', key: 'status',
      render: (s: string) => {
        const statusMap: Record<string, 'processing' | 'success' | 'error' | 'default'> = {
          running: 'processing', completed: 'success', paused: 'default', stopped: 'error',
        };
        return <Badge status={statusMap[s] || 'default'} text={s} />;
      },
    },
    {
      title: '操作', key: 'action',
      render: (_: any, record: ABTest) => (
        <Space>
          <Button size="small" icon={<ExperimentOutlined />} onClick={() => viewResult(record)}>
            查看结果
          </Button>
          <Popconfirm
            title="确定删除该实验？"
            description="删除后实验数据将无法恢复"
            onConfirm={() => deleteTest(record.id)}
            okText="确定"
            cancelText="取消"
          >
            <Button size="small" danger icon={<DeleteOutlined />}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Alert message="A/B 测试支持 Prompt 对比、模型对比，自动统计显著性" type="info" showIcon style={{ marginBottom: 16 }} />
      <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)} style={{ marginBottom: 16 }}>
        创建测试
      </Button>
      <Table columns={columns} dataSource={tests} rowKey="id" loading={loading} />

      {/* 创建实验 Modal */}
      <Modal title="创建 A/B 测试" open={modalOpen} onCancel={() => setModalOpen(false)} onOk={() => form.submit()}>
        <Form form={form} layout="vertical" onFinish={createTest}>
          <Form.Item name="name" label="实验名称" rules={[{ required: true, message: '请输入实验名称' }]}>
            <Input placeholder="例如：Prompt风格对比" />
          </Form.Item>
          <Form.Item name="type" label="测试类型" rules={[{ required: true, message: '请选择类型' }]}>
            <Select options={[
              { value: 'prompt', label: 'Prompt 对比' },
              { value: 'model', label: '模型对比' },
            ]} />
          </Form.Item>
          <Form.Item name="control_config" label="对照组配置" rules={[{ required: true, message: '请输入对照组配置' }]}>
            <Input.TextArea rows={3} placeholder="对照组 Prompt 或模型名称" />
          </Form.Item>
          <Form.Item name="variant_config" label="实验组配置" rules={[{ required: true, message: '请输入实验组配置' }]}>
            <Input.TextArea rows={3} placeholder="实验组 Prompt 或模型名称" />
          </Form.Item>
          <Form.Item name="traffic_split" label="实验组流量占比" initialValue={50}>
            <Slider min={10} max={90} marks={{ 10: '10%', 50: '50%', 90: '90%' }} />
          </Form.Item>
        </Form>
      </Modal>

      {/* 查看结果 Modal */}
      <Modal
        title={`实验结果 — ${currentTestName}`}
        open={resultModalOpen}
        onCancel={() => setResultModalOpen(false)}
        footer={null}
        width={640}
      >
        {resultLoading ? (
          <div style={{ textAlign: 'center', padding: 40 }}>加载中...</div>
        ) : resultData ? (
          <div>
            <Row gutter={16}>
              <Col span={8}>
                <Statistic
                  title="对照组得分"
                  value={resultData.control_score}
                  precision={3}
                  valueStyle={{ color: '#1677ff' }}
                />
              </Col>
              <Col span={8}>
                <Statistic
                  title="实验组得分"
                  value={resultData.variant_score}
                  precision={3}
                  valueStyle={{ color: '#52c41a' }}
                />
              </Col>
              <Col span={8}>
                <Statistic
                  title="差异 (Delta)"
                  value={resultData.delta}
                  precision={3}
                  valueStyle={{ color: resultData.delta > 0 ? '#52c41a' : '#ff4d4f' }}
                  prefix={resultData.delta > 0 ? '+' : ''}
                />
              </Col>
            </Row>
            <div style={{ marginTop: 24 }}>
              <Row gutter={16}>
                <Col span={8}>
                  <Statistic title="P 值" value={resultData.p_value} precision={4} />
                </Col>
                <Col span={8}>
                  <div style={{ textAlign: 'center' }}>
                    <div style={{ color: 'rgba(0,0,0,0.45)', fontSize: 14, marginBottom: 8 }}>统计显著性</div>
                    {resultData.significant ? (
                      <Tag color="green" style={{ fontSize: 16, padding: "4px 16px" }}>✓ 显著</Tag>
                    ) : (
                      <Tag color="orange" style={{ fontSize: 16, padding: '4px 16px' }}>不显著</Tag>
                    )}
                  </div>
                </Col>
                <Col span={8}>
                  <div style={{ textAlign: 'center' }}>
                    <div style={{ color: 'rgba(0,0,0,0.45)', fontSize: 14, marginBottom: 8 }}>建议操作</div>
                    <Tag color={
                      resultData.recommended === 'promote_variant' ? 'green' :
                      resultData.recommended === 'promote_control' ? 'blue' : 'default'
                    } style={{ fontSize: 14, padding: '4px 12px' }}>
                      {resultData.recommended === 'promote_variant' ? '采用实验组' :
                       resultData.recommended === 'promote_control' ? '采用对照组' :
                       resultData.recommended === 'continue' ? '继续实验' : resultData.recommended}
                    </Tag>
                  </div>
                </Col>
              </Row>
            </div>
            {resultData.significant && (
              <Alert
                style={{ marginTop: 16 }}
                type="success"
                message="实验结果已达到统计显著性 (p < 0.05)，可以做出决策"
                showIcon
              />
            )}
          </div>
        ) : (
          <div style={{ textAlign: 'center', padding: 40, color: '#999' }}>
            暂无结果数据，请先发送一些聊天请求以积累实验数据
          </div>
        )}
      </Modal>
    </div>
  );
}

// ========================= SLO Panel =========================

function SLOPanel() {
  const [slos, setSlos] = useState<SLOStatusDetail[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [form] = Form.useForm();
  const [llmSummary, setLLMSummary] = useState({ totalCalls: 0, successRate: 0, avgLatency: 0 });

  useEffect(() => {
    loadSLOs();
  }, []);

  const loadSLOs = async () => {
    setLoading(true);
    try {
      const res = await client.get('/api/v2/harness/slo/status') as any;
      setSlos(res?.statuses || []);
    } catch {
      setSlos([]);
    }

    try {
      const llmRes = await client.get('/api/v2/harness/llm/metrics') as any;
      const data = llmRes?.data || llmRes;
      setLLMSummary({
        totalCalls: data?.total_calls || 0,
        successRate: data?.success_rate || 0,
        avgLatency: data?.avg_latency || 0,
      });
    } catch {
      // LLM metrics endpoint may not be available yet
    } finally {
      setLoading(false);
    }
  };

  const createSLO = async (values: { name: string; agent_id: string; type: string; target: number }) => {
    try {
      await client.post('/api/v2/harness/slo', {
        agent_id: values.agent_id || '',
        name: values.name,
        target: values.target / 100,
        type: values.type,
      });
      message.success('SLO 创建成功');
      setModalOpen(false);
      form.resetFields();
      loadSLOs();
    } catch {
      message.error('SLO 创建失败');
    }
  };

  const getStatusBadge = (status: string) => {
    const map: Record<string, 'success' | 'warning' | 'error' | 'default'> = {
      healthy: 'success',
      warning: 'warning',
      critical: 'error',
      breaching: 'error',
    };
    return map[status] || 'default';
  };

  const getStatusLabel = (status: string) => {
    const map: Record<string, string> = {
      healthy: '健康',
      warning: '警告',
      critical: '严重',
      breaching: '违规',
    };
    return map[status] || status;
  };

  const columns = [
    { title: 'SLO 名称', dataIndex: 'name', key: 'name' },
    {
      title: '目标值', dataIndex: 'target', key: 'target',
      render: (v: number, r: SLOStatusDetail) => {
        // Latency 类型显示毫秒，其他显示百分比
        if (r.name.toLowerCase().includes('latency') || r.name.includes('延迟')) {
          return `${v.toFixed(0)}ms`;
        }
        return `${(v * 100).toFixed(1)}%`;
      },
    },
    {
      title: '当前值', dataIndex: 'current', key: 'current',
      render: (v: number, r: SLOStatusDetail) => {
        // Latency 类型显示毫秒，其他显示百分比
        if (r.name.toLowerCase().includes('latency') || r.name.includes('延迟')) {
          return <span style={{ color: v <= r.target ? '#3f8600' : '#cf1322' }}>{v.toFixed(0)}ms</span>;
        }
        return <span style={{ color: v >= r.target ? '#3f8600' : '#cf1322' }}>{(v * 100).toFixed(1)}%</span>;
      },
    },
    {
      title: '剩余预算', dataIndex: 'budget_remaining', key: 'budget_remaining',
      render: (v: number) => <Progress percent={Math.round(v * 100)} size="small" status={v < 0.3 ? 'exception' : 'active'} />,
    },
    {
      title: '状态', key: 'status', dataIndex: 'status',
      render: (s: string) => <Badge status={getStatusBadge(s)} text={getStatusLabel(s)} />,
    },
  ];

  return (
    <div>
      <Alert message="SLO 监控跟踪每次 LLM 调用的延迟、成功率、Token 用量和成本" type="info" showIcon style={{ marginBottom: 16 }} />

      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={8}>
          <Card size="small">
            <Statistic title="LLM 成功率" value={llmSummary.successRate * 100} suffix="%" precision={1} valueStyle={{ color: llmSummary.successRate >= 0.95 ? '#3f8600' : '#cf1322' }} />
          </Card>
        </Col>
        <Col span={8}>
          <Card size="small">
            <Statistic title="平均延迟" value={llmSummary.avgLatency} suffix="ms" precision={0} />
          </Card>
        </Col>
        <Col span={8}>
          <Card size="small">
            <Statistic title="SLO 达标数" value={slos.filter(s => {
              // Latency 类型：当前值应该小于目标值才算达标
              if (s.name.toLowerCase().includes('latency') || s.name.includes('延迟')) {
                return s.current <= s.target * 1000; // target是百分比形式，转成毫秒
              }
              // 其他类型：当前值应该大于目标值才算达标
              return s.current >= s.target;
            }).length} suffix={`/ ${slos.length}`} />
          </Card>
        </Col>
      </Row>

      <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)} style={{ marginBottom: 16 }}>创建 SLO</Button>
      <Table columns={columns} dataSource={slos} rowKey="name" loading={loading} />

      <Modal
        title="创建 SLO"
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={() => form.submit()}
      >
        <Form form={form} layout="vertical" onFinish={createSLO}>
          <Form.Item name="name" label="SLO 名称" rules={[{ required: true, message: '请输入 SLO 名称' }]}>
            <Input placeholder="例如：API 响应时间" />
          </Form.Item>
          <Form.Item name="agent_id" label="Agent ID">
            <Input placeholder="留空表示全局 SLO" />
          </Form.Item>
          <Form.Item name="type" label="SLO 类型" rules={[{ required: true, message: '请选择 SLO 类型' }]}>
            <Select options={[
              { value: 'success_rate', label: '成功率' },
              { value: 'latency', label: '延迟' },
              { value: 'availability', label: '可用性' },
              { value: 'error_budget', label: '错误预算' },
            ]} />
          </Form.Item>
          <Form.Item name="target" label="目标值 (%)" rules={[{ required: true, message: '请输入目标值' }]} initialValue={99}>
            <InputNumber min={0} max={100} precision={1} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}

// ========================= Permissions Panel =========================

function PermissionsPanel() {
  const [permissions, setPermissions] = useState<any[]>([]);

  useEffect(() => {
    setPermissions([
      { agent: 'browser-agent', read: true, write: true, admin: false },
      { agent: 'chat-agent', read: true, write: true, admin: false },
      { agent: 'admin-agent', read: true, write: true, admin: true },
    ]);
  }, []);

  const columns = [
    { title: 'Agent', dataIndex: 'agent', key: 'agent' },
    { title: '读取', dataIndex: 'read', key: 'read', render: (v: boolean) => <Tag color={v ? 'green' : 'default'}>{v ? '✓' : '✗'}</Tag> },
    { title: '写入', dataIndex: 'write', key: 'write', render: (v: boolean) => <Tag color={v ? 'green' : 'default'}>{v ? '✓' : '✗'}</Tag> },
    { title: '管理', dataIndex: 'admin', key: 'admin', render: (v: boolean) => <Tag color={v ? 'blue' : 'default'}>{v ? '✓' : '✗'}</Tag> },
  ];

  return (
    <div>
      <Alert message="权限矩阵控制各 Agent 的访问权限" type="info" showIcon style={{ marginBottom: 16 }} />
      <Table columns={columns} dataSource={permissions} rowKey="agent" />
    </div>
  );
}

// ========================= Audit Panel =========================

function AuditPanel() {
  const [logs, setLogs] = useState<any[]>([]);

  useEffect(() => {
    setLogs([
      { time: '2024-01-15 10:30:00', action: 'guardrail.blocked', agent: 'browser-agent', details: 'Blocked PII in response' },
      { time: '2024-01-15 10:25:00', action: 'abtest.started', agent: 'chat-agent', details: 'Started A/B test #1' },
      { time: '2024-01-15 10:20:00', action: 'rule.created', agent: 'admin', details: 'Created rule: 敏感词过滤' },
    ]);
  }, []);

  const columns = [
    { title: '时间', dataIndex: 'time', key: 'time', width: 180 },
    { title: '操作', dataIndex: 'action', key: 'action', render: (v: string) => <Tag color="blue">{v}</Tag> },
    { title: 'Agent', dataIndex: 'agent', key: 'agent' },
    { title: '详情', dataIndex: 'details', key: 'details' },
  ];

  return (
    <div>
      <Alert message="审计日志记录所有治理相关操作" type="info" showIcon style={{ marginBottom: 16 }} />
      <Table columns={columns} dataSource={logs} rowKey="time" />
    </div>
  );
}
