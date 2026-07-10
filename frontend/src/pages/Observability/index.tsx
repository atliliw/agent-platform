import { Card, Tabs } from 'antd';
import OverviewPanel from './OverviewPanel';
import TraceViewer from './TraceViewer';
import CostDashboard from './CostDashboard';
import EvalReport from './EvalReport';
import SLOPanel from './SLOPanel';

export default function ObservabilityPage() {
  const items = [
    {
      key: 'overview',
      label: '概览',
      children: <OverviewPanel />,
    },
    {
      key: 'cost',
      label: '成本',
      children: <CostDashboard />,
    },
    {
      key: 'eval',
      label: '评测',
      children: <EvalReport />,
    },
    {
      key: 'slo',
      label: 'SLO 与指标',
      children: <SLOPanel />,
    },
    {
      key: 'traces',
      label: '执行追踪',
      children: <TraceViewer />,
    },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 24 }}>运维观测中心</h2>
      <Card>
        <Tabs items={items} />
      </Card>
    </div>
  );
}
