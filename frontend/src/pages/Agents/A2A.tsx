import { useState, useEffect } from 'react';
import { Card, List, Tag, Button, Space, Modal, Form, Input, Select, message, Descriptions } from 'antd';
import { ApiOutlined, PlusOutlined, GlobalOutlined } from '@ant-design/icons';
import client from '../../api/client';
import { EmptyState } from '../../components/Common';

interface A2AAgent {
  id: string;
  name: string;
  description: string;
  url: string;
  capabilities: string[];
  input_modes: string[];
  output_modes: string[];
  status?: string;
}

const CAPABILITY_COLORS: Record<string, string> = {
  research: 'blue',
  web_search: 'cyan',
  report_generation: 'geekblue',
  code_generation: 'green',
  bash_execution: 'lime',
  web_browsing: 'orange',
  file_editing: 'gold',
  planning: 'purple',
  file_operations: 'volcano',
  code_execution: 'magenta',
  collaboration: 'pink',
  task_delegation: 'rose',
  role_based: 'purple',
  form_filling: 'blue',
  data_extraction: 'cyan',
  screenshot: 'geekblue',
  chat: 'green',
  search: 'cyan',
  multi_agent: 'purple',
  tool_calling: 'orange',
};

export default function A2AManagement() {
  const [agents, setAgents] = useState<A2AAgent[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [selectedAgent, setSelectedAgent] = useState<A2AAgent | null>(null);
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

  const handleRegister = async (values: { id: string; name: string; url: string; description: string; capabilities: string[] }) => {
    try {
      await client.post('/api/v2/a2a/agents', {
        card: {
          id: values.id,
          name: values.name,
          description: values.description,
          url: values.url,
          capabilities: values.capabilities || ['chat'],
          input_modes: ['text'],
          output_modes: ['text'],
        },
        tenant_id: 'default',
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
              hoverable
              title={
                <Space>
                  <ApiOutlined />
                  {agent.name}
                </Space>
              }
              extra={
                <Button type="link" size="small" onClick={() => setSelectedAgent(agent)}>
                  详情
                </Button>
              }
            >
              <p style={{ color: '#666', marginBottom: 8 }}>{agent.description}</p>
              <div style={{ marginBottom: 4 }}>
                <GlobalOutlined style={{ marginRight: 4, color: '#999' }} />
                <span style={{ fontSize: 12, color: '#999' }}>{agent.url}</span>
              </div>
              <div>
                {agent.capabilities?.map((cap) => (
                  <Tag key={cap} color={CAPABILITY_COLORS[cap] || 'default'} style={{ marginBottom: 4 }}>
                    {cap}
                  </Tag>
                ))}
              </div>
            </Card>
          </List.Item>
        )}
      />

      {agents.length === 0 && !loading && (
        <EmptyState description="暂无注册的 Agent" />
      )}

      {/* Agent 详情弹窗 */}
      <Modal
        title={
          <span>
            <ApiOutlined style={{ marginRight: 8 }} />
            {selectedAgent?.name}
          </span>
        }
        open={!!selectedAgent}
        onCancel={() => setSelectedAgent(null)}
        footer={null}
        width={600}
      >
        {selectedAgent && (
          <Descriptions bordered column={1} size="small">
            <Descriptions.Item label="Agent ID">
              <Tag color="blue">{selectedAgent.id}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label="描述">{selectedAgent.description}</Descriptions.Item>
            <Descriptions.Item label="URL">
              <span style={{ fontFamily: 'monospace' }}>{selectedAgent.url}</span>
            </Descriptions.Item>
            <Descriptions.Item label="能力">
              <Space size={[0, 4]} wrap>
                {selectedAgent.capabilities?.map((cap) => (
                  <Tag key={cap} color={CAPABILITY_COLORS[cap] || 'default'}>{cap}</Tag>
                ))}
              </Space>
            </Descriptions.Item>
            <Descriptions.Item label="输入模式">
              <Space>
                {selectedAgent.input_modes?.map((m) => <Tag key={m}>{m}</Tag>)}
              </Space>
            </Descriptions.Item>
            <Descriptions.Item label="输出模式">
              <Space>
                {selectedAgent.output_modes?.map((m) => <Tag key={m}>{m}</Tag>)}
              </Space>
            </Descriptions.Item>
          </Descriptions>
        )}
      </Modal>

      {/* 注册 Agent 弹窗 */}
      <Modal
        title="注册 Agent"
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={() => form.submit()}
      >
        <Form form={form} layout="vertical" onFinish={handleRegister}>
          <Form.Item name="id" label="Agent ID" rules={[{ required: true }]}>
            <Input placeholder="my-agent" />
          </Form.Item>
          <Form.Item name="name" label="名称" rules={[{ required: true }]}>
            <Input placeholder="我的 Agent" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea placeholder="Agent 功能描述" rows={2} />
          </Form.Item>
          <Form.Item name="url" label="URL" rules={[{ required: true }]}>
            <Input placeholder="http://localhost:8080" />
          </Form.Item>
          <Form.Item name="capabilities" label="能力">
            <Select
              mode="tags"
              placeholder="输入能力标签"
              options={[
                { value: 'chat', label: 'chat - 对话' },
                { value: 'web_search', label: 'web_search - 网页搜索' },
                { value: 'code_generation', label: 'code_generation - 代码生成' },
                { value: 'research', label: 'research - 研究' },
                { value: 'web_browsing', label: 'web_browsing - 网页浏览' },
                { value: 'multi_agent', label: 'multi_agent - 多 Agent' },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
