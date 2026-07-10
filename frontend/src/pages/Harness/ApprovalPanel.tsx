import { useState, useEffect, useCallback } from 'react';
import {
  Card, Row, Col, Table, Tag, Button, Space, Badge, Switch,
  Modal, Form, Input, Popconfirm, message, Alert
} from 'antd';
import { CheckOutlined, CloseOutlined } from '@ant-design/icons';
import { harnessApi } from '../../api/harness';

interface ApprovalRule {
  id: string;
  type: string;
  agent_id: string;
  tool_name: string;
  condition: string;
  risk_threshold: string;
  auto_approve: boolean;
  timeout_seconds: number;
  enabled: boolean;
}

interface PendingApproval {
  id: string;
  request_type: string;
  requester: string;
  description: string;
  created_at: string;
  status: string;
}

export default function ApprovalPanel() {
  const [rules, setRules] = useState<ApprovalRule[]>([]);
  const [pending, setPending] = useState<PendingApproval[]>([]);
  const [rulesLoading, setRulesLoading] = useState(false);
  const [pendingLoading, setPendingLoading] = useState(false);
  const [rejectModalOpen, setRejectModalOpen] = useState(false);
  const [rejectTargetId, setRejectTargetId] = useState('');
  const [rejectForm] = Form.useForm();

  const loadRules = useCallback(async () => {
    setRulesLoading(true);
    try {
      const res = await harnessApi.listApprovalRules() as any;
      setRules(res?.rules || []);
    } catch {
      setRules([]);
    } finally {
      setRulesLoading(false);
    }
  }, []);

  const loadPending = useCallback(async () => {
    setPendingLoading(true);
    try {
      const res = await harnessApi.listPendingApprovals() as any;
      setPending(res?.pending || res?.approvals || res?.requests || []);
    } catch {
      setPending([]);
    } finally {
      setPendingLoading(false);
    }
  }, []);

  useEffect(() => {
    loadRules();
    loadPending();
  }, [loadRules, loadPending]);

  const handleApprove = async (id: string) => {
    try {
      await harnessApi.approveRequest(id);
      message.success('已批准');
      loadPending();
    } catch {
      message.error('批准失败');
    }
  };

  const handleReject = async () => {
    try {
      const values = await rejectForm.validateFields();
      await harnessApi.rejectRequest(rejectTargetId, values.reason);
      message.success('已拒绝');
      setRejectModalOpen(false);
      rejectForm.resetFields();
      loadPending();
    } catch {
      message.error('拒绝失败');
    }
  };

  const getTypeLabel = (t: string) => {
    const map: Record<string, string> = {
      tool_call: '工具调用',
      model_change: '模型变更',
      config_change: '配置变更',
      deployment: '部署审批',
      publish: '内容发布',
      cost_threshold: '成本阈值',
      custom: '自定义',
    };
    return map[t] || t;
  };

  const getTypeColor = (t: string) => {
    const map: Record<string, string> = {
      tool_call: 'purple',
      model_change: 'blue',
      config_change: 'cyan',
      deployment: 'green',
      publish: 'orange',
      cost_threshold: 'red',
      custom: 'default',
    };
    return map[t] || 'default';
  };

  const getRiskColor = (r: string) => r === 'low' ? 'green' : r === 'medium' ? 'orange' : 'red';
  const getRiskLabel = (r: string) => r === 'low' ? '低' : r === 'medium' ? '中' : '高';

  const ruleColumns = [
    { title: '规则 ID', dataIndex: 'id', key: 'id', render: (v: string) => <Tag color="blue">{v}</Tag> },
    { title: '类型', dataIndex: 'type', key: 'type', render: (t: string) => <Tag color={getTypeColor(t)}>{getTypeLabel(t)}</Tag> },
    { title: '工具/目标', dataIndex: 'tool_name', key: 'tool_name', render: (v: string) => v || '-' },
    { title: 'Agent', dataIndex: 'agent_id', key: 'agent_id', render: (v: string) => v || <Tag>全部</Tag> },
    {
      title: '风险等级', dataIndex: 'risk_threshold', key: 'risk_threshold', width: 90,
      render: (r: string) => <Tag color={getRiskColor(r)}>{getRiskLabel(r)}</Tag>,
    },
    {
      title: '自动审批', dataIndex: 'auto_approve', key: 'auto_approve', width: 90,
      render: (v: boolean) => v ? <Tag color="green">自动</Tag> : <Tag color="orange">手动</Tag>,
    },
    {
      title: '状态', dataIndex: 'enabled', key: 'enabled', width: 90,
      render: (enabled: boolean) => (
        <Tag color={enabled ? 'green' : 'default'}>{enabled ? '启用' : '禁用'}</Tag>
      ),
    },
  ];

  const pendingColumns = [
    { title: '请求类型', dataIndex: 'request_type', key: 'request_type', render: (t: string) => <Tag color="blue">{getTypeLabel(t) || t}</Tag> },
    { title: '请求人', dataIndex: 'requester', key: 'requester' },
    { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at', render: (v: string) => v ? new Date(v).toLocaleString() : '-' },
    {
      title: '操作', key: 'action', width: 200,
      render: (_: any, record: PendingApproval) => (
        <Space>
          <Button
            size="small"
            type="primary"
            icon={<CheckOutlined />}
            onClick={() => handleApprove(record.id)}
          >
            批准
          </Button>
          <Button
            size="small"
            danger
            icon={<CloseOutlined />}
            onClick={() => {
              setRejectTargetId(record.id);
              setRejectModalOpen(true);
            }}
          >
            拒绝
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Alert message="审批管理 — 配置审批规则，处理高风险操作审批请求" type="info" showIcon style={{ marginBottom: 16 }} />

      <Row gutter={16}>
        <Col span={14}>
          <Card title="审批规则" size="small">
            <Table
              columns={ruleColumns}
              dataSource={rules}
              rowKey="id"
              loading={rulesLoading}
              size="small"
              locale={{ emptyText: '暂无审批规则' }}
            />
          </Card>
        </Col>
        <Col span={10}>
          <Card
            title={
              <Space>
                待审批请求
                {pending.length > 0 && <Badge count={pending.length} />}
              </Space>
            }
            size="small"
          >
            <Table
              columns={pendingColumns}
              dataSource={pending}
              rowKey="id"
              loading={pendingLoading}
              size="small"
              locale={{ emptyText: '暂无待审批请求' }}
            />
          </Card>
        </Col>
      </Row>

      <Modal
        title="拒绝审批"
        open={rejectModalOpen}
        onOk={handleReject}
        onCancel={() => { setRejectModalOpen(false); rejectForm.resetFields(); }}
      >
        <Form form={rejectForm} layout="vertical">
          <Form.Item name="reason" label="拒绝原因" rules={[{ required: true, message: '请输入拒绝原因' }]}>
            <Input.TextArea rows={3} placeholder="请说明拒绝原因" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
