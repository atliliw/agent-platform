import { useState, useEffect } from 'react';
import { Table, Tag, Button, Space, Modal, Form, Input, Select, message } from 'antd';
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons';
import client from '../../api/client';
import { EmptyState } from '../../components/Common';

interface Rule {
  id: string;
  agent_id: string;
  name: string;
  type: string;
  config: Record<string, unknown>;
  enabled: boolean;
}

export default function RulesManagement() {
  const [rules, setRules] = useState<Rule[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [form] = Form.useForm();

  const loadRules = async () => {
    setLoading(true);
    try {
      const res = await client.get('/api/v2/harness/rules');
      setRules(res.data?.rules || []);
    } catch (error) {
      console.error('Load rules failed:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadRules();
  }, []);

  const handleCreate = async (values: Record<string, unknown>) => {
    try {
      await client.post('/api/v2/harness/rules', {
        ...values,
        agent_id: 'default',
        enabled: true,
      });
      message.success('创建成功');
      setModalOpen(false);
      form.resetFields();
      loadRules();
    } catch (error) {
      message.error('创建失败');
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await client.delete(`/api/v2/harness/rules/${id}`);
      message.success('删除成功');
      loadRules();
    } catch (error) {
      message.error('删除失败');
    }
  };

  const columns = [
    {
      title: '规则名称',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      render: (type: string) => <Tag color="blue">{type}</Tag>,
    },
    {
      title: 'Agent',
      dataIndex: 'agent_id',
      key: 'agent_id',
    },
    {
      title: '状态',
      dataIndex: 'enabled',
      key: 'enabled',
      render: (enabled: boolean) => (
        <Tag color={enabled ? 'green' : 'red'}>{enabled ? '启用' : '禁用'}</Tag>
      ),
    },
    {
      title: '操作',
      key: 'action',
      render: (_: unknown, record: Rule) => (
        <Space>
          <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDelete(record.id)}>
            删除
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div style={{ marginBottom: 16 }}>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)}>
          新建规则
        </Button>
      </div>

      <Table
        columns={columns}
        dataSource={rules}
        rowKey="id"
        loading={loading}
      />

      {rules.length === 0 && !loading && (
        <EmptyState description="暂无规则配置" />
      )}

      <Modal
        title="新建规则"
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={() => form.submit()}
      >
        <Form form={form} layout="vertical" onFinish={handleCreate}>
          <Form.Item name="name" label="规则名称" rules={[{ required: true }]}>
            <Input placeholder="token_limit" />
          </Form.Item>
          <Form.Item name="type" label="类型" rules={[{ required: true }]}>
            <Select>
              <Select.Option value="constraint">约束</Select.Option>
              <Select.Option value="permission">权限</Select.Option>
              <Select.Option value="budget">预算</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="config" label="配置 (JSON)">
            <Input.TextArea rows={4} defaultValue='{"max_tokens": 4000}' />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}