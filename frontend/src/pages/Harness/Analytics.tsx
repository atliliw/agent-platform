import { Card, Row, Col, Statistic, Progress } from 'antd';
import { ArrowUpOutlined } from '@ant-design/icons';

export default function AnalyticsDashboard() {
  return (
    <div>
      <Row gutter={[16, 16]}>
        <Col span={6}>
          <Card>
            <Statistic
              title="今日请求数"
              value={15234}
              valueStyle={{ color: '#3f8600' }}
              prefix={<ArrowUpOutlined />}
              suffix="次"
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="今日成本"
              value={128.5}
              precision={2}
              valueStyle={{ color: '#cf1322' }}
              prefix="$"
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="平均延迟"
              value={1.2}
              precision={1}
              suffix="秒"
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="成功率"
              value={98.5}
              precision={1}
              suffix="%"
            />
          </Card>
        </Col>
      </Row>

      <Card title="SLO 状态" style={{ marginTop: 24 }}>
        <div style={{ marginBottom: 16 }}>
          <div style={{ marginBottom: 8 }}>成功率</div>
          <Progress percent={98.5} status="active" />
        </div>
        <div style={{ marginBottom: 16 }}>
          <div style={{ marginBottom: 8 }}>P99 延迟</div>
          <Progress percent={40} format={() => '0.8s / 2s'} />
        </div>
        <div>
          <div style={{ marginBottom: 8 }}>质量评分</div>
          <Progress percent={85} format={() => '8.5 / 10'} />
        </div>
      </Card>

      <Card title="模型使用分布" style={{ marginTop: 24 }}>
        <Row gutter={16}>
          <Col span={12}>
            <Statistic title="qwen3.7-max" value={65} suffix="%" />
            <Progress percent={65} showInfo={false} />
          </Col>
          <Col span={12}>
            <Statistic title="其他模型" value={35} suffix="%" />
            <Progress percent={35} showInfo={false} />
          </Col>
        </Row>
      </Card>
    </div>
  );
}