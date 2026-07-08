import { useState, useEffect } from 'react';
import {
  Table, Tag, Button, Space, Badge, Input, Select,
  Modal, Form, InputNumber, Popconfirm, message, Alert
} from 'antd';
import {
  PlusOutlined, DeleteOutlined, CaretRightOutlined, PauseOutlined
} from '@ant-design/icons';
import { harnessApi } from '../../api/harness';

interface ChaosExperiment {
  id: string;
  name: string;
  type: 'latency' | 'error' | 'timeout';
  target: string;
  parameters: string;
  status: 'running' | 'stopped' | 'completed' | 'failed';
  created_at: string;
  duration: number;
}

export default function ChaosPanel() {
  const [experiments, setExperiments] = useState<ChaosExperiment[]>([]);
  const [loading, setLoading] = useState(false);
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [createForm] = Form.useForm();

  useEffect(() => {
    loadExperiments();
  }, []);

  const loadExperiments = async () => {
    setLoading(true);
    try {
      const res = await harnessApi.listChaosExperiments() as any;
      setExperiments(res?.experiments || []);
    } catch {
      setExperiments([]);
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async () => {
    try {
      const values = await createForm.validateFields();
      await harnessApi.createChaosExperiment({
        name: values.name,
        type: values.type,
        target: values.target,
        parameters: values.parameters ? JSON.parse(values.parameters) : {},
        duration: values.duration || 60,
      });
      message.success('混沌实验创建成功');
      setCreateModalOpen(false);
      createForm.resetFields();
      loadExperiments();
    } catch (error) {
      if (error instanceof SyntaxError) {
        message.error('参数必须是有效的 JSON');
      } else {
        message.error('创建失败');
      }
    }
  };

  const handleStart = async (id: string) => {
    try {
      await harnessApi.startChaosExperiment(id);
      message.success('实验已启动');
      loadExperiments();
    } catch {
      message.error('启动失败');
    }
  };

  const handleStop = async (id: string) => {
    try {
      await harnessApi.stopChaosExperiment(id);
      message.success('实验已停止');
      loadExperiments();
    } catch {
      message.error('停止失败');
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await harnessApi.deleteChaosExperiment(id);
      message.success('删除成功');
      loadExperiments();
    } catch {
      message.error('删除失败');
    }
  };

  const getStatusBadge = (status: string) => {
    const map: Record<string, 'processing' | 'success' | 'error' | 'default'> = {
      running: 'processing',
      stopped: 'default',
      completed: 'success',
      failed: 'error',
    };
    return map[status] || 'default';
  };

  const getStatusLabel = (status: string) => {
    const map: Record<string, string> = {
      running: '运行中',
      stopped: '已停止',
      completed: '已完成',
      failed: '失败',
    };
    return map[status] || status;
  };

  const getTypeLabel = (t: string) => {
    const map: Record<string, string> = {
      latency: '延迟注入',
      error: '错误注入',
      timeout: '超时注入',
    };
    return map[t] || t;
  };

  const getTypeColor = (t: string) => {
    const map: Record<string, string> = {
      latency: 'orange',
      error: 'red',
      timeout: 'purple',
    };
    return map[t] || 'default';
  };

  const columns = [
    { title: '实验名称', dataIndex: 'name', key: 'name' },
    {
      title: '类型', dataIndex: 'type', key: 'type',
      render: (t: string) => <Tag color={getTypeColor(t)}>{getTypeLabel(t)}</Tag>,
    },
    { title: '目标', dataIndex: 'target', key: 'target' },
    {
      title: '持续时间', dataIndex: 'duration', key: 'duration',
      render: (v: number) => v ? `${v}s` : '-',
    },
    {
      title: '状态', dataIndex: 'status', key: 'status',
      render: (s: string) => <Badge status={getStatusBadge(s)} text={getStatusLabel(s)} />,
    },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at', render: (v: string) => v ? new Date(v).toLocaleString() : '-' },
    {
      title: '操作', key: 'action', width: 200,
      render: (_: any, record: ChaosExperiment) => (
        <Space>
          {record.status === 'stopped' && (
            <Button size="small" type="primary" icon={<CaretRightOutlined />} onClick={() => handleStart(record.id)}>启动</Button>
          )}
          {record.status === 'running' && (
            <Button size="small" icon={<PauseOutlined />} onClick={() => handleStop(record.id)}>停止</Button>
          )}
          <Popconfirm
            title="确定删除此混沌实验？"
            description="删除后实验数据将无法恢复"
            onConfirm={() => handleDelete(record.id)}
            okText="确定"
            cancelText="取消"
          >
            <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Alert
        message="混沌工程 — 通过注入故障验证系统韧性"
        description="支持延迟注入、错误注入、超时注入等实验类型"
        type="warning"
        showIcon
        style={{ marginBottom: 16 }}
      />

      <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateModalOpen(true)} style={{ marginBottom: 16 }}>
        创建实验
      </Button>

      <Table columns={columns} dataSource={experiments} rowKey="id" loading={loading} />

      <Modal
        title="创建混沌实验"
        open={createModalOpen}
        onOk={handleCreate}
        onCancel={() => { setCreateModalOpen(false); createForm.resetFields(); }}
        width={600}
      >
        <Form form={createForm} layout="vertical">
          <Form.Item name="name" label="实验名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="例如：API延迟测试" />
          </Form.Item>
          <Form.Item name="type" label="实验类型" rules={[{ required: true, message: '请选择类型' }]}>
            <Select placeholder="选择实验类型">
              <Select.Option value="latency">延迟注入</Select.Option>
              <Select.Option value="error">错误注入</Select.Option>
              <Select.Option value="timeout">超时注入</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="target" label="目标" rules={[{ required: true, message: '请输入目标' }]}>
            <Input placeholder="例如：chat-agent / /api/v2/chat" />
          </Form.Item>
          <Form.Item name="duration" label="持续时间 (秒)" initialValue={60}>
            <InputNumber min={10} max={3600} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="parameters" label="参数 (JSON，可选)">
            <Input.TextArea rows={3} placeholder='{"latency_ms": 500, "error_rate": 0.1}' />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
