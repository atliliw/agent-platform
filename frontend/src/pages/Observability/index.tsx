import { Card, Tabs } from 'antd';
import TraceViewer from './TraceViewer';
import CostDashboard from './CostDashboard';
import EvalReport from './EvalReport';
import MemoryManager from './MemoryManager';
import AgentEditor from './AgentEditor';

export default function ObservabilityPage() {
  const items = [
    {
      key: 'traces',
      label: '执行追踪',
      children: <TraceViewer />,
    },
    {
      key: 'cost',
      label: '成本监控',
      children: <CostDashboard />,
    },
    {
      key: 'eval',
      label: '评测报告',
      children: <EvalReport />,
    },
    {
      key: 'memory',
      label: '记忆管理',
      children: <MemoryManager />,
    },
    {
      key: 'agent-editor',
      label: 'Agent 编辑器',
      children: <AgentEditor />,
    },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 24 }}>可观测性与管理</h2>
      <Card>
        <Tabs items={items} />
      </Card>
    </div>
  );
}

export { TraceViewer, CostDashboard, EvalReport, MemoryManager, AgentEditor };
