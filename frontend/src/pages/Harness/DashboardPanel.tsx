import { useState, useEffect } from 'react';
import { Card, Row, Col, Statistic, Alert } from 'antd';
import { harnessApi } from '../../api/harness';

export default function DashboardPanel() {
  const [stats, setStats] = useState({
    totalRules: 0,
    activeRules: 0,
    runningABTests: 0,
    sloCompliance: 0,
  });
  const [sloLoading, setSloLoading] = useState(true);

  useEffect(() => {
    loadStats();
  }, []);

  const loadStats = async () => {
    let sloCompliance = 0;
    try {
      setSloLoading(true);
      const res = await harnessApi.getSLOStatus() as any;
      const statuses = res?.statuses || [];
      if (statuses.length > 0) {
        const totalBudget = statuses.reduce((sum: number, s: any) => sum + (s.budget_remaining || 0), 0);
        sloCompliance = (totalBudget / statuses.length) * 100;
      }
    } catch {
      sloCompliance = 0;
    } finally {
      setSloLoading(false);
    }

    let totalRules = 0;
    let activeRules = 0;
    try {
      const rulesRes = await harnessApi.listRules() as any;
      const rules = rulesRes?.rules || [];
      totalRules = rules.length;
      activeRules = rules.filter((r: any) => r.enabled).length;
    } catch {
      // Rules API not available
    }

    let runningABTests = 0;
    try {
      const abRes = await harnessApi.listABTests() as any;
      const tests = abRes?.tests || [];
      runningABTests = tests.filter((t: any) => t.status === 'running').length;
    } catch {
      // AB test API not available
    }

    setStats({
      totalRules,
      activeRules,
      runningABTests,
      sloCompliance,
    });
  };

  return (
    <div>
      <Row gutter={16}>
        <Col span={6}>
          <Card>
            <Statistic title="规则数量" value={stats.totalRules} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="活跃规则" value={stats.activeRules} valueStyle={{ color: '#3f8600' }} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="运行中 A/B 测试" value={stats.runningABTests} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="SLO 合规率" value={sloLoading ? 0 : stats.sloCompliance} suffix="%" precision={1} valueStyle={{ color: stats.sloCompliance >= 90 ? '#3f8600' : '#cf1322' }} loading={sloLoading} />
          </Card>
        </Col>
      </Row>

      <Card title="系统状态" style={{ marginTop: 24 }}>
        <Alert message="所有服务运行正常" type="success" showIcon />
      </Card>
    </div>
  );
}
