import { useState, useEffect, useCallback } from 'react';
import {
  Card,
  Table,
  Button,
  Space,
  Drawer,
  Form,
  Input,
  Select,
  Tag,
  Popconfirm,
  message,
  Typography,
  Empty,
  Modal,
  Upload,
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  ThunderboltOutlined,
  DownloadOutlined,
  UploadOutlined,
  ImportOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { skillApi } from '../../api/skill';
import type { Skill, SkillInput, SkillStatus } from '../../api/skill';

const { Title, Text } = Typography;

export default function SkillsPage() {
  const [skills, setSkills] = useState<Skill[]>([]);
  const [loading, setLoading] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editing, setEditing] = useState<Skill | null>(null);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm<SkillInput>();
  const [importOpen, setImportOpen] = useState(false);
  const [importYaml, setImportYaml] = useState('');
  const [importing, setImporting] = useState(false);

  const loadSkills = useCallback(async () => {
    setLoading(true);
    try {
      const resp = await skillApi.listSkills();
      setSkills(resp?.skills || []);
    } catch (err) {
      message.error('加载技能列表失败');
      console.error(err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadSkills();
  }, [loadSkills]);

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    form.setFieldsValue({ status: 'active' as SkillStatus, tags: [] });
    setDrawerOpen(true);
  };

  const openEdit = (skill: Skill) => {
    setEditing(skill);
    form.setFieldsValue({
      name: skill.name,
      description: skill.description,
      instructions: skill.instructions,
      tools: skill.tools,
      tags: skill.tags,
      status: skill.status,
    });
    setDrawerOpen(true);
  };

  const handleSave = async () => {
    try {
      const values = await form.validateFields();
      setSaving(true);
      if (editing) {
        await skillApi.updateSkill(editing.id, values);
        message.success('技能已更新');
      } else {
        await skillApi.createSkill(values);
        message.success('技能已创建');
      }
      setDrawerOpen(false);
      loadSkills();
    } catch (err) {
      // validation errors are silent; network errors show a message
      if (err instanceof Error && err.message) {
        message.error(`保存失败: ${err.message}`);
      }
      console.error(err);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (skill: Skill) => {
    try {
      await skillApi.deleteSkill(skill.id);
      message.success(`技能 ${skill.name} 已删除`);
      loadSkills();
    } catch (err) {
      message.error('删除失败');
      console.error(err);
    }
  };

  // 导出：拉取 YAML 字符串，前端拼 Blob 触发下载
  const handleExport = async (skill: Skill) => {
    try {
      const resp = await skillApi.exportSkill(skill.id);
      const blob = new Blob([resp.yaml], { type: 'application/x-yaml' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = resp.filename;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
      message.success(`已导出 ${skill.name}`);
    } catch (err) {
      message.error('导出失败');
      console.error(err);
    }
  };

  const openImport = () => {
    setImportYaml('');
    setImportOpen(true);
  };

  // 读取上传的 .yaml 文件文本到文本框，供编辑后导入
  const onUploadFile = (file: File) => {
    const reader = new FileReader();
    reader.onload = () => setImportYaml(String(reader.result || ''));
    reader.onerror = () => message.error('读取文件失败');
    reader.readAsText(file);
    return false; // 阻止 antd 自动上传
  };

  const handleImport = async () => {
    if (!importYaml.trim()) {
      message.warning('请粘贴或上传技能 YAML');
      return;
    }
    try {
      setImporting(true);
      const resp = await skillApi.importSkill(importYaml);
      message.success(`已导入技能：${resp.skill.name}`);
      setImportOpen(false);
      loadSkills();
    } catch (err) {
      if (err instanceof Error && err.message) {
        message.error(`导入失败: ${err.message}`);
      } else {
        message.error('导入失败');
      }
      console.error(err);
    } finally {
      setImporting(false);
    }
  };

  const columns: ColumnsType<Skill> = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      render: (name: string) => <Tag color="geekblue">{name}</Tag>,
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: '标签',
      dataIndex: 'tags',
      key: 'tags',
      width: 200,
      render: (tags?: string[]) =>
        tags && tags.length > 0 ? (
          <Space size={4} wrap>
            {tags.map((t) => (
              <Tag key={t}>{t}</Tag>
            ))}
          </Space>
        ) : (
          <Text type="secondary">-</Text>
        ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 90,
      render: (status: SkillStatus) => (
        <Tag color={status === 'active' ? 'green' : 'default'}>{status}</Tag>
      ),
    },
    {
      title: '版本',
      dataIndex: 'version',
      key: 'version',
      width: 70,
    },
    {
      title: '操作',
      key: 'action',
      width: 220,
      render: (_, record) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(record)}>
            编辑
          </Button>
          <Button
            size="small"
            icon={<DownloadOutlined />}
            onClick={() => handleExport(record)}
          >
            导出
          </Button>
          <Popconfirm
            title="删除技能"
            description={`确定删除 ${record.name}？挂载该技能的 Agent 将不再可用。`}
            onConfirm={() => handleDelete(record)}
            okText="删除"
            cancelText="取消"
            okButtonProps={{ danger: true }}
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
      <Card
        title={
          <Space>
            <ThunderboltOutlined />
            <Title level={4} style={{ margin: 0 }}>
              技能库
            </Title>
            <Text type="secondary">能力模块，Agent 通过 ID 挂载（多对多）</Text>
          </Space>
        }
        extra={
          <Space>
            <Button icon={<ImportOutlined />} onClick={openImport}>
              导入技能
            </Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              新建技能
            </Button>
          </Space>
        }
      >
        <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
          技能采用渐进式披露：仅 Name + Description 注入 Agent 提示词，完整 Instructions 通过
          load_skill 工具按需加载。在 Agent 编辑器中为 Agent 勾选挂载。
        </Text>
        <Table
          columns={columns}
          dataSource={skills}
          rowKey="id"
          loading={loading}
          pagination={false}
          locale={{
            emptyText: (
              <Empty description="暂无技能，点击「新建技能」创建第一个" />
            ),
          }}
        />
      </Card>

      <Drawer
        title={editing ? `编辑技能：${editing.name}` : '新建技能'}
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        width={560}
        extra={
          <Space>
            <Button onClick={() => setDrawerOpen(false)}>取消</Button>
            <Button type="primary" loading={saving} onClick={handleSave}>
              保存
            </Button>
          </Space>
        }
        destroyOnClose
      >
        <Form form={form} layout="vertical" initialValues={{ status: 'active' }}>
          <Form.Item
            name="name"
            label="技能名称（唯一，Agent 调用 load_skill 时使用）"
            rules={[{ required: true, message: '请输入技能名称' }]}
          >
            <Input placeholder="例如：code-review" />
          </Form.Item>
          <Form.Item
            name="description"
            label="描述（注入 Agent 提示词的一句话摘要）"
            rules={[{ required: true, message: '请输入描述' }]}
          >
            <Input.TextArea rows={2} placeholder="一句话说明这个技能做什么" />
          </Form.Item>
          <Form.Item
            name="instructions"
            label="完整 Instructions（按需加载的详细提示词）"
            rules={[{ required: true, message: '请输入 Instructions' }]}
          >
            <Input.TextArea
              rows={12}
              placeholder="Agent 调用 load_skill 后收到的完整指令..."
              style={{ fontFamily: 'monospace', fontSize: 13 }}
            />
          </Form.Item>
          <Form.Item name="tags" label="标签">
            <Select mode="tags" placeholder="按回车添加标签" tokenSeparators={[',']} />
          </Form.Item>
          <Form.Item name="status" label="状态">
            <Select
              options={[
                { value: 'active', label: 'active（已启用，注入提示词）' },
                { value: 'draft', label: 'draft（草稿，不注入）' },
              ]}
            />
          </Form.Item>
        </Form>
      </Drawer>

      <Modal
        title="导入技能"
        open={importOpen}
        onCancel={() => setImportOpen(false)}
        width={620}
        footer={
          <Space>
            <Button onClick={() => setImportOpen(false)}>取消</Button>
            <Button type="primary" loading={importing} onClick={handleImport}>
              导入
            </Button>
          </Space>
        }
      >
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Upload
            accept=".yaml,.yml"
            showUploadList={false}
            beforeUpload={(file) => onUploadFile(file)}
          >
            <Button icon={<UploadOutlined />}>上传 YAML 文件</Button>
          </Upload>
          <Input.TextArea
            value={importYaml}
            onChange={(e) => setImportYaml(e.target.value)}
            rows={14}
            placeholder={
              '粘贴技能 YAML，或上传 .yaml 文件后在此编辑。\n' +
              '带 id 的会更新已有技能，不带 id 的会新建。'
            }
            style={{ fontFamily: 'monospace', fontSize: 13 }}
          />
        </Space>
      </Modal>
    </div>
  );
}
