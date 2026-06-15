import { useState, useEffect } from 'react';
import {
  Card,
  Form,
  Input,
  Select,
  InputNumber,
  Switch,
  Button,
  Space,
  message,
  Divider,
  Row,
  Col,
  Tabs,
  Table,
  Tag,
  Alert,
} from 'antd';
import {
  SaveOutlined,
  UndoOutlined,
  PlayCircleOutlined,
  PlusOutlined,
  DeleteOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import yaml from 'js-yaml';

// Agent 配置类型
interface AgentConfig {
  name: string;
  model: string;
  system_prompt: string;
  max_steps?: number;
  timeout?: number;
  temperature?: number;
  tools?: string[];
  memory_enabled?: boolean;
  reflection_enabled?: boolean;
  description?: string;
  metadata?: Record<string, unknown>;
}

// 工具定义
interface ToolDefinition {
  name: string;
  description: string;
  category: string;
}

export default function AgentEditor() {
  // Agent 列表
  const [agents, setAgents] = useState<string[]>([]);
  const [selectedAgent, setSelectedAgent] = useState<string>('');
  const [config, setConfig] = useState<AgentConfig | null>(null);
  const [originalConfig, setOriginalConfig] = useState<AgentConfig | null>(null);
  const [yamlSource, setYamlSource] = useState('');
  const [, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  // 表单
  const [form] = Form.useForm();

  // 可用工具
  const availableTools: ToolDefinition[] = [
    { name: 'web_search', description: '网络搜索', category: 'search' },
    { name: 'browser_navigate', description: '浏览器导航', category: 'browser' },
    { name: 'browser_click', description: '浏览器点击', category: 'browser' },
    { name: 'browser_type', description: '浏览器输入', category: 'browser' },
    { name: 'browser_screenshot', description: '浏览器截图', category: 'browser' },
    { name: 'file_read', description: '读取文件', category: 'file' },
    { name: 'file_write', description: '写入文件', category: 'file' },
    { name: 'http_request', description: 'HTTP 请求', category: 'network' },
    { name: 'code_execute', description: '执行代码', category: 'code' },
    { name: 'memory_store', description: '存储记忆', category: 'memory' },
    { name: 'memory_recall', description: '召回记忆', category: 'memory' },
  ];

  // 可用模型
  const availableModels = [
    { value: 'qwen-plus', label: 'Qwen Plus' },
    { value: 'qwen-turbo', label: 'Qwen Turbo' },
    { value: 'qwen-max', label: 'Qwen Max' },
    { value: 'gpt-4', label: 'GPT-4' },
    { value: 'gpt-3.5-turbo', label: 'GPT-3.5 Turbo' },
    { value: 'claude-3-opus', label: 'Claude 3 Opus' },
    { value: 'claude-3-sonnet', label: 'Claude 3 Sonnet' },
  ];

  // 加载 Agent 列表
  const loadAgents = async () => {
    try {
      // 模拟数据，实际应从 API 获取
      const agentList = ['browser-agent', 'chat-agent', 'task-agent', 'research-agent'];
      setAgents(agentList);
      if (agentList.length > 0) {
        loadAgent(agentList[0]);
      }
    } catch (error) {
      message.error('加载 Agent 列表失败');
      console.error(error);
    }
  };

  // 加载 Agent 配置
  const loadAgent = async (agentName: string) => {
    setLoading(true);
    try {
      // 模拟数据，实际应从 API 获取
      const agentConfig: AgentConfig = {
        name: agentName,
        model: 'qwen-plus',
        system_prompt: '你是一个智能助手，帮助用户完成各种任务。',
        max_steps: 10,
        timeout: 300,
        temperature: 0.7,
        tools: ['web_search', 'browser_navigate'],
        memory_enabled: true,
        reflection_enabled: false,
        description: '这是一个示例 Agent',
        metadata: {
          version: '1.0',
          author: 'system',
        },
      };
      setConfig(agentConfig);
      setOriginalConfig(agentConfig);
      form.setFieldsValue(agentConfig);
      setYamlSource(yaml.dump(agentConfig, { indent: 2 }));
    } catch (error) {
      message.error('加载 Agent 配置失败');
      console.error(error);
    } finally {
      setLoading(false);
    }
  };

  // 保存配置
  const handleSave = async () => {
    try {
      const values = await form.validateFields();
      setSaving(true);
      const newConfig = { ...config, ...values };
      // 实际应调用 API 保存
      setConfig(newConfig);
      setYamlSource(yaml.dump(newConfig, { indent: 2 }));
      message.success('配置已保存');
    } catch (error) {
      message.error('保存失败');
      console.error(error);
    } finally {
      setSaving(false);
    }
  };

  // 重置配置
  const handleReset = () => {
    if (originalConfig) {
      form.setFieldsValue(originalConfig);
      setConfig(originalConfig);
      setYamlSource(yaml.dump(originalConfig, { indent: 2 }));
      message.info('配置已重置');
    }
  };

  // 表单值变化时更新 YAML
  const handleValuesChange = (_: unknown, allValues: AgentConfig) => {
    const newConfig = { ...config, ...allValues };
    setConfig(newConfig);
    setYamlSource(yaml.dump(newConfig, { indent: 2 }));
  };

  // 工具表格列
  const toolColumns: ColumnsType<ToolDefinition> = [
    {
      title: '工具名称',
      dataIndex: 'name',
      key: 'name',
      render: (name: string) => <Tag color="blue">{name}</Tag>,
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
    },
    {
      title: '分类',
      dataIndex: 'category',
      key: 'category',
      render: (cat: string) => <Tag>{cat}</Tag>,
    },
    {
      title: '操作',
      key: 'action',
      render: (_, record) => (
        <Button
          size="small"
          type="primary"
          ghost
          onClick={() => {
            const currentTools = form.getFieldValue('tools') || [];
            if (!currentTools.includes(record.name)) {
              form.setFieldValue('tools', [...currentTools, record.name]);
              handleValuesChange(null, { ...form.getFieldsValue(), tools: [...currentTools, record.name] });
            }
          }}
        >
          添加
        </Button>
      ),
    },
  ];

  // 已选工具列
  const selectedToolColumns: ColumnsType<string> = [
    {
      title: '工具名称',
      dataIndex: 'name',
      key: 'name',
      render: (name: string) => <Tag color="green">{name}</Tag>,
    },
    {
      title: '描述',
      dataIndex: 'name',
      key: 'description',
      render: (name: string) => availableTools.find((t) => t.name === name)?.description || '-',
    },
    {
      title: '操作',
      key: 'action',
      render: (_, name) => (
        <Button
          size="small"
          danger
          icon={<DeleteOutlined />}
          onClick={() => {
            const currentTools = form.getFieldValue('tools') || [];
            const newTools = currentTools.filter((t: string) => t !== name);
            form.setFieldValue('tools', newTools);
            handleValuesChange(null, { ...form.getFieldsValue(), tools: newTools });
          }}
        >
          移除
        </Button>
      ),
    },
  ];

  // Tab 项
  const tabItems = [
    {
      key: 'basic',
      label: '基本配置',
      children: (
        <>
          <Form.Item name="name" label="Agent 名称" rules={[{ required: true }]}>
            <Input placeholder="例如：browser-agent" disabled />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={2} placeholder="Agent 功能描述" />
          </Form.Item>
          <Form.Item name="model" label="模型" rules={[{ required: true }]}>
            <Select options={availableModels} />
          </Form.Item>
          <Row gutter={16}>
            <Col span={8}>
              <Form.Item name="max_steps" label="最大步骤数">
                <InputNumber min={1} max={100} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="timeout" label="超时时间(秒)">
                <InputNumber min={1} max={3600} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="temperature" label="Temperature">
                <InputNumber min={0} max={2} step={0.1} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="memory_enabled" label="启用记忆" valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="reflection_enabled" label="启用反思" valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
          </Row>
        </>
      ),
    },
    {
      key: 'prompt',
      label: 'System Prompt',
      children: (
        <Form.Item name="system_prompt" label="系统提示词" rules={[{ required: true }]}>
          <Input.TextArea
            rows={12}
            placeholder="输入 Agent 的系统提示词..."
            style={{ fontFamily: 'monospace' }}
          />
        </Form.Item>
      ),
    },
    {
      key: 'tools',
      label: '工具配置',
      children: (
        <>
          <Alert
            message="已选工具"
            description={
              <Table
                columns={selectedToolColumns}
                dataSource={form.getFieldValue('tools') || []}
                rowKey="name"
                size="small"
                pagination={false}
              />
            }
            type="info"
            style={{ marginBottom: 16 }}
          />
          <Divider>可用工具</Divider>
          <Table
            columns={toolColumns}
            dataSource={availableTools}
            rowKey="name"
            size="small"
            pagination={false}
          />
        </>
      ),
    },
    {
      key: 'yaml',
      label: 'YAML 源码',
      children: (
        <>
          <Alert
            message="直接编辑 YAML 配置文件"
            description="可以复制此 YAML 内容到配置文件中"
            type="info"
            style={{ marginBottom: 16 }}
          />
          <Input.TextArea
            value={yamlSource}
            onChange={(e) => setYamlSource(e.target.value)}
            rows={20}
            style={{ fontFamily: 'monospace', fontSize: 13 }}
          />
        </>
      ),
    },
  ];

  // 初始化
  useEffect(() => {
    loadAgents();
  }, []);

  return (
    <div className="agent-editor">
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <h2 style={{ margin: 0 }}>Agent 配置编辑器</h2>
        <Space>
          <Select
            style={{ width: 200 }}
            value={selectedAgent}
            onChange={(value) => {
              setSelectedAgent(value);
              loadAgent(value);
            }}
            options={agents.map((a) => ({ value: a, label: a }))}
          />
          <Button icon={<PlusOutlined />}>新建 Agent</Button>
        </Space>
      </div>

      <Form
        form={form}
        layout="vertical"
        onValuesChange={handleValuesChange}
        initialValues={config || {}}
      >
        <Card
          extra={
            <Space>
              <Button icon={<UndoOutlined />} onClick={handleReset}>
                重置
              </Button>
              <Button
                type="primary"
                icon={<SaveOutlined />}
                onClick={handleSave}
                loading={saving}
              >
                保存
              </Button>
              <Button icon={<PlayCircleOutlined />}>测试运行</Button>
            </Space>
          }
        >
          <Tabs items={tabItems} />
        </Card>
      </Form>
    </div>
  );
}