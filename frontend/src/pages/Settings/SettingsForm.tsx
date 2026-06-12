import { Card, Form, Input, Select, Button, message } from 'antd';
import { useSettingsStore } from '../../stores';

export default function SettingsForm() {
  const { apiBaseUrl, theme, defaultModel, language, updateSettings } = useSettingsStore();

  const handleSave = (values: Record<string, string>) => {
    updateSettings(values);
    message.success('设置已保存');
  };

  return (
    <Card>
      <Form
        layout="vertical"
        initialValues={{ apiBaseUrl, theme, defaultModel, language }}
        onFinish={handleSave}
      >
        <Form.Item name="apiBaseUrl" label="API 地址">
          <Input placeholder="http://192.168.10.100:9000" />
        </Form.Item>

        <Form.Item name="defaultModel" label="默认模型">
          <Select>
            <Select.Option value="qwen3.7-max-2026-05-17">Qwen3.7 Max</Select.Option>
            <Select.Option value="gpt-4">GPT-4</Select.Option>
            <Select.Option value="gpt-3.5-turbo">GPT-3.5 Turbo</Select.Option>
          </Select>
        </Form.Item>

        <Form.Item name="theme" label="主题">
          <Select>
            <Select.Option value="light">浅色</Select.Option>
            <Select.Option value="dark">深色</Select.Option>
          </Select>
        </Form.Item>

        <Form.Item name="language" label="语言">
          <Select>
            <Select.Option value="zh-CN">中文</Select.Option>
            <Select.Option value="en-US">英文</Select.Option>
          </Select>
        </Form.Item>

        <Form.Item>
          <Button type="primary" htmlType="submit">
            保存设置
          </Button>
        </Form.Item>
      </Form>
    </Card>
  );
}