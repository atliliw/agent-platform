import { useState, useEffect, useCallback } from 'react';
import {
  Card, Row, Col, Table, Tag, Button, Space, Badge, Switch,
  Modal, Form, Input, Popconfirm, message, Alert
} from 'antd';
import { CheckOutlined, CloseOutlined } from '@ant-design/icons';
import { harnessApi } from '../../api/harness';

interface ApprovalRule {
  id: string;
  name: string;
  type: string;
  enabled: boolean;
  description: string;
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
      setPending(res?.pending || res?.approvals || []);
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

  // 3-second polling for pending approvals
  useEffect(() => {
    const interval = setInterval(() => {
      loadPending();
    }, 3000);
    return () => clearInterval(interval);
  }, [loadPending]);

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
      model_change: '模型变更',
      config_change: '配置变更',
      deployment: '部署审批',
      cost_threshold: '成本阈值',
      custom: '自定义',
    };
    return map[t] || t;
  };

  const getTypeColor = (t: string) => {
    const map: Record<string, string> = {
      model_change: 'purple',
      config_change: 'blue',
      deployment: 'green',
      cost_threshold: 'orange',
      custom: 'default',
    };
    return map[t] || 'default';
  };

  const ruleColumns = [
    { title: '规则名称', dataIndex: 'name', key: 'name' },
    { title: '类型', dataIndex: 'type', key: 'type', render: (t: string) => <Tag color={getTypeColor(t)}>{getTypeLabel(t)}</Tag> },
    { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
    {
      title: '状态', dataIndex: 'enabled', key: 'enabled',
      render: (enabled: boolean, record: ApprovalRule) => (
        <Switch
          checked={enabled}
          onChange={async (checked) => {
            try {
              await harnessApi.approveRequest(record.id);
              message.success(checked ? '已启用' : '已禁用');
              loadRules();
            } catch {
              message.error('操作失败');
            }
          }}
          checkedChildren="启用"
          unCheckedChildren="禁用"
        />
      ),
    },
  ];

  const pendingColumns = [
    { title: '请求类型', dataIndex: 'request_type', key: 'request_type', render: (t: string) => <Tag color="blue">{t}</Tag> },
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
      <Alert message="审批管理 — 配置审批规则，处理待审批请求" type="info" showIcon style={{ marginBottom: 16 }} />

      <Row gutter={16}>
        <Col span={12}>
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
        <Col span={12}>
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
