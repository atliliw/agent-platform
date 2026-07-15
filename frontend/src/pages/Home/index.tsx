import { Row, Col, Card, Statistic } from 'antd';
import { MessageOutlined, BookOutlined, TeamOutlined, DashboardOutlined } from '@ant-design/icons';

export default function HomePage() {
  return (
    <div>
      <h2 style={{ marginBottom: 24 }}>欢迎使用 Agent Platform</h2>

      <Row gutter={[16, 16]}>
        <Col span={6}>
          <Card>
            <Statistic
              title="今日对话"
              value={1234}
              prefix={<MessageOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="知识库文档"
              value={56}
              prefix={<BookOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="活跃 Agent"
              value={8}
              prefix={<TeamOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="系统健康度"
              value={99.9}
              suffix="%"
              prefix={<DashboardOutlined />}
            />
          </Card>
        </Col>
      </Row>

      <Card style={{ marginTop: 24 }}>
        <h3>快速开始</h3>
        <p>这是一个 AI Agent 平台，提供以下功能：</p>
        <ul>
          <li>💬 <strong>智能对话</strong> - 支持多轮对话、工具调用、多 Agent 协作</li>
          <li>📚 <strong>知识库</strong> - 上传文档，智能检索</li>
          <li>🧠 <strong>记忆管理</strong> - 长期记忆、语义召回</li>
          <li>🤝 <strong>A2A 协议</strong> - 跨服务 Agent 通信</li>
          <li>⚙️ <strong>运维治理</strong> - 规则引擎、护栏、评估、A/B 测试</li>
        </ul>
      </Card>
    </div>
  );
}