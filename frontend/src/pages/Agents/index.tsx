import { Card, Tabs, Badge } from 'antd';
import A2AManagement from './A2A';
import MCPManagement from './MCP';
import AgentList from './AgentList';
import AgentEditor from './AgentEditor';
import { RobotOutlined, ToolOutlined, ApiOutlined, EditOutlined } from '@ant-design/icons';
import { useState, useEffect } from 'react';
import { agentApi } from '../../api/agent';

export default function AgentsPage() {
  const [agentCount, setAgentCount] = useState(0);

  useEffect(() => {
    agentApi.listAgents()
      .then((res: any) => setAgentCount(res?.agents?.length || 0))
      .catch(() => {});
  }, []);
  const items = [
    {
      key: 'agents',
      label: (
        <span>
          <RobotOutlined />
          多 Agent 编排
          <Badge count={agentCount} style={{ marginLeft: 8 }} />
        </span>
      ),
      children: <AgentList />,
    },
    {
      key: 'mcp',
      label: (
        <span>
          <ToolOutlined />
          MCP 工具
        </span>
      ),
      children: <MCPManagement />,
    },
    {
      key: 'a2a',
      label: (
        <span>
          <ApiOutlined />
          A2A Agent
        </span>
      ),
      children: <A2AManagement />,
    },
    {
      key: 'editor',
      label: (
        <span>
          <EditOutlined />
          Agent 编辑器
        </span>
      ),
      children: <AgentEditor />,
    },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 24 }}>
        <RobotOutlined style={{ marginRight: 8 }} />
        Agent 管理
      </h2>
      <Card>
        <Tabs items={items} />
      </Card>
    </div>
  );
}
