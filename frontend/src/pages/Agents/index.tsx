import { Card, Tabs, Badge } from 'antd';
import A2AManagement from './A2A';
import MCPManagement from './MCP';
import AgentList from './AgentList';
import { RobotOutlined, ToolOutlined, ApiOutlined } from '@ant-design/icons';

export default function AgentsPage() {
  const items = [
    {
      key: 'agents',
      label: (
        <span>
          <RobotOutlined />
          多 Agent 编排
          <Badge count={5} style={{ marginLeft: 8 }} />
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
