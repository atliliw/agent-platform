import { Collapse, Tag, Timeline, Typography } from 'antd';
import {
  RobotOutlined,
  ToolOutlined,
  SwapOutlined,
  BulbOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
} from '@ant-design/icons';
import type { AgentExecutionRecord } from '../../api/agent';

const { Text, Paragraph } = Typography;

interface AgentTraceProps {
  agentHistory: AgentExecutionRecord[];
  collapsed?: boolean;
}

export default function AgentTrace({ agentHistory, collapsed = true }: AgentTraceProps) {
  if (!agentHistory || agentHistory.length === 0) {
    return null;
  }

  // 获取 Agent 颜色
  const getAgentColor = (agentId: string) => {
    const colors: Record<string, string> = {
      'main-agent': 'blue',
      'researcher-agent': 'green',
      'coder-agent': 'orange',
      'analyst-agent': 'purple',
      'browser-agent': 'cyan',
    };
    return colors[agentId] || 'default';
  };

  // 获取操作图标
  const getActionIcon = (action: string) => {
    if (action === 'handoff') return <SwapOutlined />;
    return <ToolOutlined />;
  };

  // 获取操作颜色
  const getActionColor = (action: string) => {
    if (action === 'handoff') return 'purple';
    return 'green';
  };

  return (
    <Collapse
      defaultActiveKey={collapsed ? [] : ['trace']}
      ghost
      items={[
        {
          key: 'trace',
          label: (
            <span>
              <RobotOutlined style={{ marginRight: 8 }} />
              Agent 执行轨迹
              <Tag color="blue" style={{ marginLeft: 8 }}>
                {agentHistory.length} 步
              </Tag>
            </span>
          ),
          children: (
            <Timeline
              items={agentHistory.map((record, index) => ({
                color: record.handoff_to ? 'purple' : 'green',
                dot: record.handoff_to ? (
                  <SwapOutlined style={{ fontSize: 16 }} />
                ) : (
                  <ToolOutlined style={{ fontSize: 16 }} />
                ),
                children: (
                  <div key={index} style={{ paddingBottom: 8 }}>
                    <div style={{ marginBottom: 8 }}>
                      <Tag color={getAgentColor(record.agent_id)} icon={<RobotOutlined />}>
                        {record.agent_name || record.agent_id}
                      </Tag>
                      <Tag color={getActionColor(record.action)} icon={getActionIcon(record.action)}>
                        {record.action}
                      </Tag>
                      {record.tokens_used > 0 && (
                        <Tag icon={<ClockCircleOutlined />}>
                          {record.tokens_used} tokens
                        </Tag>
                      )}
                    </div>

                    {/* 思考过程 */}
                    {record.thought && (
                      <div style={{ marginBottom: 8, padding: 8, background: '#f5f5f5', borderRadius: 4 }}>
                        <Text type="secondary">
                          <BulbOutlined style={{ marginRight: 4 }} />
                          思考:
                        </Text>
                        <Paragraph
                          style={{ margin: '4px 0 0 0', whiteSpace: 'pre-wrap' }}
                          ellipsis={{ rows: 2, expandable: true }}
                        >
                          {record.thought}
                        </Paragraph>
                      </div>
                    )}

                    {/* 参数 */}
                    {record.arguments && record.arguments !== '{}' && (
                      <div style={{ marginBottom: 8 }}>
                        <Text type="secondary">参数: </Text>
                        <Text code style={{ fontSize: 12 }}>
                          {record.arguments}
                        </Text>
                      </div>
                    )}

                    {/* 结果 */}
                    {record.result && (
                      <div style={{ marginBottom: 8 }}>
                        <Text type="secondary">
                          <CheckCircleOutlined style={{ marginRight: 4 }} />
                          结果:
                        </Text>
                        <Paragraph
                          style={{ margin: '4px 0 0 0', whiteSpace: 'pre-wrap' }}
                          ellipsis={{ rows: 3, expandable: true }}
                        >
                          {record.result}
                        </Paragraph>
                      </div>
                    )}

                    {/* Handoff 目标 */}
                    {record.handoff_to && (
                      <div>
                        <SwapOutlined style={{ marginRight: 4, color: '#722ed1' }} />
                        <Text strong style={{ color: '#722ed1' }}>
                          交接给 → {record.handoff_to}
                        </Text>
                      </div>
                    )}
                  </div>
                ),
              }))}
            />
          ),
        },
      ]}
    />
  );
}
