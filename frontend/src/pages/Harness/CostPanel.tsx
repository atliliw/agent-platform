import { useState, useEffect } from 'react';
import {
  Card, Row, Col, Statistic, Alert, List, Tag, Space, Button
} from 'antd';
import { harnessApi } from '../../api/harness';

interface CostRecommendation {
  type: string;
  priority: string;
  title: string;
  description: string;
  potential_savings: number;
  potentialSavings?: number;
  agent_id: string;
}

export default function CostPanel() {
  const [recommendations, setRecommendations] = useState<CostRecommendation[]>([]);
  const [loading, setLoading] = useState(false);
  const [costStats, setCostStats] = useState({
    totalCost: 0,
    forecastCost: 0,
    totalRequests: 0,
    inputTokens: 0,
    outputTokens: 0,
  });

  useEffect(() => {
    loadCostData();
  }, []);

  const loadCostData = async () => {
    setLoading(true);
    try {
      const now = new Date();
      const startOfMonth = new Date(now.getFullYear(), now.getMonth(), 1);
      const reportRes = await harnessApi.getCostReport({
        start: startOfMonth.toISOString(),
        end: now.toISOString(),
      }) as any;

      if (reportRes && reportRes.report) {
        const report = reportRes.report;
        const totalCost = report.totalCost || report.total_cost || 0;
        const totalRequests = report.requestCount || report.request_count || 0;

        const daysInMonth = new Date(now.getFullYear(), now.getMonth() + 1, 0).getDate();
        const daysPassed = now.getDate();
        const forecastCost = daysPassed > 0 ? (totalCost / daysPassed) * daysInMonth : 0;

        setCostStats({
          totalCost,
          forecastCost,
          totalRequests,
          inputTokens: report.totalInputTokens || report.total_input_tokens || 0,
          outputTokens: report.totalOutputTokens || report.total_output_tokens || 0,
        });
      }
    } catch (error) {
      console.error('Failed to load cost report:', error);
    }

    try {
      const res = await harnessApi.getRecommendations() as any;
      setRecommendations(res?.recommendations || []);
    } catch (error) {
      console.error('Failed to load recommendations:', error);
      setRecommendations([]);
    } finally {
      setLoading(false);
    }
  };

  const getPriorityColor = (p: string) => p === 'high' ? 'red' : p === 'medium' ? 'orange' : 'blue';

  return (
    <div>
      <Alert message="Cost Intelligence 提供成本分析、闲置检测、优化建议等功能" type="info" showIcon style={{ marginBottom: 16 }} />

      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}>
          <Card>
            <Statistic title="本月成本" value={costStats.totalCost} prefix="¥" precision={2} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="预测成本" value={costStats.forecastCost} prefix="¥" precision={2} valueStyle={{ color: '#faad14' }} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="潜在节省" value={recommendations.reduce((a, r) => a + (r.potentialSavings || r.potential_savings || 0), 0)} prefix="¥" precision={2} valueStyle={{ color: '#52c41a' }} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="请求总数" value={costStats.totalRequests} />
          </Card>
        </Col>
      </Row>

      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}>
          <Card>
            <Statistic title="输入Tokens" value={costStats.inputTokens} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="输出Tokens" value={costStats.outputTokens} />
          </Card>
        </Col>
      </Row>

      <Card title="优化建议">
        <List
          loading={loading}
          dataSource={recommendations}
          renderItem={(item) => (
            <List.Item actions={[<Button type="link">查看详情</Button>]}>
              <List.Item.Meta
                title={<Space><Tag color={getPriorityColor(item.priority)}>{item.priority}</Tag>{item.title}</Space>}
                description={item.description}
              />
              <Statistic title="潜在节省" value={item.potentialSavings || item.potential_savings || 0} prefix="¥" precision={2} valueStyle={{ fontSize: 16, color: '#52c41a' }} />
            </List.Item>
          )}
        />
      </Card>
    </div>
  );
}
