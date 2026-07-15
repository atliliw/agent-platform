import { useState } from 'react';
import { Card, Tabs } from 'antd';
import {
  DashboardOutlined, SettingOutlined, ExperimentOutlined,
  RocketOutlined, SafetyCertificateOutlined, FlagOutlined, ScheduleOutlined
} from '@ant-design/icons';

import DashboardPanel from './DashboardPanel';
import RulesPanel from './RulesPanel';
import ABTestPanel from './ABTestPanel';
import ProposalsPanel from './ProposalsPanel';
import ApprovalPanel from './ApprovalPanel';
import FeatureFlagsPanel from './FeatureFlagsPanel';
import SchedulerPanel from './SchedulerPanel';

export default function HarnessPage() {
  const [activeTab, setActiveTab] = useState('dashboard');

  return (
    <div>
      <h2 style={{ marginBottom: 24 }}>治理配置</h2>
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
