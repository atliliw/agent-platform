import { useState, useEffect } from 'react';
import {
  Table, Tag, Button, Space, Badge, Input, Select,
  Modal, Form, Popconfirm, message, Alert
} from 'antd';
import {
  PlusOutlined, DeleteOutlined, PauseCircleOutlined,
  PlayCircleOutlined, ThunderboltOutlined
} from '@ant-design/icons';
import { harnessApi } from '../../api/harness';

interface Schedule {
  id: string;
  name: string;
  cron_expression: string;
  task_type: string;
  agent_id: string;
  status: 'running' | 'paused' | 'completed' | 'error';
  next_run: string;
  last_run: string;
  created_at: string;
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
        cron_expression: values.cron_expression,
        task_type: values.task_type,
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

  const getStatusBadge = (status: string) => {
    const map: Record<string, 'processing' | 'success' | 'warning' | 'error' | 'default'> = {
      running: 'processing',
      paused: 'warning',
      completed: 'success',
      error: 'error',
    };
    return map[status] || 'default';
  };

  const getStatusLabel = (status: string) => {
    const map: Record<string, string> = {
      running: '运行中',
      paused: '已暂停',
      completed: '已完成',
      error: '错误',
    };
    return map[status] || status;
  };

  const getTaskTypeLabel = (t: string) => {
    const map: Record<string, string> = {
      evaluation: '评估任务',
      cleanup: '清理任务',
      report: '报告生成',
      health_check: '健康检查',
      custom: '自定义',
    };
    return map[t] || t;
  };

  const columns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: 'Cron 表达式', dataIndex: 'cron_expression', key: 'cron_expression', render: (v: string) => <Tag color="blue">{v}</Tag> },
    { title: '任务类型', dataIndex: 'task_type', key: 'task_type', render: (t: string) => getTaskTypeLabel(t) },
    { title: 'Agent', dataIndex: 'agent_id', key: 'agent_id' },
    {
      title: '状态', dataIndex: 'status', key: 'status',
      render: (s: string) => <Badge status={getStatusBadge(s)} text={getStatusLabel(s)} />,
    },
    {
      title: '下次执行', dataIndex: 'next_run', key: 'next_run',
      render: (v: string) => v ? new Date(v).toLocaleString() : '-',
    },
    {
      title: '操作', key: 'action', width: 240,
      render: (_: any, record: Schedule) => (
        <Space>
          {record.status === 'running' && (
            <Button size="small" icon={<PauseCircleOutlined />} onClick={() => handlePause(record.id)}>暂停</Button>
          )}
          {record.status === 'paused' && (
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
      <Alert message="调度器 — 定时执行评估、清理、报告等任务" type="info" showIcon style={{ marginBottom: 16 }} />

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
            <Input placeholder="例如：每日评估任务" />
          </Form.Item>
          <Form.Item name="cron_expression" label="Cron 表达式" rules={[{ required: true, message: '请输入 Cron 表达式' }]}>
            <Input placeholder="0 8 * * * (每天8点)" />
          </Form.Item>
          <Form.Item name="task_type" label="任务类型" rules={[{ required: true, message: '请选择任务类型' }]}>
            <Select placeholder="选择任务类型">
              <Select.Option value="evaluation">评估任务</Select.Option>
              <Select.Option value="cleanup">清理任务</Select.Option>
              <Select.Option value="report">报告生成</Select.Option>
              <Select.Option value="health_check">健康检查</Select.Option>
              <Select.Option value="custom">自定义</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="agent_id" label="Agent ID">
            <Input placeholder="留空表示全局任务" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
