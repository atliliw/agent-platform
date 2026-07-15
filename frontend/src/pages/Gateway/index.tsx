import { useState, useEffect, useCallback } from "react";
import { Card, Tabs, Table, Tag, Button, Space, message, Modal, Badge, Popconfirm, Alert, Input, Select, Drawer, Descriptions, Form, InputNumber, Tooltip } from "antd";
import { PlusOutlined, ApiOutlined, BarChartOutlined, EditOutlined, DeleteOutlined, ThunderboltOutlined, ApartmentOutlined, InfoCircleOutlined } from "@ant-design/icons";
import ConfigForm from "./ConfigForm";
import GatewayStats from "./Stats";
import { gatewayApi, type GatewayProvider, type GatewayConfigRequest, type GatewayRoute } from "../../api/gateway";

// Parse models JSON string for display - handles both string[] and ModelConfig object arrays
const parseModels = (modelsStr: string): string[] => {
  if (!modelsStr) return [];
  try {
    const parsed = JSON.parse(modelsStr);
    if (!Array.isArray(parsed)) return [];
    // Handle ModelConfig objects: [{model_id, model_name, ...}] or string arrays: ["gpt-4o"]
    return parsed.map((item: any) => {
      if (typeof item === 'string') return item;
      if (item && typeof item === 'object') return item.model_id || item.modelName || item.name || '';
      return '';
    }).filter(Boolean);
  } catch {
    return [];
  }
};

// Parse models JSON to get ModelConfig details (prices, max_tokens)
const parseModelDetails = (modelsStr: string): any[] => {
  if (!modelsStr) return [];
  try {
    const parsed = JSON.parse(modelsStr);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((item: any) => typeof item === 'object' && item.model_id);
  } catch {
    return [];
  }
};

const LOAD_BALANCE_STRATEGIES = [
  { value: 'least_latency', label: 'Least Latency', description: 'Route to provider with lowest average latency' },
  { value: 'round_robin', label: 'Round Robin', description: 'Distribute requests evenly across providers' },
  { value: 'least_cost', label: 'Least Cost', description: 'Route to cheapest available provider' },
  { value: 'weighted_random', label: 'Weighted Random', description: 'Random selection weighted by priority' },
];

