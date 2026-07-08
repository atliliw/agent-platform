import { useState, useEffect } from 'react';
import {
  Table, Tag, Button, Space, Badge, Input, Select, Slider,
  Modal, Form, Tooltip, Popconfirm, Row, Col, Statistic, Alert, message
} from 'antd';
import { PlusOutlined, ExperimentOutlined, DeleteOutlined } from '@ant-design/icons';
import { harnessApi } from '../../api/harness';

interface ABTest {
  id: string;
  name: string;
  type: string;
  control_config: string;
  variant_config: string;
  traffic_split: number;
  status: string;
  created_at: number;
}

interface ABTestResult {
  control_score: number;
  variant_score: number;
  delta: number;
  p_value: number;
  significant: boolean;
  recommended: string;
}

export default function ABTestPanel() {
  const [tests, setTests] = useState<ABTest[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [resultModalOpen, setResultModalOpen] = useState(false);
  const [resultData, setResultData] = useState<ABTestResult | null>(null);
  const [resultLoading, setResultLoading] = useState(false);
  const [currentTestName, setCurrentTestName] = useState('');
  const [form] = Form.useForm();

  useEffect(() => {
    loadTests();
  }, []);

  const loadTests = async () => {
    setLoading(true);
    try {
      const res = await harnessApi.listABTests() as any;
      setTests(res?.tests || []);
    } catch {
      setTests([]);
    } finally {
      setLoading(false);
    }
  };

  const createTest = async (values: any) => {
    try {
      await harnessApi.createABTest({
        name: values.name,
        type: values.type,
        control_model: values.type === 'model' ? values.control_config : '',
        variant_model: values.type === 'model' ? values.variant_config : '',
        control_config: values.type === 'prompt' ? values.control_config : '',
        variant_config: values.type === 'prompt' ? values.variant_config : '',
        traffic_split: values.traffic_split / 100,
      });
      message.success('创建成功');
      setModalOpen(false);
      form.resetFields();
      loadTests();
    } catch {
      message.error('创建失败');
    }
  };

  const viewResult = async (test: ABTest) => {
    setCurrentTestName(test.name);
    setResultModalOpen(true);
    setResultLoading(true);
    setResultData(null);
    try {
      const res = await harnessApi.getABTestResult(test.id) as any;
      setResultData(res?.result || null);
    } catch {
      setResultData(null);
    } finally {
      setResultLoading(false);
    }
  };

  const deleteTest = async (id: string) => {
    try {
      await harnessApi.deleteABTest(id);
      message.success('删除成功');
      loadTests();
    } catch {
      message.error('删除失败');
    }
  };

  const columns = [
    { title: '实验名称', dataIndex: 'name', key: 'name' },
    {
      title: '类型', dataIndex: 'type', key: 'type',
      render: (t: string) => {
        if (t === 'prompt') return <Tag color="blue">Prompt</Tag>;
        if (t === 'model') return <Tag color="green">模型</Tag>;
        return <Tag>{t || '-'}</Tag>;
      },
    },
    {
      title: '对照组', dataIndex: 'control_config', key: 'control_config',
      render: (v: string) => (
        <Tooltip title={v}>
          <span>{v ? (v.length > 20 ? v.slice(0, 20) + '...' : v) : '-'}</span>
        </Tooltip>
      ),
    },
    {
      title: '实验组', dataIndex: 'variant_config', key: 'variant_config',
      render: (v: string) => (
        <Tooltip title={v}>
          <Tag color="blue">{v ? (v.length > 20 ? v.slice(0, 20) + '...' : v) : '-'}</Tag>
        </Tooltip>
      ),
    },
    {
      title: '流量分配', dataIndex: 'traffic_split', key: 'traffic_split',
      render: (v: number) => `${Math.round(v * 100)}% / ${Math.round((1 - v) * 100)}%`,
    },
    {
      title: '状态', dataIndex: 'status', key: 'status',
      render: (s: string) => {
        const statusMap: Record<string, 'processing' | 'success' | 'error' | 'default'> = {
          running: 'processing', completed: 'success', paused: 'default', stopped: 'error',
        };
        return <Badge status={statusMap[s] || 'default'} text={s} />;
      },
    },
    {
      title: '操作', key: 'action',
      render: (_: any, record: ABTest) => (
        <Space>
          <Button size="small" icon={<ExperimentOutlined />} onClick={() => viewResult(record)}>
            查看结果
          </Button>
          <Popconfirm
            title="确定删除该实验？"
            description="删除后实验数据将无法恢复"
            onConfirm={() => deleteTest(record.id)}
            okText="确定"
            cancelText="取消"
          >
            <Button size="small" danger icon={<DeleteOutlined />}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Alert message="A/B 测试支持 Prompt 对比、模型对比，自动统计显著性" type="info" showIcon style={{ marginBottom: 16 }} />
      <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)} style={{ marginBottom: 16 }}>
        创建测试
      </Button>
      <Table columns={columns} dataSource={tests} rowKey="id" loading={loading} />

      {/* 创建实验 Modal */}
      <Modal title="创建 A/B 测试" open={modalOpen} onCancel={() => setModalOpen(false)} onOk={() => form.submit()}>
        <Form form={form} layout="vertical" onFinish={createTest}>
          <Form.Item name="name" label="实验名称" rules={[{ required: true, message: '请输入实验名称' }]}>
            <Input placeholder="例如：Prompt风格对比" />
          </Form.Item>
          <Form.Item name="type" label="测试类型" rules={[{ required: true, message: '请选择类型' }]}>
            <Select options={[
              { value: 'prompt', label: 'Prompt 对比' },
              { value: 'model', label: '模型对比' },
            ]} />
          </Form.Item>
          <Form.Item name="control_config" label="对照组配置" rules={[{ required: true, message: '请输入对照组配置' }]}>
            <Input.TextArea rows={3} placeholder="对照组 Prompt 或模型名称" />
          </Form.Item>
          <Form.Item name="variant_config" label="实验组配置" rules={[{ required: true, message: '请输入实验组配置' }]}>
            <Input.TextArea rows={3} placeholder="实验组 Prompt 或模型名称" />
          </Form.Item>
          <Form.Item name="traffic_split" label="实验组流量占比" initialValue={50}>
            <Slider min={10} max={90} marks={{ 10: '10%', 50: '50%', 90: '90%' }} />
          </Form.Item>
        </Form>
      </Modal>

      {/* 查看结果 Modal */}
      <Modal
        title={`实验结果 — ${currentTestName}`}
        open={resultModalOpen}
        onCancel={() => setResultModalOpen(false)}
        footer={null}
        width={640}
      >
        {resultLoading ? (
          <div style={{ textAlign: 'center', padding: 40 }}>加载中...</div>
        ) : resultData ? (
          <div>
            <Row gutter={16}>
              <Col span={8}>
                <Statistic
                  title="对照组得分"
                  value={resultData.control_score}
                  precision={3}
                  valueStyle={{ color: '#1677ff' }}
                />
              </Col>
              <Col span={8}>
                <Statistic
                  title="实验组得分"
                  value={resultData.variant_score}
                  precision={3}
                  valueStyle={{ color: '#52c41a' }}
                />
              </Col>
              <Col span={8}>
                <Statistic
                  title="差异 (Delta)"
                  value={resultData.delta}
                  precision={3}
                  valueStyle={{ color: resultData.delta > 0 ? '#52c41a' : '#ff4d4f' }}
                  prefix={resultData.delta > 0 ? '+' : ''}
                />
              </Col>
            </Row>
            <div style={{ marginTop: 24 }}>
              <Row gutter={16}>
                <Col span={8}>
                  <Statistic title="P 值" value={resultData.p_value} precision={4} />
                </Col>
                <Col span={8}>
                  <div style={{ textAlign: 'center' }}>
                    <div style={{ color: 'rgba(0,0,0,0.45)', fontSize: 14, marginBottom: 8 }}>统计显著性</div>
                    {resultData.significant ? (
                      <Tag color="green" style={{ fontSize: 16, padding: '4px 16px' }}>✓ 显著</Tag>
                    ) : (
                      <Tag color="orange" style={{ fontSize: 16, padding: '4px 16px' }}>不显著</Tag>
                    )}
                  </div>
                </Col>
                <Col span={8}>
                  <div style={{ textAlign: 'center' }}>
                    <div style={{ color: 'rgba(0,0,0,0.45)', fontSize: 14, marginBottom: 8 }}>建议操作</div>
                    <Tag color={
                      resultData.recommended === 'promote_variant' ? 'green' :
                      resultData.recommended === 'promote_control' ? 'blue' : 'default'
                    } style={{ fontSize: 14, padding: '4px 12px' }}>
                      {resultData.recommended === 'promote_variant' ? '采用实验组' :
                       resultData.recommended === 'promote_control' ? '采用对照组' :
                       resultData.recommended === 'continue' ? '继续实验' : resultData.recommended}
                    </Tag>
                  </div>
                </Col>
              </Row>
            </div>
            {resultData.significant && (
              <Alert
                style={{ marginTop: 16 }}
                type="success"
                message="实验结果已达到统计显著性 (p < 0.05)，可以做出决策"
                showIcon
              />
            )}
          </div>
        ) : (
          <div style={{ textAlign: 'center', padding: 40, color: '#999' }}>
            暂无结果数据，请先发送一些聊天请求以积累实验数据
          </div>
        )}
      </Modal>
    </div>
  );
}
