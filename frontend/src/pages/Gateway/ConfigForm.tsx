import { useEffect } from 'react';
import {
  Form,
  Input,
  Select,
  InputNumber,
  Slider,
  Switch,
  Space,
  Button,
  Divider,

  message,
} from 'antd';
import {
  PlusOutlined,
  MinusCircleOutlined,
} from '@ant-design/icons';
import type { GatewayConfigRequest as GatewayConfigRequestType } from '../../api/gateway';

// Use any to avoid type mismatch between form models (string[]) and API models (JSON string)
type GatewayConfigRequest = any;

interface ConfigFormProps {
  form: any;
  initialValues?: any;
  onSubmit: (values: any) => Promise<void>;
  onCancel: () => void;
  loading?: boolean;
}

const PROVIDER_DEFAULTS: Record<string, Partial<GatewayConfigRequest>> = {
  openai: {
    base_url: 'https://api.openai.com/v1',
    models: ['gpt-4o', 'gpt-4o-mini', 'gpt-4-turbo', 'gpt-3.5-turbo'],
    timeout: 30,
    retry_count: 3,
    rate_limit: 100,
  },
  anthropic: {
    base_url: 'https://api.anthropic.com',
    models: ['claude-3-5-sonnet-20241022', 'claude-3-5-haiku-20241022', 'claude-3-opus-20240229'],
    timeout: 60,
    retry_count: 3,
    rate_limit: 50,
  },
  dashscope: {
    base_url: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    models: ['qwen-turbo', 'qwen-plus', 'qwen-max', 'qwen-max-longcontext'],
    timeout: 30,
    retry_count: 3,
    rate_limit: 100,
  },
  custom: {
    base_url: '',
    models: [],
    timeout: 30,
    retry_count: 3,
    rate_limit: 100,
  },
};

export default function ConfigForm({ form, initialValues, onSubmit, onCancel, loading }: ConfigFormProps) {
  const providerType = Form.useWatch('provider', form);

  // Handle provider type change - fill in defaults
  useEffect(() => {
    if (providerType && !initialValues) {
      const defaults = PROVIDER_DEFAULTS[providerType] || {};
      form.setFieldsValue({
        base_url: defaults.base_url || '',
        models: defaults.models || [],
        timeout: defaults.timeout || 30,
        retry_count: defaults.retry_count || 3,
        rate_limit: defaults.rate_limit || 100,
      });
    }
  }, [providerType, form, initialValues]);

  const handleSubmit = async (values: GatewayConfigRequest) => {
    try {
      // Convert models string[] to JSON string for API, and apply camelCase field names
      const submitValues = {
        ...values,
        models: values.models ? JSON.stringify(values.models) : undefined,
      };
      await onSubmit(submitValues);
    } catch {
      message.error('保存失败');
    }
  };

  return (
    <Form
      form={form}
      layout="vertical"
      initialValues={{
        provider: 'openai',
        timeout: 30,
        retry_count: 3,
        rate_limit: 100,
        priority: 1,
        enabled: true,
        ...initialValues,
        // Mask API key for security - show placeholder
        api_key: initialValues ? '••••••••••••••••' : '',
      }}
      onFinish={handleSubmit}
    >
      <Form.Item
        name="name"
        label="配置名称"
        rules={[{ required: true, message: '请输入配置名称' }]}
      >
        <Input placeholder="例如：生产环境 OpenAI" />
      </Form.Item>

      <Form.Item
        name="provider"
        label="Provider 类型"
        rules={[{ required: true, message: '请选择 Provider 类型' }]}
      >
        <Select
          disabled={!!initialValues}
          options={[
            { value: 'openai', label: 'OpenAI' },
            { value: 'anthropic', label: 'Anthropic' },
            { value: 'dashscope', label: 'DashScope (阿里云)' },
            { value: 'custom', label: '自定义' },
          ]}
        />
      </Form.Item>

      <Form.Item
        name="api_key"
        label="API Key"
        rules={[{ required: !initialValues, message: '请输入 API Key' }]}
        extra={initialValues ? '留空保持原有 API Key 不变' : 'API Key 将加密存储'}
      >
        <Input.Password placeholder="sk-..." />
      </Form.Item>

      <Form.Item
        name="base_url"
        label="Base URL"
        tooltip="API 基础地址，可选填，使用默认值"
      >
        <Input placeholder="https://api.example.com/v1" />
      </Form.Item>

      <Divider>模型配置</Divider>

      <Form.List name="models">
        {(fields, { add, remove }) => (
          <>
            <div style={{ marginBottom: 8 }}>
              <Button type="dashed" onClick={() => add()} icon={<PlusOutlined />}>
                添加模型
              </Button>
            </div>
            {fields.map(({ key, name, ...restField }) => (
              <Space key={key} style={{ display: 'flex', marginBottom: 8 }} align="baseline">
                <Form.Item
                  {...restField}
                  name={name}
                  rules={[{ required: true, message: '请输入模型名称' }]}
                  style={{ flex: 1, marginBottom: 0 }}
                >
                  <Input placeholder="例如：gpt-4o" />
                </Form.Item>
                <MinusCircleOutlined onClick={() => remove(name)} />
              </Space>
            ))}
          </>
        )}
      </Form.List>

      <Divider>高级配置</Divider>

      <div style={{ display: 'flex', gap: 16 }}>
        <Form.Item
          name="rate_limit"
          label="Rate Limit (请求/分钟)"
          style={{ flex: 1 }}
          tooltip="每分钟最大请求数"
        >
          <InputNumber min={1} max={1000} style={{ width: '100%' }} />
        </Form.Item>

        <Form.Item
          name="timeout"
          label="Timeout (秒)"
          style={{ flex: 1 }}
          tooltip="请求超时时间"
        >
          <InputNumber min={5} max={300} style={{ width: '100%' }} />
        </Form.Item>

        <Form.Item
          name="retry_count"
          label="重试次数"
          style={{ flex: 1 }}
          tooltip="失败后的重试次数"
        >
          <InputNumber min={0} max={5} style={{ width: '100%' }} />
        </Form.Item>
      </div>

      <Form.Item
        name="priority"
        label="优先级"
        tooltip="用于 Fallback 排序，数值越大优先级越高"
      >
        <Slider
          min={1}
          max={10}
          marks={{ 1: '低', 5: '中', 10: '高' }}
        />
      </Form.Item>

      <Form.Item
        name="enabled"
        label="启用状态"
        valuePropName="checked"
      >
        <Switch checkedChildren="启用" unCheckedChildren="禁用" />
      </Form.Item>

      <Form.Item style={{ marginBottom: 0, marginTop: 24 }}>
        <Space>
          <Button type="primary" htmlType="submit" loading={loading}>
            {initialValues ? '更新' : '创建'}
          </Button>
          <Button onClick={onCancel}>取消</Button>
        </Space>
      </Form.Item>
    </Form>
  );
}
