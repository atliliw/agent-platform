import { useState, useEffect } from 'react';
import {
  Table, Tag, Button, Space, Switch, Input, Select,
  Modal, Form, Popconfirm, Card, message, Alert
} from 'antd';
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons';
import { harnessApi } from '../../api/harness';

interface FeatureFlag {
  id: string;
  key: string;
  name: string;
  description: string;
  type: string;
  value: string;
  status: string;
  rollout: number;
  enabled: boolean;
  created_at: number;
}

export default function FeatureFlagsPanel() {
  const [flags, setFlags] = useState<FeatureFlag[]>([]);
  const [loading, setLoading] = useState(false);
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [evaluateModalOpen, setEvaluateModalOpen] = useState(false);
  const [evaluateResult, setEvaluateResult] = useState<any>(null);
  const [evaluateLoading, setEvaluateLoading] = useState(false);
  const [createForm] = Form.useForm();
  const [evaluateForm] = Form.useForm();

  useEffect(() => {
    loadFlags();
  }, []);

  const loadFlags = async () => {
    setLoading(true);
    try {
      const res = await harnessApi.listFlags() as any;
      setFlags(res?.flags || []);
    } catch {
      setFlags([]);
    } finally {
      setLoading(false);
    }
  };

  const handleToggle = async (key: string, enabled: boolean) => {
    try {
      await harnessApi.toggleFlag(key, enabled);
      message.success(enabled ? '已启用' : '已禁用');
      loadFlags();
    } catch {
      message.error('操作失败');
    }
  };

  const handleCreate = async () => {
    try {
      const values = await createForm.validateFields();
      await harnessApi.createFlag({
        key: values.key,
        name: values.name,
        description: values.description,
        type: values.type || 'boolean',
        value: values.value || 'true',
      });
      message.success('Feature Flag 创建成功');
      setCreateModalOpen(false);
      createForm.resetFields();
      loadFlags();
    } catch {
      message.error('创建失败');
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await harnessApi.deleteFlag(id);
      message.success('删除成功');
      loadFlags();
    } catch {
      message.error('删除失败');
    }
  };

  const handleEvaluate = async () => {
    try {
      const values = await evaluateForm.validateFields();
      setEvaluateLoading(true);
      setEvaluateResult(null);
      const context = values.context ? JSON.parse(values.context) : {};
      const res = await harnessApi.evaluateFlag(values.flag_key, context) as any;
      setEvaluateResult(res?.result || res);
    } catch (error) {
      if (error instanceof SyntaxError) {
        message.error('Context 必须是有效的 JSON');
      } else {
        message.error('评估失败');
      }
    } finally {
      setEvaluateLoading(false);
    }
  };

  const columns = [
    { title: 'Key', dataIndex: 'key', key: 'key', render: (v: string) => <Tag color="blue">{v}</Tag> },
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
    { title: '类型', dataIndex: 'type', key: 'type', width: 80, render: (t: string) => <Tag>{t}</Tag> },
    { title: '值', dataIndex: 'value', key: 'value', width: 100 },
    {
      title: '状态', key: 'status', width: 100,
      render: (_: any, record: FeatureFlag) => (
        <Switch
          checked={record.status === 'active'}
          onChange={(checked) => handleToggle(record.key, checked)}
          checkedChildren="启用"
          unCheckedChildren="禁用"
        />
      ),
    },
    { title: '灰度%', dataIndex: 'rollout', key: 'rollout', width: 80, render: (v: number) => `${v}%` },
    {
      title: '操作', key: 'action', width: 80,
      render: (_: any, record: FeatureFlag) => (
        <Popconfirm
          title="确定删除此 Feature Flag？"
          onConfirm={() => handleDelete(record.id)}
          okText="确定"
          cancelText="取消"
        >
          <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
        </Popconfirm>
      ),
    },
  ];

  return (
    <div>
      <Alert message="Feature Flags — 动态控制功能开关，支持条件评估和灰度发布" type="info" showIcon style={{ marginBottom: 16 }} />

      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Space>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateModalOpen(true)}>
            创建 Flag
          </Button>
          <Button onClick={() => setEvaluateModalOpen(true)}>
            评估 Flag
          </Button>
        </Space>
      </div>

      <Table columns={columns} dataSource={flags} rowKey="id" loading={loading} />

      {/* 创建 Flag Modal */}
      <Modal
        title="创建 Feature Flag"
        open={createModalOpen}
        onOk={handleCreate}
        onCancel={() => { setCreateModalOpen(false); createForm.resetFields(); }}
        width={600}
      >
        <Form form={createForm} layout="vertical">
          <Form.Item name="key" label="Flag Key" rules={[{ required: true, message: '请输入 Key' }]}>
            <Input placeholder="例如：enable_new_model" />
          </Form.Item>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="例如：启用新模型" />
          </Form.Item>
          <Form.Item name="description" label="描述" rules={[{ required: true, message: '请输入描述' }]}>
            <Input.TextArea rows={2} placeholder="描述此 Flag 控制的功能" />
          </Form.Item>
          <div style={{ display: 'flex', gap: 16 }}>
            <Form.Item name="type" label="类型" initialValue="boolean" style={{ flex: 1 }}>
              <Select>
                <Select.Option value="boolean">布尔</Select.Option>
                <Select.Option value="string">字符串</Select.Option>
                <Select.Option value="number">数字</Select.Option>
              </Select>
            </Form.Item>
            <Form.Item name="value" label="默认值" initialValue="true" style={{ flex: 1 }}>
              <Input placeholder="true / false / 数值" />
            </Form.Item>
          </div>
        </Form>
      </Modal>

      {/* 评估 Flag Modal */}
      <Modal
        title="评估 Feature Flag"
        open={evaluateModalOpen}
        onOk={handleEvaluate}
        onCancel={() => { setEvaluateModalOpen(false); evaluateForm.resetFields(); setEvaluateResult(null); }}
        width={600}
      >
        <Form form={evaluateForm} layout="vertical">
          <Form.Item name="flag_key" label="选择 Flag" rules={[{ required: true, message: '请选择 Flag' }]}>
            <Select placeholder="选择要评估的 Flag">
              {flags.map((f) => (
                <Select.Option key={f.key} value={f.key}>{f.key} ({f.name})</Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item name="context" label="上下文 (JSON)">
            <Input.TextArea rows={3} placeholder='{"user_id": "123", "region": "cn"}' />
          </Form.Item>
        </Form>
        {evaluateLoading && <div style={{ textAlign: 'center', padding: 16 }}>评估中...</div>}
        {evaluateResult !== null && (
          <Card size="small" title="评估结果" style={{ marginTop: 16 }}>
            <pre style={{ margin: 0, fontSize: 12 }}>{JSON.stringify(evaluateResult, null, 2)}</pre>
          </Card>
        )}
      </Modal>
    </div>
  );
}
