import { useState, useEffect } from 'react';
import { Card, List, Tag, Button, Space, Modal, Form, Input, message } from 'antd';
import { ApiOutlined, PlusOutlined } from '@ant-design/icons';
import client from '../../api/client';
import { EmptyState } from '../../components/Common';

interface Agent {
  id: string;
  name: string;
  url: string;
  capabilities: string[];
  status: string;
}

export default function A2AManagement() {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [form] = Form.useForm();

  const loadAgents = async () => {
    setLoading(true);
    try {
      const res = await client.get('/api/v2/a2a/agents');
      setAgents((res as any)?.agents || []);
    } catch (error) {
      console.error('Load agents failed:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadAgents();
  }, []);

  const handleRegister = async (values: { id: string; name: string; url: string }) => {
    try {
      await client.post('/api/v2/a2a/agents', {
        ...values,
        capabilities: ['chat'],
        input_modes: ['text'],
        output_modes: ['text'],
      });
      message.success('注册成功');
      setModalOpen(false);
      form.resetFields();
      loadAgents();
    } catch (error) {
      message.error('注册失败');
    }
  };

  return (
    <div>
      <div style={{ marginBottom: 16 }}>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)}>
          注册 Agent
        </Button>
      </div>

      <List
        loading={loading}
        grid={{ gutter: 16, column: 3 }}
        dataSource={agents}
        renderItem={(agent) => (
          <List.Item>
            <Card
              title={
                <Space>
                  <ApiOutlined />
                  {agent.name}
                </Space>
              }
            >
              <p><strong>ID:</strong> {agent.id}</p>
              <p><strong>URL:</strong> {agent.url}</p>
              <div>
                <strong>能力:</strong>{' '}
                {agent.capabilities?.map((cap) => (
                  <Tag key={cap}>{cap}</Tag>
                ))}
              </div>
            </Card>
          </List.Item>
        )}
      />

      {agents.length === 0 && !loading && (
        <EmptyState description="暂无注册的 Agent" />
      )}

      <Modal
        title="注册 Agent"
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={() => form.submit()}
      >
        <Form form={form} layout="vertical" onFinish={handleRegister}>
          <Form.Item name="id" label="Agent ID" rules={[{ required: true }]}>
            <Input placeholder="agent-1" />
          </Form.Item>
          <Form.Item name="name" label="名称" rules={[{ required: true }]}>
            <Input placeholder="我的 Agent" />
          </Form.Item>
          <Form.Item name="url" label="URL" rules={[{ required: true }]}>
            <Input placeholder="http://localhost:8080" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}