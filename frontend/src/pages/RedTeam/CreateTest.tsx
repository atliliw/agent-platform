import { useState, useEffect } from "react";
import { Card, Form, Input, Select, InputNumber, Switch, Button, Space, message, Checkbox, Alert } from "antd";
import { SecurityScanOutlined, ArrowLeftOutlined } from "@ant-design/icons";
import { useNavigate } from "react-router-dom";
import client from "../../api/client";
import { redteamApi } from "../../api/redteam";

const categoryOptions = [
  { value: "prompt_injection", label: "Prompt Injection" },
  { value: "jailbreak", label: "Jailbreak" },
  { value: "data_leak", label: "Data Leak" },
  { value: "harmful_content", label: "Harmful Content" },
  { value: "all", label: "All Categories" },
];

export default function CreateTestPage() {
  const navigate = useNavigate();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [agents, setAgents] = useState<{ id: string; name: string }[]>([]);

  useEffect(() => { loadAgents(); }, []);

  const loadAgents = async () => {
    try {
      const res = await client.get("/api/v2/agents") as any;
      setAgents(res?.agents || []);
    } catch {
      setAgents([
        { id: "chat-agent", name: "Chat Agent" },
        { id: "browser-agent", name: "Browser Agent" },
        { id: "research-agent", name: "Research Agent" },
      ]);
    }
  };

  const handleSubmit = async (values: any) => {
    setLoading(true);
    try {
      await redteamApi.createTest({
        name: values.name,
        description: values.description || "",
        agent_id: values.agent_id,
        model: values.model,
        category: values.category,
      });
      message.success("Test created successfully");
      navigate("/redteam");
    } catch {
      message.error("Failed to create test");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <h2 style={{ marginBottom: 24 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate("/redteam")} style={{ marginRight: 8 }} />
        <SecurityScanOutlined style={{ marginRight: 8 }} />
        Create Red Team Test
      </h2>
      <Alert message="Red team tests simulate attacker behavior to discover security vulnerabilities in your Agent systems" type="info" showIcon style={{ marginBottom: 24 }} />
      <Card>
        <Form form={form} layout="vertical" onFinish={handleSubmit} initialValues={{ category: "all", model: "qwen-plus" }}>
          <Form.Item name="name" label="Test Name" rules={[{ required: true }]}>
            <Input placeholder="e.g. Chat Agent Security Test" />
          </Form.Item>
          <Form.Item name="description" label="Description">
            <Input.TextArea rows={2} placeholder="Test description" />
          </Form.Item>
          <Form.Item name="agent_id" label="Target Agent" rules={[{ required: true }]}>
            <Select placeholder="Select agent to test">
              {agents.map((a) => <Select.Option key={a.id} value={a.id}>{a.name} ({a.id})</Select.Option>)}
            </Select>
          </Form.Item>
          <Form.Item name="model" label="Model" rules={[{ required: true }]}>
            <Select>
              {["qwen-turbo", "qwen-plus", "qwen-max", "gpt-4o", "gpt-3.5-turbo", "claude-3-5-sonnet-20241022"].map(m => <Select.Option key={m} value={m}>{m}</Select.Option>)}
            </Select>
          </Form.Item>
          <Form.Item name="category" label="Attack Category" rules={[{ required: true }]}>
            <Select>{categoryOptions.map(o => <Select.Option key={o.value} value={o.value}>{o.label}</Select.Option>)}</Select>
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit" loading={loading}>Create Test</Button>
              <Button onClick={() => navigate("/redteam")}>Cancel</Button>
            </Space>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}
