import { useState, useEffect } from "react";
import { Card, Row, Col, Statistic, Empty, Spin, Table, Tag } from "antd";
import { CheckCircleOutlined, ClockCircleOutlined, DollarOutlined } from "@ant-design/icons";
import { gatewayApi } from "../../api/gateway";

export default function GatewayStats() {
  const [stats, setStats] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    const load = async () => {
      setLoading(true);
      try {
        const res = await gatewayApi.getGatewayStats() as any;
        setStats(res?.stats || []);
      } catch {
        setStats([]);
      } finally { setLoading(false); }
    };
    load();
  }, []);

  if (loading) return <Spin />;
  if (!stats.length) return <Empty description="No gateway stats available" />;

  const totalRequests = stats.reduce((sum: number, s: any) => sum + (s.total_requests || 0), 0);
  const totalSuccess = stats.reduce((sum: number, s: any) => sum + (s.success_count || 0), 0);
  const avgLatency = stats.length > 0 ? stats.reduce((sum: number, s: any) => sum + (s.avg_latency || 0), 0) / stats.length : 0;
  const totalCost = stats.reduce((sum: number, s: any) => sum + (s.total_cost || 0), 0);

  const columns = [
    { title: 'Provider', dataIndex: 'provider', key: 'provider', render: (p: string) => <Tag color="blue">{p}</Tag> },
    { title: 'Requests', dataIndex: 'total_requests', key: 'total_requests' },
    { title: 'Success', dataIndex: 'success_count', key: 'success_count' },
    { title: 'Errors', dataIndex: 'error_count', key: 'error_count', render: (e: number) => <span style={{ color: e > 0 ? '#ff4d4f' : '#52c41a' }}>{e}</span> },
    { title: 'Avg Latency', dataIndex: 'avg_latency', key: 'avg_latency', render: (l: number) => `${Math.round(l)}ms` },
    { title: 'Total Cost', dataIndex: 'total_cost', key: 'total_cost', render: (c: number) => `$${(c || 0).toFixed(4)}` },
  ];

  return (
    <div>
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}><Card><Statistic title="Total Requests" value={totalRequests} prefix={<CheckCircleOutlined />} /></Card></Col>
        <Col span={6}><Card><Statistic title="Success Rate" value={totalRequests > 0 ? Math.round(totalSuccess / totalRequests * 100) : 0} suffix="%" /></Card></Col>
        <Col span={6}><Card><Statistic title="Avg Latency" value={Math.round(avgLatency)} suffix="ms" prefix={<ClockCircleOutlined />} /></Card></Col>
        <Col span={6}><Card><Statistic title="Total Cost" value={totalCost} prefix={<DollarOutlined />} precision={4} /></Card></Col>
      </Row>
      <Card title="By Provider">
        <Table columns={columns} dataSource={stats} rowKey="provider" pagination={false} />
      </Card>
    </div>
  );
}
