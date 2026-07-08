import { useState, useEffect } from 'react';
import {
  Table, Tag, Button, Space, Input, Select, InputNumber,
  Modal, Form, Popconfirm, message, Alert
} from 'antd';
import { PlusOutlined, ThunderboltOutlined } from '@ant-design/icons';
import { harnessApi } from '../../api/harness';

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

export default function ProposalsPanel() {
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
      const res = await harnessApi.listProposals() as any;
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
      const res = await harnessApi.analyzeProposals() as any;
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
      await harnessApi.approveProposal(id);
      message.success('已批准');
      loadProposals();
    } catch {
      message.error('批准失败');
    }
  };

  const rejectProposal = async (id: string) => {
    try {
      await harnessApi.rejectProposal(id, 'Not suitable');
      message.success('已拒绝');
      loadProposals();
    } catch {
      message.error('拒绝失败');
    }
  };

  const executeProposal = async (id: string) => {
    try {
      await harnessApi.executeProposal(id);
      message.success('执行成功');
      loadProposals();
    } catch {
      message.error('执行失败');
    }
  };

  const handleCreate = async () => {
    try {
      const values = await createForm.validateFields();
      await harnessApi.createProposal({
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
