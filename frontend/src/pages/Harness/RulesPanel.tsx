import { useState, useEffect } from 'react';
import {
  Table, Tag, Button, Space, Badge, Input, Select, Switch,
  Modal, Form, message, Alert
} from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { harnessApi } from '../../api/harness';

export default function RulesPanel() {
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
      const res = await harnessApi.listRules() as any;
      setRules(res?.rules || []);
    } catch (error) {
      console.error('Failed to load rules:', error);
      setRules([]);
    } finally {
      setLoading(false);
    }
  };

  const createRule = async (values: any) => {
    try {
      await harnessApi.createRule({
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
      await harnessApi.deleteRule(id);
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
