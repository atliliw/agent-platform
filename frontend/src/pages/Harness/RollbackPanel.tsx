import { useState, useEffect } from 'react';
import {
  Table, Tag, Button, Space, Badge, Modal, Form, Input,
  Popconfirm, message, Alert, Descriptions
} from 'antd';
import { PlusOutlined, RollbackOutlined, DeleteOutlined } from '@ant-design/icons';
import { harnessApi } from '../../api/harness';

interface Snapshot {
  id: string;
  name: string;
  description: string;
  agent_configs: number;
  prompt_configs: number;
  model_configs: number;
  created_at: string;
  created_by: string;
}

export default function RollbackPanel() {
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [loading, setLoading] = useState(false);
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [detailModalOpen, setDetailModalOpen] = useState(false);
  const [selectedSnapshot, setSelectedSnapshot] = useState<Snapshot | null>(null);
  const [createForm] = Form.useForm();

  useEffect(() => {
    loadSnapshots();
  }, []);

  const loadSnapshots = async () => {
    setLoading(true);
    try {
      const res = await harnessApi.listSnapshots() as any;
      setSnapshots(res?.snapshots || []);
    } catch {
      setSnapshots([]);
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async () => {
    try {
      const values = await createForm.validateFields();
      await harnessApi.createSnapshot({
        name: values.name,
        description: values.description || '',
      });
      message.success('快照创建成功');
      setCreateModalOpen(false);
      createForm.resetFields();
      loadSnapshots();
    } catch {
      message.error('创建失败');
    }
  };

  const handleRollback = async (id: string) => {
    try {
      await harnessApi.rollbackSnapshot(id);
      message.success('回滚成功');
      loadSnapshots();
    } catch {
      message.error('回滚失败');
    }
  };

  const showDetail = (snapshot: Snapshot) => {
    setSelectedSnapshot(snapshot);
    setDetailModalOpen(true);
  };

  const columns = [
    { title: '快照名称', dataIndex: 'name', key: 'name' },
    { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
    {
      title: '配置数', key: 'configs',
      render: (_: any, record: Snapshot) => (
        <Space>
          <Tag color="blue">Agent: {record.agent_configs}</Tag>
          <Tag color="green">Prompt: {record.prompt_configs}</Tag>
          <Tag color="orange">Model: {record.model_configs}</Tag>
        </Space>
      ),
    },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at', render: (v: string) => v ? new Date(v).toLocaleString() : '-' },
    { title: '创建人', dataIndex: 'created_by', key: 'created_by' },
    {
      title: '操作', key: 'action', width: 240,
      render: (_: any, record: Snapshot) => (
        <Space>
          <Button size="small" onClick={() => showDetail(record)}>详情</Button>
          <Popconfirm
            title="确定回滚到此快照？"
            description="回滚将恢复快照时的所有配置，当前未保存的更改将丢失"
            onConfirm={() => handleRollback(record.id)}
            okText="确定回滚"
            cancelText="取消"
            okButtonProps={{ danger: true }}
          >
            <Button size="small" danger icon={<RollbackOutlined />}>回滚</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Alert message="配置回滚 — 创建配置快照，支持一键回滚" type="info" showIcon style={{ marginBottom: 16 }} />

      <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateModalOpen(true)} style={{ marginBottom: 16 }}>
        创建快照
      </Button>

      <Table columns={columns} dataSource={snapshots} rowKey="id" loading={loading} />

      {/* 创建快照 Modal */}
      <Modal
        title="创建配置快照"
        open={createModalOpen}
        onOk={handleCreate}
        onCancel={() => { setCreateModalOpen(false); createForm.resetFields(); }}
      >
        <Form form={createForm} layout="vertical">
          <Form.Item name="name" label="快照名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="例如：v1.2-stable" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} placeholder="描述此快照包含的配置变更" />
          </Form.Item>
        </Form>
      </Modal>

      {/* 快照详情 Modal */}
      <Modal
        title={`快照详情 — ${selectedSnapshot?.name || ''}`}
        open={detailModalOpen}
        onCancel={() => setDetailModalOpen(false)}
        footer={null}
        width={640}
      >
        {selectedSnapshot && (
          <Descriptions bordered column={2} size="small">
            <Descriptions.Item label="名称">{selectedSnapshot.name}</Descriptions.Item>
            <Descriptions.Item label="创建人">{selectedSnapshot.created_by || '-'}</Descriptions.Item>
            <Descriptions.Item label="创建时间">{selectedSnapshot.created_at ? new Date(selectedSnapshot.created_at).toLocaleString() : '-'}</Descriptions.Item>
            <Descriptions.Item label="描述" span={2}>{selectedSnapshot.description || '-'}</Descriptions.Item>
            <Descriptions.Item label="Agent 配置数">
              <Badge count={selectedSnapshot.agent_configs} showZero color="blue" />
            </Descriptions.Item>
            <Descriptions.Item label="Prompt 配置数">
              <Badge count={selectedSnapshot.prompt_configs} showZero color="green" />
            </Descriptions.Item>
            <Descriptions.Item label="Model 配置数" span={2}>
              <Badge count={selectedSnapshot.model_configs} showZero color="orange" />
            </Descriptions.Item>
          </Descriptions>
        )}
      </Modal>
    </div>
  );
}
