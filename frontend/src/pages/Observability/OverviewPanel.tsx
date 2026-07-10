import { useState, useEffect } from 'react';
import { Card, Row, Col, Statistic, Spin, Alert } from 'antd';
import {
  DollarOutlined,
  ApiOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
} from '@ant-design/icons';
import { harnessApi } from '../../api/harness';

interface OverviewData {
  totalCost: number;
  totalRequests: number;
  totalTokens: number;
  llmSuccessRate: number;
  llmAvgLatency: number;
  llmTotalCalls: number;
  sloMet: number;
  sloTotal: number;
}

const EMPTY: OverviewData = {
  totalCost: 0,
  totalRequests: 0,
  totalTokens: 0,
  llmSuccessRate: 0,
  llmAvgLatency: 0,
  llmTotalCalls: 0,
  sloMet: 0,
  sloTotal: 0,
};

export default function OverviewPanel() {
  const [data, setData] = useState<OverviewData>(EMPTY);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    loadOverview();
  }, []);

  const loadOverview = async () => {
    setLoading(true);
    const now = new Date();
    const startOfMonth = new Date(now.getFullYear(), now.getMonth(), 1);

    const [costRes, llmRes, sloRes] = await Promise.allSettled([
      harnessApi.getCostReport({ start: startOfMonth.toISOString(), end: now.toISOString() }),
      harnessApi.getLLMMetrics(),
      harnessApi.getSLOStatus(),
    ]);

    const next: OverviewData = { ...EMPTY };

    if (costRes.status === 'fulfilled') {
      const r = (costRes.value as any)?.report || {};
      next.totalCost = r.totalCost || r.total_cost || 0;
      next.totalRequests = r.requestCount || r.request_count || 0;
      next.totalTokens =
        (r.totalInputTokens || r.total_input_tokens || 0) +
        (r.totalOutputTokens || r.total_output_tokens || 0);
    }
    if (llmRes.status === 'fulfilled') {
      const m = (llmRes.value as any)?.data || (llmRes.value as any) || {};
      next.llmTotalCalls = m.total_calls || 0;
      next.llmSuccessRate = m.success_rate || 0;
      next.llmAvgLatency = m.avg_latency || 0;
    }
    if (sloRes.status === 'fulfilled') {
      const slos = (sloRes.value as any)?.statuses || [];
      next.sloTotal = slos.length;
      next.sloMet = slos.filter((s: any) => {
        if (String(s.name).toLowerCase().includes('latency') || s.name.includes('延迟')) {
          return s.current <= s.target * 1000;
        }
        return s.current >= s.target;
      }).length;
    }

    setData(next);
    setLoading(false);
  };

  return (
    <Spin spinning={loading}>
      <Alert
        message="运维观测中心概览：聚合本月成本、调用、LLM 指标与 SLO 达标情况"
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
      />

      <Row gutter={16}>
        <Col span={6}>
          <Card>
            <Statistic title="本月成本" value={data.totalCost} prefix={<DollarOutlined />} suffix="¥" precision={2} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="请求总数" value={data.totalRequests} prefix={<ApiOutlined />} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="Token 用量" value={data.totalTokens} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="LLM 调用次数" value={data.llmTotalCalls} />
          </Card>
        </Col>
      </Row>

      <Row gutter={16} style={{ marginTop: 16 }}>
        <Col span={8}>
          <Card>
            <Statistic
              title="LLM 成功率"
              value={data.llmSuccessRate * 100}
              suffix="%"
              precision={1}
              prefix={<CheckCircleOutlined />}
              valueStyle={{ color: data.llmSuccessRate >= 0.95 ? '#3f8600' : '#cf1322' }}
            />
          </Card>
        </Col>
        <Col span={8}>
          <Card>
            <Statistic
              title="平均延迟"
              value={data.llmAvgLatency}
              suffix="ms"
              precision={0}
              prefix={<ClockCircleOutlined />}
            />
          </Card>
        </Col>
        <Col span={8}>
          <Card>
            <Statistic
              title="SLO 达标"
              value={data.sloMet}
              suffix={` / ${data.sloTotal}`}
              valueStyle={{
                color: data.sloTotal === 0 || data.sloMet === data.sloTotal ? '#3f8600' : '#faad14',
              }}
            />
          </Card>
        </Col>
      </Row>
    </Spin>
  );
}
