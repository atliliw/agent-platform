import { Layout, Menu, Button, theme } from 'antd';
import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import {
  MessageOutlined,
  BookOutlined,
  TeamOutlined,
  SettingOutlined,
  DashboardOutlined,
  DatabaseOutlined,
  MonitorOutlined,
  ExperimentOutlined,
  ApiOutlined,
  CodeOutlined,
  HistoryOutlined,
  BarChartOutlined,
  ApartmentOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';

const { Header, Sider, Content } = Layout;

const menuItems = [
  { key: '/chat', icon: <MessageOutlined />, label: '对话' },
  { key: '/knowledge', icon: <BookOutlined />, label: '知识库' },
  { key: '/memory', icon: <DatabaseOutlined />, label: '记忆' },
  { key: '/agents', icon: <TeamOutlined />, label: 'Agent' },
  { key: '/skills', icon: <ThunderboltOutlined />, label: 'skill' },
  { key: '/harness', icon: <DashboardOutlined />, label: '治理配置' },
  { key: '/session', icon: <HistoryOutlined />, label: 'Session回放' },
  { key: '/prompt', icon: <CodeOutlined />, label: 'Prompt管理' },
  { key: '/playground', icon: <ExperimentOutlined />, label: 'Playground' },
  { key: '/workflow', icon: <ApartmentOutlined />, label: 'Workflow' },
  { key: '/rag-metrics', icon: <BarChartOutlined />, label: 'RAG指标' },
  { key: '/gateway', icon: <ApiOutlined />, label: 'Gateway' },
  { key: '/observability', icon: <MonitorOutlined />, label: '运维观测' },
  { key: '/settings', icon: <SettingOutlined />, label: '设置' },
];

export default function MainLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const { token: { colorBgContainer, borderRadiusLG } } = theme.useToken();

  // Match menu item by prefix so /workflow/:id highlights /workflow
  const selectedKey = menuItems.find(item => location.pathname.startsWith(item.key))?.key ?? location.pathname;

  return (
    <Layout style={{ height: '100vh' }}>
      <Header style={{ display: 'flex', alignItems: 'center', background: colorBgContainer, padding: '0 24px', borderBottom: '1px solid #f0f0f0' }}>
        <div style={{ fontSize: '20px', fontWeight: 'bold', marginRight: '24px' }}>Agent Platform</div>
        <div style={{ flex: 1 }} />
        <Button type="link">文档</Button>
      </Header>
      <Layout>
        <Sider width={200} style={{ background: colorBgContainer }} theme="light">
          <Menu mode="inline" selectedKeys={[selectedKey]} style={{ height: '100%', borderRight: 0 }} items={menuItems} onClick={({ key }) => navigate(key)} />
        </Sider>
        <Content style={{ padding: 24, margin: 0, minHeight: 280, background: colorBgContainer, borderRadius: borderRadiusLG, overflow: 'auto' }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}
