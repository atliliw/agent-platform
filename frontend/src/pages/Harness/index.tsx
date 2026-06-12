import { Card, Tabs } from 'antd';
import RulesManagement from './Rules';
import AnalyticsDashboard from './Analytics';

export default function HarnessPage() {
  const items = [
    {
      key: 'rules',
      label: '规则引擎',
      children: <RulesManagement />,
    },
    {
      key: 'analytics',
      label: '数据分析',
      children: <AnalyticsDashboard />,
    },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 24 }}>运维治理</h2>
      <Card>
        <Tabs items={items} />
      </Card>
    </div>
  );
}