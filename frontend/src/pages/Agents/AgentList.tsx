import { useState, useEffect } from 'react';
import { Table, Tag, Button, Space, Modal, Descriptions } from 'antd';
import { RobotOutlined, ToolOutlined, SwapOutlined, InfoCircleOutlined } from '@ant-design/icons';
import { agentApi, type Agent } from '../../api/agent';

export default function AgentList() {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedAgent, setSelectedAgent] = useState<Agent | null>(null);

  // 加载 Agent 列表
  const loadAgents = async () => {
    setLoading(true);
    try {
      const res = await agentApi.listAgents();
      setAgents(res.agents);
    } catch (error) {
      console.error('Failed to load agents:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadAgents();
  }, []);

  // 表格列定义
  const columns = [
    {
      title: 'Agent ID',
      dataIndex: 'id',
      key: 'id',
      render: (id: string) => (
        <Tag color="blue" icon={<RobotOutlined />}>
          {id}
        </Tag>
      ),
    },
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      render: (name: string) => <strong>{name}</strong>,
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: '工具',
      dataIndex: 'tools',
      key: 'tools',
      render: (tools: string[]) => (
        <Space size={[0, 4]} wrap>
          {tools?.map((tool) => (
            <Tag key={tool} color="green" icon={<ToolOutlined />}>
              {tool}
            </Tag>
          ))}
          {(!tools || tools.length === 0) && <Tag>无工具</Tag>}
        </Space>
      ),
    },
    {
      title: '可交接',
      dataIndex: 'handoffs',
      key: 'handoffs',
      render: (handoffs: string[]) => (
        <Space size={[0, 4]} wrap>
          {handoffs?.map((agentId) => (
            <Tag key={agentId} color="purple" icon={<SwapOutlined />}>
              {agentId}
            </Tag>
          ))}
          {(!handoffs || handoffs.length === 0) && <Tag>无</Tag>}
        </Space>
      ),
    },
    {
      title: '模型',
      dataIndex: 'model',
      key: 'model',
      render: (model: string) => model || '默认',
    },
    {
      title: '操作',
      key: 'action',
      render: (_: unknown, record: Agent) => (
        <Button
          type="link"
          icon={<InfoCircleOutlined />}
          onClick={() => setSelectedAgent(record)}
        >
          详情
        </Button>
      ),
    },
  ];

  return (
    <div>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h3 style={{ margin: 0 }}>
          <RobotOutlined style={{ marginRight: 8 }} />
          已注册 Agent ({agents.length})
        </h3>
        <Button onClick={loadAgents} loading={loading}>
          刷新
        </Button>
      </div>

      <Table
        columns={columns}
        dataSource={agents}
        rowKey="id"
        loading={loading}
        pagination={false}
      />

      {/* Agent 详情弹窗 */}
      <Modal
        title={
          <span>
            <RobotOutlined style={{ marginRight: 8 }} />
            {selectedAgent?.name}
          </span>
        }
        open={!!selectedAgent}
        onCancel={() => setSelectedAgent(null)}
        footer={null}
        width={700}
      >
        {selectedAgent && (
          <Descriptions bordered column={1}>
            <Descriptions.Item label="Agent ID">
              <Tag color="blue">{selectedAgent.id}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label="描述">{selectedAgent.description}</Descriptions.Item>
            <Descriptions.Item label="可用工具">
              <Space size={[0, 4]} wrap>
                {selectedAgent.tools?.map((tool) => (
                  <Tag key={tool} color="green">
                    {tool}
                  </Tag>
                ))}
                {(!selectedAgent.tools || selectedAgent.tools.length === 0) && <span>无</span>}
              </Space>
            </Descriptions.Item>
            <Descriptions.Item label="可交接 Agent">
              <Space size={[0, 4]} wrap>
                {selectedAgent.handoffs?.map((agentId) => (
                  <Tag key={agentId} color="purple">
                    {agentId}
                  </Tag>
                ))}
                {(!selectedAgent.handoffs || selectedAgent.handoffs.length === 0) && <span>无</span>}
              </Space>
            </Descriptions.Item>
            <Descriptions.Item label="模型">{selectedAgent.model || '默认'}</Descriptions.Item>
            <Descriptions.Item label="Max Tokens">{selectedAgent.max_tokens}</Descriptions.Item>
            <Descriptions.Item label="Temperature">{selectedAgent.temperature}</Descriptions.Item>
            <Descriptions.Item label="系统指令">
              <div style={{ maxHeight: 300, overflow: 'auto', whiteSpace: 'pre-wrap' }}>
                {selectedAgent.instructions}
              </div>
            </Descriptions.Item>
          </Descriptions>
        )}
      </Modal>
    </div>
  );
}