export default function GatewayPage() {
  const [activeTab, setActiveTab] = useState("providers");
  const [providers, setProviders] = useState<any[]>([]);
  const [routes, setRoutes] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingProvider, setEditingProvider] = useState<GatewayProvider | null>(null);
  const [form] = Form.useForm();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [selectedProvider, setSelectedProvider] = useState<GatewayProvider | null>(null);
  const [ruleModalOpen, setRuleModalOpen] = useState(false);
  const [ruleForm] = Form.useForm();
  const [strategy, setStrategy] = useState("least_latency");
  const [strategyLoading, setStrategyLoading] = useState(false);

  const loadProviders = useCallback(async () => {
    setLoading(true);
    try {
      const res = (await gatewayApi.listGatewayConfigs()) as any;
      const list = res?.configs || res?.providers || [];
      setProviders(Array.isArray(list) ? list : []);
    } catch (e) {
      console.error("Failed to load providers:", e);
      message.error("Failed to load providers");
      setProviders([]);
    } finally {
      setLoading(false);
    }
  }, []);

  const loadRoutes = useCallback(async () => {
    try {
      const res = (await gatewayApi.listRoutingRules()) as any;
      const list = res?.routes || res?.rules || [];
      setRoutes(Array.isArray(list) ? list : []);
    } catch (e) {
      console.error("Failed to load routes:", e);
      setRoutes([]);
    }
  }, []);

  useEffect(() => { loadProviders(); loadRoutes(); }, [loadProviders, loadRoutes]);

  const handleCreate = async (values: GatewayConfigRequest) => {
    try {
      await gatewayApi.createGatewayConfig(values);
      message.success("Provider created");
      setModalOpen(false);
      form.resetFields();
      loadProviders();
    } catch (e) {
      message.error("Failed to create provider");
    }
  };

  const handleUpdate = async (values: GatewayConfigRequest) => {
    if (!editingProvider) return;
    try {
      await gatewayApi.updateGatewayConfig(editingProvider.id, values);
      message.success("Provider updated");
      setModalOpen(false);
      setEditingProvider(null);
      form.resetFields();
      loadProviders();
    } catch (e) {
      message.error("Failed to update provider");
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await gatewayApi.deleteGatewayConfig(id);
      message.success("Provider deleted");
      loadProviders();
    } catch (e) {
      message.error("Failed to delete provider");
    }
  };

  const handleToggleProvider = async (record: GatewayProvider) => {
    try {
      // Merge existing config data with toggled enabled field
      const fullConfig: GatewayConfigRequest = {
        name: record.name,
        provider: record.provider,
        api_key: '',  // Don't send API key back; leave empty to keep existing
        base_url: record.base_url,
        models: record.models,
        rate_limit: record.rate_limit,
        timeout: record.timeout,
        retry_count: record.retry_count,
        priority: record.priority,
        enabled: !record.enabled,
        description: record.description,
        tenant_id: record.tenant_id,
      };
      await gatewayApi.toggleProvider(record.id, !record.enabled, fullConfig);
      message.success(`Provider ${!record.enabled ? 'enabled' : 'disabled'}`);
      loadProviders();
    } catch (e) {
      message.error("Failed to toggle provider");
    }
  };

  const handleCreateRule = async (values: any) => {
    try {
      // Convert fallbacks string to JSON array if provided
      const submitValues = {
        ...values,
        fallbacks: values.fallbacks ? JSON.stringify(JSON.parse(values.fallbacks)) : undefined,
      };
      await gatewayApi.createRoutingRule(submitValues);
      message.success("Route created");
      setRuleModalOpen(false);
      ruleForm.resetFields();
      loadRoutes();
    } catch (e) {
      message.error("Failed to create route");
    }
  };

  const handleDeleteRule = async (id: string) => {
    try {
      await gatewayApi.deleteRoutingRule(id);
      message.success("Route deleted");
      loadRoutes();
    } catch (e) {
      message.error("Failed to delete route");
    }
  };

  const handleSetStrategy = async () => {
    setStrategyLoading(true);
    try {
      await gatewayApi.setLoadBalanceStrategy(strategy);
      message.success(`Load balance strategy set to: ${LOAD_BALANCE_STRATEGIES.find(s => s.value === strategy)?.label}`);
    } catch (e) {
      message.error("Failed to set strategy");
    } finally {
      setStrategyLoading(false);
    }
  };

  const providerColumns = [
    { title: "Name", dataIndex: "name", key: "name", render: (n: string) => <strong>{n}</strong> },
    { title: "Type", dataIndex: "provider", key: "provider", render: (t: string) => <Tag color={t === "openai" ? "green" : t === "anthropic" ? "blue" : t === "dashscope" ? "orange" : "default"}>{t}</Tag> },
    { title: "Models", dataIndex: "models", key: "models", render: (m: string) => {
      const arr = parseModels(m);
      if (arr.length === 0) return <Tag>0 models</Tag>;
      if (arr.length <= 3) return <Space size={4}>{arr.map(name => <Tag key={name}>{name}</Tag>)}</Space>;
      return <Tooltip title={arr.join(', ')}><Tag>{arr.length} models</Tag></Tooltip>;
    }},
    { title: "Priority", dataIndex: "priority", key: "priority", render: (p: number) => <Badge count={p || 0} style={{ backgroundColor: "#1677ff" }} /> },
    { title: "Rate Limit", dataIndex: "rate_limit", key: "rate_limit", render: (r: number) => `${r || 0}/min` },
    { title: "Status", dataIndex: "enabled", key: "enabled", render: (e: boolean, record: GatewayProvider) => (
      <Space>
        <Badge status={e ? "success" : "default"} text={e ? "Enabled" : "Disabled"} />
        {!record.api_key && !e && <Tooltip title="Add API key to enable"><Tag color="warning">No API Key</Tag></Tooltip>}
        <Button size="small" type="link" onClick={() => handleToggleProvider(record)} disabled={!record.api_key && !e}>
          {e ? 'Disable' : 'Enable'}
        </Button>
      </Space>
    ) },
    { title: "Actions", key: "actions", render: (_: unknown, record: GatewayProvider) => (
      <Space>
        <Button size="small" icon={<EditOutlined />} onClick={() => {
          setEditingProvider(record);
          setModalOpen(true);
          // Parse models JSON for form display
          const modelNames = parseModels(record.models);
          form.setFieldsValue({
            name: record.name,
            provider: record.provider,
            api_key: '••••••••••••••••',
            base_url: record.base_url,
            models: modelNames,
            rate_limit: record.rate_limit,
            timeout: record.timeout,
            retry_count: record.retry_count,
            priority: record.priority,
            enabled: record.enabled,
            description: record.description,
          });
        }} />
        <Button size="small" icon={<ApiOutlined />} onClick={() => { setSelectedProvider(record); setDrawerOpen(true); }} />
        <Popconfirm title="Delete this provider?" onConfirm={() => handleDelete(record.id)}><Button size="small" danger icon={<DeleteOutlined />} /></Popconfirm>
      </Space>
    ) },
  ];

  const routeColumns = [
    { title: "Name", dataIndex: "name", key: "name", render: (n: string) => <strong>{n}</strong> },
    { title: "Pattern", dataIndex: "pattern", key: "pattern", ellipsis: true, render: (p: string) => <code>{p}</code> },
    { title: "Model", dataIndex: "model_id", key: "model_id", render: (m: string) => <Tag color="blue">{m}</Tag> },
    { title: "Fallbacks", dataIndex: "fallbacks", key: "fallbacks", render: (f: string) => {
      if (!f) return <Tag>none</Tag>;
      try {
        const arr = JSON.parse(f);
        if (!Array.isArray(arr) || arr.length === 0) return <Tag>none</Tag>;
        return <Space size={4}>{arr.map((m: string) => <Tag key={m}>{m}</Tag>)}</Space>;
      } catch { return <Tag>{f}</Tag>; }
    }},
    { title: "Status", dataIndex: "enabled", key: "enabled", render: (e: boolean) => <Tag color={e ? "green" : "default"}>{e ? "Active" : "Inactive"}</Tag> },
    { title: "Actions", key: "actions", render: (_: unknown, record: GatewayRoute) => (
      <Space>
        <Popconfirm title="Delete this route?" onConfirm={() => handleDeleteRule(record.id)}><Button size="small" danger icon={<DeleteOutlined />} /></Popconfirm>
      </Space>
    ) },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 24 }}>LLM Gateway</h2>
      <Card>
        <Tabs activeKey={activeTab} onChange={setActiveTab} items={[
          { key: "providers", label: <span><ApiOutlined /> Providers</span>, children: (
            <div>
              <Alert message="LLM Gateway provides unified API access with fallback, rate limiting, and cost optimization. Seed providers are pre-configured — add your API key to enable them." type="info" showIcon style={{ marginBottom: 16 }} />
              <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditingProvider(null); setModalOpen(true); }} style={{ marginBottom: 16 }}>Add Provider</Button>
              <Table columns={providerColumns} dataSource={providers} rowKey="id" loading={loading} pagination={false} />
            </div>
          ) },
          { key: "routes", label: <span><ApartmentOutlined /> Routing</span>, children: (
            <div>
              <Alert message="Routing rules determine which model and provider to use based on request patterns. Fallback models are used when the primary is unavailable." type="info" showIcon style={{ marginBottom: 16 }} />
              <Button type="primary" icon={<PlusOutlined />} onClick={() => setRuleModalOpen(true)} style={{ marginBottom: 16 }}>Add Route</Button>
              <Table columns={routeColumns} dataSource={routes} rowKey="id" loading={loading} pagination={false} />
            </div>
          ) },
          { key: "strategy", label: <span><ThunderboltOutlined /> Strategy</span>, children: (
            <div>
              <Alert message="The load balance strategy determines how the gateway selects a provider when no specific routing rule matches." type="info" showIcon style={{ marginBottom: 16 }} />
              <Card title="Load Balance Strategy" style={{ marginBottom: 16 }}>
                <Form layout="vertical">
                  <Form.Item label="Strategy">
                    <Select value={strategy} onChange={setStrategy} style={{ width: 300 }}>
                      {LOAD_BALANCE_STRATEGIES.map(s => (
                        <Select.Option key={s.value} value={s.value}>
                          <Tooltip title={s.description}>{s.label} <InfoCircleOutlined style={{ marginLeft: 8, opacity: 0.5 }} /></Tooltip>
                        </Select.Option>
                      ))}
                    </Select>
                  </Form.Item>
                  <Form.Item>
                    <Button type="primary" loading={strategyLoading} onClick={handleSetStrategy}>Apply Strategy</Button>
                  </Form.Item>
                </Form>
                <Descriptions bordered column={1} size="small" style={{ marginTop: 16 }}>
                  {LOAD_BALANCE_STRATEGIES.map(s => (
                    <Descriptions.Item key={s.value} label={s.label}>{s.description}</Descriptions.Item>
                  ))}
                </Descriptions>
              </Card>
            </div>
          ) },
          { key: "stats", label: <span><BarChartOutlined /> Statistics</span>, children: <GatewayStats /> },
        ]} />
      </Card>
      <Modal title={editingProvider ? "Edit Provider" : "Add Provider"} open={modalOpen} onCancel={() => { setModalOpen(false); setEditingProvider(null); form.resetFields(); }} footer={null} width={600}>
        <ConfigForm form={form} initialValues={editingProvider} onSubmit={editingProvider ? handleUpdate : handleCreate} onCancel={() => { setModalOpen(false); setEditingProvider(null); }} loading={loading} />
      </Modal>
      <Drawer title="Provider Details" placement="right" width={500} open={drawerOpen} onClose={() => setDrawerOpen(false)}>
        {selectedProvider && (
          <Descriptions bordered column={1}>
            <Descriptions.Item label="ID">{selectedProvider.id}</Descriptions.Item>
            <Descriptions.Item label="Name">{selectedProvider.name}</Descriptions.Item>
            <Descriptions.Item label="Type"><Tag color="blue">{selectedProvider.provider}</Tag></Descriptions.Item>
            <Descriptions.Item label="Description">{selectedProvider.description || '-'}</Descriptions.Item>
            <Descriptions.Item label="Base URL">{selectedProvider.base_url || '-'}</Descriptions.Item>
            <Descriptions.Item label="Models">{parseModels(selectedProvider.models).map(m => <Tag key={m}>{m}</Tag>)}</Descriptions.Item>
            <Descriptions.Item label="Rate Limit">{selectedProvider.rate_limit}/min</Descriptions.Item>
            <Descriptions.Item label="Timeout">{selectedProvider.timeout}s</Descriptions.Item>
            <Descriptions.Item label="Retry Count">{selectedProvider.retry_count}</Descriptions.Item>
            <Descriptions.Item label="Priority">{selectedProvider.priority}</Descriptions.Item>
            <Descriptions.Item label="API Key">{selectedProvider.api_key ? '✓ Configured' : '✗ Not set — provider is disabled'}</Descriptions.Item>
            <Descriptions.Item label="Status">
              <Badge status={selectedProvider.enabled ? "success" : "default"} text={selectedProvider.enabled ? "Enabled" : "Disabled"} />
            </Descriptions.Item>
            {/* Show model pricing details if available */}
            {parseModelDetails(selectedProvider.models).length > 0 && (
              <Descriptions.Item label="Pricing">
                <Table size="small" pagination={false} dataSource={parseModelDetails(selectedProvider.models)} rowKey="model_id" columns={[
                  { title: 'Model', dataIndex: 'model_name', key: 'model_name' },
                  { title: 'Input $/1M', dataIndex: 'input_price', key: 'input_price', render: (v: number) => `$${v}` },
                  { title: 'Output $/1M', dataIndex: 'output_price', key: 'output_price', render: (v: number) => `$${v}` },
                  { title: 'Max Tokens', dataIndex: 'max_tokens', key: 'max_tokens', render: (v: number) => v >= 1000 ? `${v/1000}K` : v },
                ]} />
              </Descriptions.Item>
            )}
          </Descriptions>
        )}
      </Drawer>
      <Modal title="Add Routing Rule" open={ruleModalOpen} onCancel={() => { setRuleModalOpen(false); ruleForm.resetFields(); }} onOk={() => ruleForm.submit()}>
        <Form form={ruleForm} layout="vertical" onFinish={handleCreateRule}>
          <Form.Item name="name" label="Rule Name" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="pattern" label="Routing Pattern" rules={[{ required: true }]}><Input placeholder="e.g. quality-sensitive, cost-sensitive, default" /></Form.Item>
          <Form.Item name="model_id" label="Primary Model" rules={[{ required: true }]}><Input placeholder="e.g. qwen-plus, gpt-4o" /></Form.Item>
          <Form.Item name="fallbacks" label="Fallback Models (JSON)"><Input.TextArea rows={2} placeholder='["qwen-turbo", "gpt-4o-mini"]' /></Form.Item>
          <Form.Item name="priority" label="Priority" initialValue={1}><InputNumber min={1} max={10} /></Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
