import { useState, useEffect } from 'react';
import {
  Table, Tag, Button, Badge, Input, Select, InputNumber,
  Modal, Form, Progress, Row, Col, Card, Statistic, message, Alert
} from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { harnessApi } from '../../api/harness';

interface SLOStatusDetail {
  name: string;
  current: number;
  target: number;
  budget_remaining: number;
  status: string;
}

export default function SLOPanel() {
  const [slos, setSlos] = useState<SLOStatusDetail[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [form] = Form.useForm();
  const [llmSummary, setLLMSummary] = useState({ totalCalls: 0, successRate: 0, avgLatency: 0 });

  useEffect(() => {
    loadSLOs();
  }, []);

  const loadSLOs = async () => {
    setLoading(true);
    try {
      const res = await harnessApi.getSLOStatus() as any;
      setSlos(res?.statuses || []);
    } catch {
      setSlos([]);
    }

    try {
      const llmRes = await harnessApi.getLLMMetrics() as any;
      const data = llmRes?.data || llmRes;
      setLLMSummary({
        totalCalls: data?.total_calls || 0,
        successRate: data?.success_rate || 0,
        avgLatency: data?.avg_latency || 0,
      });
    } catch {
      // LLM metrics endpoint may not be available yet
    } finally {
      setLoading(false);
    }
  };

  const createSLO = async (values: { name: string; agent_id: string; type: string; target: number }) => {
    try {
      await harnessApi.createSLO({
        agent_id: values.agent_id || '',
        name: values.name,
        target: values.target / 100,
        type: values.type,
      });
      message.success('SLO 创建成功');
      setModalOpen(false);
      form.resetFields();
      loadSLOs();
    } catch {
      message.error('SLO 创建失败');
    }
  };

  const getStatusBadge = (status: string) => {
    const map: Record<string, 'success' | 'warning' | 'error' | 'default'> = {
      healthy: 'success',
      warning: 'warning',
      critical: 'error',
      breaching: 'error',
    };
    return map[status] || 'default';
  };

  const getStatusLabel = (status: string) => {
    const map: Record<string, string> = {
      healthy: '健康',
      warning: '警告',
      critical: '严重',
      breaching: '违规',
    };
    return map[status] || status;
  };

  const columns = [
    { title: 'SLO 名称', dataIndex: 'name', key: 'name' },
    {
      title: '目标值', dataIndex: 'target', key: 'target',
      render: (v: number, r: SLOStatusDetail) => {
        if (r.name.toLowerCase().includes('latency') || r.name.includes('延迟')) {
          return `${v.toFixed(0)}ms`;
        }
        return `${(v * 100).toFixed(1)}%`;
      },
    },
    {
      title: '当前值', dataIndex: 'current', key: 'current',
      render: (v: number, r: SLOStatusDetail) => {
        if (r.name.toLowerCase().includes('latency') || r.name.includes('延迟')) {
          return <span style={{ color: v <= r.target ? '#3f8600' : '#cf1322' }}>{v.toFixed(0)}ms</span>;
        }
        return <span style={{ color: v >= r.target ? '#3f8600' : '#cf1322' }}>{(v * 100).toFixed(1)}%</span>;
      },
    },
    {
      title: '剩余预算', dataIndex: 'budget_remaining', key: 'budget_remaining',
      render: (v: number) => <Progress percent={Math.round(v * 100)} size="small" status={v < 0.3 ? 'exception' : 'active'} />,
    },
    {
      title: '状态', key: 'status', dataIndex: 'status',
      render: (s: string) => <Badge status={getStatusBadge(s)} text={getStatusLabel(s)} />,
    },
  ];

  return (
    <div>
      <Alert message="SLO 监控跟踪每次 LLM 调用的延迟、成功率、Token 用量和成本" type="info" showIcon style={{ marginBottom: 16 }} />

      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={8}>
          <Card size="small">
            <Statistic title="LLM 成功率" value={llmSummary.successRate * 100} suffix="%" precision={1} valueStyle={{ color: llmSummary.successRate >= 0.95 ? '#3f8600' : '#cf1322' }} />
          </Card>
        </Col>
        <Col span={8}>
          <Card size="small">
            <Statistic title="平均延迟" value={llmSummary.avgLatency} suffix="ms" precision={0} />
          </Card>
        </Col>
        <Col span={8}>
          <Card size="small">
            <Statistic title="SLO 达标数" value={slos.filter(s => {
              if (s.name.toLowerCase().includes('latency') || s.name.includes('延迟')) {
                return s.current <= s.target * 1000;
              }
              return s.current >= s.target;
            }).length} suffix={`/ ${slos.length}`} />
          </Card>
        </Col>
      </Row>

      <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)} style={{ marginBottom: 16 }}>创建 SLO</Button>
      <Table columns={columns} dataSource={slos} rowKey="name" loading={loading} />

      <Modal
        title="创建 SLO"
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={() => form.submit()}
      >
        <Form form={form} layout="vertical" onFinish={createSLO}>
          <Form.Item name="name" label="SLO 名称" rules={[{ required: true, message: '请输入 SLO 名称' }]}>
            <Input placeholder="例如：API 响应时间" />
          </Form.Item>
          <Form.Item name="agent_id" label="Agent ID">
            <Input placeholder="留空表示全局 SLO" />
          </Form.Item>
          <Form.Item name="type" label="SLO 类型" rules={[{ required: true, message: '请选择 SLO 类型' }]}>
            <Select options={[
              { value: 'success_rate', label: '成功率' },
              { value: 'latency', label: '延迟' },
              { value: 'availability', label: '可用性' },
              { value: 'error_budget', label: '错误预算' },
            ]} />
          </Form.Item>
          <Form.Item name="target" label="目标值 (%)" rules={[{ required: true, message: '请输入目标值' }]} initialValue={99}>
            <InputNumber min={0} max={100} precision={1} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
