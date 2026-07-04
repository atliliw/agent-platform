import { useState, useEffect, useCallback } from "react";
import { Card, Tabs, Table, Tag, Button, Space, message, Modal, Badge, Popconfirm, Alert, Input, Select, Drawer, Descriptions, Form, InputNumber } from "antd";
import { PlusOutlined, ApiOutlined, BarChartOutlined, EditOutlined, DeleteOutlined, ThunderboltOutlined, ApartmentOutlined } from "@ant-design/icons";
import ConfigForm from "./ConfigForm";
import GatewayStats from "./Stats";
import { gatewayApi, type GatewayProvider, type GatewayConfigRequest, type GatewayRoute } from "../../api/gateway";

// Parse models JSON string for display - handles both string[] and ModelConfig object arrays
const parseModels = (modelsStr: string): string[] => {
  if (!modelsStr) return [];
  try {
    const parsed = JSON.parse(modelsStr);
    if (!Array.isArray(parsed)) return [];
    // Handle ModelConfig objects: [{modelId, modelName, ...}] or string arrays: ["gpt-4o"]
    return parsed.map((item: any) => {
      if (typeof item === 'string') return item;
      if (item && typeof item === 'object') return item.model_id || item.modelName || item.name || '';
      return '';
    }).filter(Boolean);
  } catch {
    return [];
  }
};

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

  const providerColumns = [
    { title: "Name", dataIndex: "name", key: "name" },
    { title: "Type", dataIndex: "provider", key: "provider", render: (t: string) => <Tag color={t === "openai" ? "green" : t === "anthropic" ? "blue" : "orange"}>{t}</Tag> },
    { title: "Models", dataIndex: "models", key: "models", render: (m: string) => { const arr = parseModels(m); return <Tag>{arr.length} models</Tag>; } },
    { title: "Priority", dataIndex: "priority", key: "priority", render: (p: number) => <Badge count={p || 0} style={{ backgroundColor: "#1677ff" }} /> },
    { title: "Status", dataIndex: "enabled", key: "enabled", render: (e: boolean, record: GatewayProvider) => (
      <Space>
        <Badge status={e ? "success" : "default"} text={e ? "Enabled" : "Disabled"} />
        <Button size="small" type="link" onClick={() => handleToggleProvider(record)}>
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
        <Popconfirm title="Delete?" onConfirm={() => handleDelete(record.id)}><Button size="small" danger icon={<DeleteOutlined />} /></Popconfirm>
      </Space>
    ) },
  ];

  const routeColumns = [
    { title: "Name", dataIndex: "name", key: "name" },
    { title: "Pattern", dataIndex: "pattern", key: "pattern", ellipsis: true, render: (p: string) => <code>{p}</code> },
    { title: "Model", dataIndex: "model_id", key: "model_id", render: (m: string) => <Tag>{m}</Tag> },
    { title: "Status", dataIndex: "enabled", key: "enabled", render: (e: boolean) => <Tag color={e ? "green" : "default"}>{e ? "Active" : "Inactive"}</Tag> },
    { title: "Actions", key: "actions", render: (_: unknown, record: GatewayRoute) => (
      <Space>
        <Popconfirm title="Delete?" onConfirm={() => handleDeleteRule(record.id)}><Button size="small" danger icon={<DeleteOutlined />} /></Popconfirm>
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
              <Alert message="LLM Gateway provides unified API access with fallback, rate limiting, and cost optimization" type="info" showIcon style={{ marginBottom: 16 }} />
              <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditingProvider(null); setModalOpen(true); }} style={{ marginBottom: 16 }}>Add Provider</Button>
              <Table columns={providerColumns} dataSource={providers} rowKey="id" loading={loading} pagination={false} />
            </div>
          ) },
          { key: "routes", label: <span><ApartmentOutlined /> Routing</span>, children: (
            <div>
              <Alert message="Routing rules determine which model and provider to use based on request patterns" type="info" showIcon style={{ marginBottom: 16 }} />
              <Button type="primary" icon={<PlusOutlined />} onClick={() => setRuleModalOpen(true)} style={{ marginBottom: 16 }}>Add Route</Button>
              <Table columns={routeColumns} dataSource={routes} rowKey="id" loading={loading} pagination={false} />
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
            <Descriptions.Item label="Base URL">{selectedProvider.base_url}</Descriptions.Item>
            <Descriptions.Item label="Models">{parseModels(selectedProvider.models).map(m => <Tag key={m}>{m}</Tag>)}</Descriptions.Item>
            <Descriptions.Item label="Rate Limit">{selectedProvider.rate_limit}/min</Descriptions.Item>
            <Descriptions.Item label="Timeout">{selectedProvider.timeout}s</Descriptions.Item>
            <Descriptions.Item label="Priority">{selectedProvider.priority}</Descriptions.Item>
          </Descriptions>
        )}
      </Drawer>
      <Modal title="Add Routing Rule" open={ruleModalOpen} onCancel={() => { setRuleModalOpen(false); ruleForm.resetFields(); }} onOk={() => ruleForm.submit()}>
        <Form form={ruleForm} layout="vertical" onFinish={handleCreateRule}>
          <Form.Item name="name" label="Rule Name" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="pattern" label="Routing Pattern" rules={[{ required: true }]}><Input placeholder="e.g. gpt-4*" /></Form.Item>
          <Form.Item name="model_id" label="Primary Model" rules={[{ required: true }]}><Input placeholder="e.g. gpt-4o" /></Form.Item>
          <Form.Item name="fallbacks" label="Fallback Models (JSON)"><Input.TextArea rows={2} placeholder='["gpt-3.5-turbo", "qwen-plus"]' /></Form.Item>
          <Form.Item name="priority" label="Priority" initialValue={1}><InputNumber min={1} max={10} /></Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
