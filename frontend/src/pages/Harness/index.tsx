import { useState } from 'react';
import { Card, Tabs } from 'antd';
import {
  DashboardOutlined, SettingOutlined, ExperimentOutlined,
  ThunderboltOutlined, DollarOutlined, RocketOutlined,
  SafetyCertificateOutlined, FlagOutlined, ScheduleOutlined
} from '@ant-design/icons';

import DashboardPanel from './DashboardPanel';
import RulesPanel from './RulesPanel';
import ABTestPanel from './ABTestPanel';
import SLOPanel from './SLOPanel';
import CostPanel from './CostPanel';
import ProposalsPanel from './ProposalsPanel';
import ApprovalPanel from './ApprovalPanel';
import FeatureFlagsPanel from './FeatureFlagsPanel';
import SchedulerPanel from './SchedulerPanel';

export default function HarnessPage() {
  const [activeTab, setActiveTab] = useState('dashboard');

  return (
    <div>
      <h2 style={{ marginBottom: 24 }}>运维治理中心</h2>
      <Card>
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          items={[
            {
              key: 'dashboard',
              label: <span><DashboardOutlined /> 概览</span>,
              children: <DashboardPanel />,
            },
            {
              key: 'rules',
              label: <span><SettingOutlined /> 规则引擎</span>,
              children: <RulesPanel />,
            },
            {
              key: 'abtest',
              label: <span><ExperimentOutlined /> A/B 测试</span>,
              children: <ABTestPanel />,
            },
            {
              key: 'slo',
              label: <span><ThunderboltOutlined /> SLO 监控</span>,
              children: <SLOPanel />,
            },
            {
              key: 'cost',
              label: <span><DollarOutlined /> Cost</span>,
              children: <CostPanel />,
            },
            {
              key: 'proposals',
              label: <span><RocketOutlined /> Proposals</span>,
              children: <ProposalsPanel />,
            },
            {
              key: 'approval',
              label: <span><SafetyCertificateOutlined /> 审批管理</span>,
              children: <ApprovalPanel />,
            },
            {
              key: 'flags',
              label: <span><FlagOutlined /> Feature Flags</span>,
              children: <FeatureFlagsPanel />,
            },
            {
              key: 'scheduler',
              label: <span><ScheduleOutlined /> 调度器</span>,
              children: <SchedulerPanel />,
            },
          ]}
        />
      </Card>
    </div>
  );
}
