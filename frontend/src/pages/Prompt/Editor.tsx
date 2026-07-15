import { useState, useEffect, useCallback } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { Card, Form, Input, Select, Button, Space, message, Alert } from "antd";
import { SaveOutlined, ArrowLeftOutlined } from "@ant-design/icons";
import Editor from "@monaco-editor/react";
import { promptApi } from "../../api/prompt";

const categories = ['system', 'user', 'template', 'rag', 'agent'];

export default function PromptEditor() {
  const { key } = useParams<{ key: string }>();
  const navigate = useNavigate();
  const [form] = Form.useForm();
  const [content, setContent] = useState('');
  const [loading, setLoading] = useState(false);
  const isNew = key === 'new';

  const loadPrompt = useCallback(async () => {
    if (!key || isNew) return;
    setLoading(true);
    try {
      const res = await promptApi.getPrompt(key) as any;
      form.setFieldsValue({
        name: res?.name || '',
        category: res?.category || 'system',
        description: res?.description || '',
      });
      // Load active version content
      try {
        const versionRes = await promptApi.getActiveVersion(key) as any;
        setContent(versionRes?.content || '');
      } catch { setContent(''); }
    } catch { setContent(''); }
    finally { setLoading(false); }
  }, [key, isNew, form]);

  useEffect(() => { loadPrompt(); }, [loadPrompt]);

  const handleSave = async (values: any) => {
    setLoading(true);
    try {
      if (isNew) {
        await promptApi.createPrompt({
          key: values.key,
          name: values.name,
          description: values.description || '',
          category: values.category || 'system',
        });
        message.success('Prompt created');
      }
      // Save version with content
      const promptKey = isNew ? values.key : key;
      if (promptKey && content) {
        await promptApi.createVersion(promptKey, {
          version: `v1.${Date.now()}`,
          content,
          activate: true,
        });
        message.success('Version saved');
      }
      navigate('/prompt');
    } catch {
      message.error('Save failed');
    } finally { setLoading(false); }
  };

  return (
    <div>
      <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/prompt')} style={{ marginBottom: 16 }}>Back</Button>
      <h2>{isNew ? 'Create Prompt' : 'Edit Prompt'}</h2>
      <Card>
        <Form form={form} layout="vertical" onFinish={handleSave} initialValues={{ category: 'system' }}>
          {isNew && (
            <Form.Item name="key" label="Prompt Key" rules={[{ required: true }]}>
              <Input placeholder="e.g. greeting-template" />
            </Form.Item>
          )}
          <Form.Item name="name" label="Name" rules={[{ required: true }]}>
            <Input placeholder="Prompt name" />
          </Form.Item>
          <Form.Item name="category" label="Category" rules={[{ required: true }]}>
            <Select>{categories.map(c => <Select.Option key={c} value={c}>{c}</Select.Option>)}</Select>
          </Form.Item>
          <Form.Item name="description" label="Description">
            <Input.TextArea rows={2} placeholder="Describe this prompt's purpose" />
          </Form.Item>
        </Form>
        <div style={{ marginBottom: 8, fontWeight: 600 }}>Prompt Content</div>
        <Alert message="Use {{variable}} syntax for template variables" type="info" showIcon style={{ marginBottom: 12 }} />
        <div style={{ border: '1px solid #d9d9d9', borderRadius: 4, overflow: 'hidden' }}>
          <Editor height="400px" language="markdown" theme="vs" value={content} onChange={(v) => setContent(v || '')} options={{ minimap: { enabled: false }, wordWrap: 'on' }} />
        </div>
        <div style={{ marginTop: 16, textAlign: 'right' }}>
          <Space>
            <Button onClick={() => navigate('/prompt')}>Cancel</Button>
            <Button type="primary" icon={<SaveOutlined />} onClick={() => form.submit()} loading={loading}>Save</Button>
          </Space>
        </div>
      </Card>
    </div>
  );
}
