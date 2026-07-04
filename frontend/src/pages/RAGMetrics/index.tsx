import { Card, Typography } from 'antd';
import { DashboardOutlined } from '@ant-design/icons';

export default function RAGMetricsPage() {
  return (
    <div>
      <Typography.Title level={3}>
        <DashboardOutlined /> RAG Metrics Dashboard
      </Typography.Title>
      <Card>RAG Metrics content will be displayed here</Card>
    </div>
  );
}
