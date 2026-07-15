import { useState, useEffect } from 'react';
import {
  Table, Tag, Button, Space, Badge, Select,
  Modal, Form, Input, Popconfirm, message, Alert
} from 'antd';
import {
  PlusOutlined, DeleteOutlined, PauseCircleOutlined,
  PlayCircleOutlined, ThunderboltOutlined
} from '@ant-design/icons';
import { harnessApi } from '../../api/harness';

interface Schedule {
  id: string;
  name: string;
  type: string;
  eval_type: string;
  schedule_expr: string;
  enabled: boolean;
  agent_id: string;
  created_at: number;
  updated_at: number;
}

export default function SchedulerPanel() {
  const [schedules, setSchedules] = useState<Schedule[]>([]);
  const [loading, setLoading] = useState(false);
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [createForm] = Form.useForm();

  useEffect(() => {
    loadSchedules();
  }, []);

  const loadSchedules = async () => {
    setLoading(true);
    try {
      const res = await harnessApi.listSchedules() as any;
      setSchedules(res?.schedules || []);
    } catch {
      setSchedules([]);
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async () => {
    try {
      const values = await createForm.validateFields();
      await harnessApi.createSchedule({
        name: values.name,
        type: values.type || 'interval',
        eval_type: values.eval_type || 'slo',
        schedule_expr: values.schedule_expr,
        agent_id: values.agent_id || '',
      });
      message.success('调度任务创建成功');
      setCreateModalOpen(false);
      createForm.resetFields();
      loadSchedules();
    } catch {
      message.error('创建失败');
    }
  };

  const handlePause = async (id: string) => {
    try {
      await harnessApi.pauseSchedule(id);
      message.success('已暂停');
      loadSchedules();
    } catch {
      message.error('暂停失败');
    }
  };

  const handleResume = async (id: string) => {
    try {
      await harnessApi.resumeSchedule(id);
      message.success('已恢复');
      loadSchedules();
    } catch {
      message.error('恢复失败');
    }
  };

  const handleRunNow = async (id: string) => {
    try {
      await harnessApi.runScheduleNow(id);
      message.success('已触发执行');
      loadSchedules();
    } catch {
      message.error('触发失败');
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await harnessApi.deleteSchedule(id);
      message.success('删除成功');
      loadSchedules();
    } catch {
      message.error('删除失败');
    }
  };

  const getEvalTypeLabel = (t: string) => {
    const map: Record<string, string> = {
      slo: 'SLO 监控',
      abtest: 'A/B 测试',
      cost: '成本审计',
      feature_flag: 'Flag 检查',
      evaluation: '质量评估',
    };
    return map[t] || t;
  };

  const getEvalTypeColor = (t: string) => {
    const map: Record<string, string> = {
      slo: 'blue',
      abtest: 'cyan',
      cost: 'green',
      feature_flag: 'purple',
      evaluation: 'orange',
    };
    return map[t] || 'default';
  };

  const getTypeLabel = (t: string) => {
    const map: Record<string, string> = {
      interval: '固定间隔',
      cron: 'Cron 表达式',
      once: '一次性',
    };
    return map[t] || t;
  };

  const columns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    {
      title: '评估类型', dataIndex: 'eval_type', key: 'eval_type',
      render: (t: string) => <Tag color={getEvalTypeColor(t)}>{getEvalTypeLabel(t)}</Tag>,
    },
    {
      title: '调度方式', dataIndex: 'type', key: 'type',
      render: (t: string) => <Tag>{getTypeLabel(t)}</Tag>,
    },
    { title: '表达式', dataIndex: 'schedule_expr', key: 'schedule_expr', render: (v: string) => <Tag color="blue">{v}</Tag> },
    { title: 'Agent', dataIndex: 'agent_id', key: 'agent_id', render: (v: string) => v || <Tag>全局</Tag> },
    {
      title: '状态', key: 'status', width: 100,
      render: (_: any, record: Schedule) => (
        <Badge
          status={record.enabled ? 'processing' : 'default'}
          text={record.enabled ? '运行中' : '已暂停'}
        />
      ),
    },
    {
      title: '操作', key: 'action', width: 240,
      render: (_: any, record: Schedule) => (
        <Space>
          {record.enabled && (
            <Button size="small" icon={<PauseCircleOutlined />} onClick={() => handlePause(record.id)}>暂停</Button>
          )}
          {!record.enabled && (
            <Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={() => handleResume(record.id)}>恢复</Button>
          )}
          <Button size="small" icon={<ThunderboltOutlined />} onClick={() => handleRunNow(record.id)}>立即执行</Button>
          <Popconfirm
            title="确定删除此调度任务？"
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
      <Alert message="调度器 — 定时执行 SLO 监控、A/B 测试、成本审计等任务" type="info" showIcon style={{ marginBottom: 16 }} />

      <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateModalOpen(true)} style={{ marginBottom: 16 }}>
        创建调度
      </Button>

      <Table columns={columns} dataSource={schedules} rowKey="id" loading={loading} />

      <Modal
        title="创建调度任务"
        open={createModalOpen}
        onOk={handleCreate}
        onCancel={() => { setCreateModalOpen(false); createForm.resetFields(); }}
        width={600}
      >
        <Form form={createForm} layout="vertical">
          <Form.Item name="name" label="任务名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="例如：SLO 每5分钟监控" />
          </Form.Item>
          <div style={{ display: 'flex', gap: 16 }}>
            <Form.Item name="eval_type" label="评估类型" initialValue="slo" rules={[{ required: true }]} style={{ flex: 1 }}>
              <Select>
                <Select.Option value="slo">SLO 监控</Select.Option>
                <Select.Option value="abtest">A/B 测试</Select.Option>
                <Select.Option value="cost">成本审计</Select.Option>
                <Select.Option value="feature_flag">Flag 检查</Select.Option>
                <Select.Option value="evaluation">质量评估</Select.Option>
              </Select>
            </Form.Item>
            <Form.Item name="type" label="调度方式" initialValue="interval" style={{ flex: 1 }}>
              <Select>
                <Select.Option value="interval">固定间隔</Select.Option>
                <Select.Option value="cron">Cron 表达式</Select.Option>
                <Select.Option value="once">一次性</Select.Option>
              </Select>
            </Form.Item>
          </div>
          <Form.Item name="schedule_expr" label="间隔/表达式" rules={[{ required: true, message: '请输入' }]}>
            <Input placeholder="5m (5分钟) / 0 8 * * * (每天8点)" />
          </Form.Item>
          <Form.Item name="agent_id" label="Agent ID">
            <Input placeholder="留空表示全局任务" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
