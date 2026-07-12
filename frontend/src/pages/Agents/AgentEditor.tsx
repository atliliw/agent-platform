import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
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
  Typography,
} from 'antd';
import {
  SaveOutlined,
  UndoOutlined,
  PlayCircleOutlined,
  PlusOutlined,
  DeleteOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import yaml from 'js-yaml';
import { agentApi } from '../../api/agent';
import type { Agent } from '../../api/agent';
import { skillApi } from '../../api/skill';
import type { Skill } from '../../api/skill';

// Agent 配置类型
interface AgentConfig {
  name: string;
  model: string;
  system_prompt: string;
  max_steps?: number;
  timeout?: number;
  temperature?: number;
  tools?: string[];
  skills?: string[];
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

export default function AgentEditor({ focusAgentId }: { focusAgentId?: string }) {
  const navigate = useNavigate();
  // Agent 列表
  const [agents, setAgents] = useState<string[]>([]);
  const [agentIdMap, setAgentIdMap] = useState<Record<string, string>>({});
  const [selectedAgent, setSelectedAgent] = useState<string>('');
  const [config, setConfig] = useState<AgentConfig | null>(null);
  const [originalConfig, setOriginalConfig] = useState<AgentConfig | null>(null);
  // Raw agent from getAgent, kept so handleSave can preserve fields the editor
  // doesn't expose (handoffs, prompt_template_key). RegisterAgent is a full
  // upsert, so omitting them would wipe them on every save.
  const [originalAgent, setOriginalAgent] = useState<Agent | null>(null);
  const [yamlSource, setYamlSource] = useState('');
  const [, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  // 可挂载的技能列表（从技能库加载）
  const [availableSkills, setAvailableSkills] = useState<Skill[]>([]);

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

  // 加载可用技能列表
  const loadAvailableSkills = async () => {
    try {
      const resp = await skillApi.listSkills();
      setAvailableSkills(resp?.skills || []);
    } catch (error) {
      console.error('加载技能列表失败', error);
      setAvailableSkills([]);
    }
  };

  // 加载 Agent 列表
  const loadAgents = async () => {
    try {
      const response = await agentApi.listAgents();
      const agentList = response?.agents || [];
      const nameToId: Record<string, string> = {};
      const names = agentList.map((a: Agent) => {
        const name = a.name || a.id;
        nameToId[name] = a.id;
        return name;
      });
      setAgents(names);
      setAgentIdMap(nameToId);
      if (names.length > 0) {
        // Prefer the agent passed in via focusAgentId (user clicked "编辑" on a
        // list row); fall back to the first agent otherwise.
        const focusName = focusAgentId
          ? Object.keys(nameToId).find((n) => nameToId[n] === focusAgentId)
          : undefined;
        const first = focusName || names[0];
        setSelectedAgent(first);
        // nameToId 是本次 listAgents 刚建好的本地 map，可立即用；不能依赖
        // agentIdMap state（尚未更新）。
        loadAgent(nameToId[first]);
      }
    } catch (error) {
      console.error('加载 Agent 列表失败', error);
      setAgents([]);
      setAgentIdMap({});
    }
  };

  // 加载 Agent 配置。直接接收 agent ID，不依赖 agentIdMap state —— 否则
  // loadAgents 在 setAgentIdMap 后立刻调用本函数时，闭包里的 agentIdMap 仍是
  // 旧值（空），agentId 解析为 undefined 直接早退，setFieldsValue 不执行，
  // 表单除顶部下拉框的 name 外全部不回填。
  const loadAgent = async (agentId: string) => {
    if (!agentId) {
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const response = await agentApi.getAgent(agentId);
      const agent = response?.agent;
      if (agent) {
        setOriginalAgent(agent);
        const agentConfig: AgentConfig = {
          name: agent.name,
          model: agent.model || 'qwen-plus',
          system_prompt: agent.instructions || '',
          max_steps: agent.max_tokens ? Math.floor(agent.max_tokens / 1000) : undefined,
          temperature: agent.temperature,
          tools: agent.tools,
          skills: agent.skills || [],
          description: agent.description,
        };
        setConfig(agentConfig);
        setOriginalConfig(agentConfig);
        setYamlSource(yaml.dump(agentConfig, { indent: 2 }));
        form.setFieldsValue(agentConfig);
      }
    } catch (error) {
      console.error('加载 Agent 配置失败', error);
    } finally {
      setLoading(false);
    }
  };

  // 保存配置
  const handleSave = async () => {
    try {
      const values = await form.validateFields();
      setSaving(true);
      const newConfig = { ...config, ...values } as AgentConfig;

      const agentId = agentIdMap[selectedAgent];
      if (agentId) {
        await agentApi.registerAgent({
          id: agentId,
          name: newConfig.name,
          description: newConfig.description || '',
          instructions: newConfig.system_prompt,
          tools: newConfig.tools || [],
          skills: newConfig.skills || [],
          model: newConfig.model,
          temperature: newConfig.temperature ?? 0.7,
          max_tokens: (newConfig.max_steps || 10) * 1000,
          // Preserve fields the editor doesn't expose. RegisterAgent is a full
          // upsert (agent_service.go:135), so hardcoding [] here used to wipe
          // an existing agent's handoffs and drop its prompt_template_key.
          handoffs: originalAgent?.handoffs || [],
          prompt_template_key: originalAgent?.prompt_template_key || '',
        });
        message.success('配置已保存');
      } else {
        message.info('配置已更新到本地');
      }

      setConfig(newConfig);
      setOriginalConfig(newConfig);
      setYamlSource(yaml.dump(newConfig, { indent: 2 }));
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

  // 已选技能的完整详情，用于在「技能挂载」tab 预览（挂之前知道每个 skill 干嘛）。
  // config.skills 由 handleValuesChange 与 loadAgent 同步。
  const selectedSkillDetails: Skill[] = (config?.skills || [])
    .map((id) => availableSkills.find((s) => s.id === id))
    .filter((s): s is Skill => !!s);

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
      key: 'skills',
      label: '技能挂载',
      children: (
        <>
          <Alert
            message="渐进式披露"
            description="仅勾选技能的 Name + Description 注入提示词（低成本常驻）；Agent 需要使用时通过 load_skill 工具按需加载完整 Instructions。可多选。"
            type="info"
            style={{ marginBottom: 16 }}
          />
          <Row gutter={16} align="bottom">
            <Col flex="auto">
              <Form.Item name="skills" label="已挂载技能">
                <Select
                  mode="multiple"
                  placeholder="选择要挂载的技能"
                  allowClear
                  optionFilterProp="label"
                  options={availableSkills.map((s) => ({
                    value: s.id,
                    label: `${s.name} - ${s.description}`,
                  }))}
                />
              </Form.Item>
            </Col>
            <Col flex="none">
              <Form.Item label=" " colon={false}>
                <Button icon={<ThunderboltOutlined />} onClick={() => navigate('/skills')}>
                  管理技能
                </Button>
              </Form.Item>
            </Col>
          </Row>
          {selectedSkillDetails.length > 0 && (
            <Card size="small" title="已选技能详情" style={{ marginTop: 8 }}>
              {selectedSkillDetails.map((sk) => (
                <div
                  key={sk.id}
                  style={{ marginBottom: 12, paddingBottom: 12, borderBottom: '1px solid #f0f0f0' }}
                >
                  <Space size={6} wrap>
                    <Tag color="geekblue">{sk.name}</Tag>
                    {sk.status !== 'active' && <Tag color="default">{sk.status}</Tag>}
                    {sk.tools?.map((t) => (
                      <Tag key={t} color="green">{t}</Tag>
                    ))}
                  </Space>
                  <div style={{ color: '#666', marginTop: 6 }}>{sk.description}</div>
                  <Typography.Paragraph
                    type="secondary"
                    ellipsis={{ rows: 3, expandable: true, symbol: '展开' }}
                    style={{ marginTop: 6, marginBottom: 0, fontFamily: 'monospace', fontSize: 12, whiteSpace: 'pre-wrap' }}
                  >
                    {sk.instructions}
                  </Typography.Paragraph>
                </div>
              ))}
            </Card>
          )}
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
    loadAvailableSkills();
  }, []);

  // 当用户在列表点「编辑」跳过来时（focusAgentId 变化），切到该 agent。
  // 初次加载由 loadAgents 内部处理 focusAgentId；这里处理编辑器已挂载后的后续切换。
  useEffect(() => {
    if (!focusAgentId || Object.keys(agentIdMap).length === 0) return;
    const entry = Object.entries(agentIdMap).find(([, id]) => id === focusAgentId);
    if (entry && entry[0] !== selectedAgent) {
      setSelectedAgent(entry[0]);
      // focusAgentId 本身就是 agent ID，直接传入。
      loadAgent(focusAgentId);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [focusAgentId]);

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
              loadAgent(agentIdMap[value]);
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