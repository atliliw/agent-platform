import { useState } from 'react';
import { Row, Col, Card, Select, Slider, InputNumber, Input, Button, Space, Tag, message, Tabs } from 'antd';
import { PlayCircleOutlined, ThunderboltOutlined, ClearOutlined } from '@ant-design/icons';
import { playgroundApi, playgroundHelpers } from '../../api/playground';

const { TextArea } = Input;

const availableModels = [
  { id: 'qwen-turbo', name: 'Qwen Turbo', provider: 'Alibaba' },
  { id: 'qwen-plus', name: 'Qwen Plus', provider: 'Alibaba' },
  { id: 'qwen-max', name: 'Qwen Max', provider: 'Alibaba' },
  { id: 'gpt-4o', name: 'GPT-4o', provider: 'OpenAI' },
  { id: 'gpt-4o-mini', name: 'GPT-4o Mini', provider: 'OpenAI' },
  { id: 'claude-3-5-sonnet-20241022', name: 'Claude 3.5 Sonnet', provider: 'Anthropic' },
];

const toNum = playgroundHelpers.toNumber;

export default function PlaygroundPage() {
  const [model, setModel] = useState('qwen-plus');
  const [temperature, setTemperature] = useState(0.7);
  const [maxTokens, setMaxTokens] = useState(2048);
  const [systemPrompt, setSystemPrompt] = useState('You are a helpful assistant.');
  const [userMessage, setUserMessage] = useState('');
  const [result, setResult] = useState('');
  const [isLoading, setIsLoading] = useState(false);

  // Compare models state
  const [compareModelsState, setCompareModelsState] = useState<string[]>(['qwen-plus', 'gpt-4o']);
  const [compareResults, setCompareResults] = useState<any[]>([]);
  const [isComparing, setIsComparing] = useState(false);
  const [comparison, setComparison] = useState<any>(null);

  const handleExecute = async () => {
    if (!userMessage.trim()) {
      message.warning('Please enter a message');
      return;
    }
    setIsLoading(true);
    setResult('');
    try {
      const messages = [];
      if (systemPrompt) messages.push({ role: 'system', content: systemPrompt });
      messages.push({ role: 'user', content: userMessage });
      const res = await playgroundApi.execute({
        model,
        messages,
        temperature,
        max_tokens: maxTokens,
      }) as any;
      setResult(res?.content || res?.result?.content || JSON.stringify(res, null, 2));
    } catch (e) {
      message.error('Execution failed');
      setResult('Error: ' + (e as Error).message);
    } finally {
      setIsLoading(false);
    }
  };

  const handleCompare = async () => {
    if (!userMessage.trim()) {
      message.warning('Please enter a message');
      return;
    }
    setIsComparing(true);
    setCompareResults([]);
    setComparison(null);
    try {
      const messages = [];
      if (systemPrompt) messages.push({ role: 'system', content: systemPrompt });
      messages.push({ role: 'user', content: userMessage });
      const res = await playgroundApi.compareModels({
        models: compareModelsState,
        messages,
        temperature,
        max_tokens: maxTokens,
      }) as any;
      setCompareResults(res?.results || []);
      setComparison(res?.comparison || null);
    } catch (e) {
      message.error('Comparison failed');
    } finally {
      setIsComparing(false);
    }
  };

  const tabItems = [
    {
      key: 'single',
      label: <span><PlayCircleOutlined /> Single Model</span>,
      children: (
        <div>
          <Card title="Model Configuration" size="small" style={{ marginBottom: 16 }}>
            <Row gutter={16}>
              <Col span={12}>
                <div style={{ marginBottom: 8 }}>Model</div>
                <Select value={model} onChange={setModel} style={{ width: '100%' }}>
                  {availableModels.map(m => <Select.Option key={m.id} value={m.id}>{m.name} ({m.provider})</Select.Option>)}
                </Select>
              </Col>
              <Col span={6}>
                <div style={{ marginBottom: 8 }}>Temperature: {temperature}</div>
                <Slider min={0} max={2} step={0.1} value={temperature} onChange={setTemperature} />
              </Col>
              <Col span={6}>
                <div style={{ marginBottom: 8 }}>Max Tokens</div>
                <InputNumber min={1} max={32768} value={maxTokens} onChange={v => setMaxTokens(v || 2048)} style={{ width: '100%' }} />
              </Col>
            </Row>
          </Card>
          <Card title="Messages" size="small" style={{ marginBottom: 16 }}>
            <div style={{ marginBottom: 8 }}>System Prompt</div>
            <TextArea rows={2} value={systemPrompt} onChange={e => setSystemPrompt(e.target.value)} placeholder="System prompt..." style={{ marginBottom: 12 }} />
            <div style={{ marginBottom: 8 }}>User Message</div>
            <TextArea rows={4} value={userMessage} onChange={e => setUserMessage(e.target.value)} placeholder="Type your message here..." />
            <div style={{ marginTop: 12, textAlign: 'right' }}>
              <Space>
                <Button icon={<ClearOutlined />} onClick={() => { setUserMessage(''); setResult(''); }}>Clear</Button>
                <Button type="primary" icon={<PlayCircleOutlined />} onClick={handleExecute} loading={isLoading}>Execute</Button>
              </Space>
            </div>
          </Card>
          {result && (
            <Card title="Result" size="small">
              <div style={{ whiteSpace: 'pre-wrap', fontFamily: 'inherit' }}>{result}</div>
            </Card>
          )}
        </div>
      ),
    },
    {
      key: 'compare',
      label: <span><ThunderboltOutlined /> Compare Models</span>,
      children: (
        <div>
          <Card title="Model Comparison" size="small" style={{ marginBottom: 16 }}>
            <div style={{ marginBottom: 8 }}>Select models to compare (min 2)</div>
            <Select mode="multiple" value={compareModelsState} onChange={v => setCompareModelsState(v)} style={{ width: '100%' }} placeholder="Select models">
              {availableModels.map(m => <Select.Option key={m.id} value={m.id}>{m.name}</Select.Option>)}
            </Select>
            <div style={{ marginTop: 12, textAlign: 'right' }}>
              <Button type="primary" icon={<ThunderboltOutlined />} onClick={handleCompare} loading={isComparing} disabled={compareModelsState.length < 2}>
                Compare Models
              </Button>
            </div>
          </Card>
          {compareResults.length > 0 && (
            <Row gutter={16}>
              {compareResults.map((r: any, i: number) => (
                <Col span={Math.max(6, 24 / compareResults.length)} key={i}>
                  <Card title={<Tag color="blue">{r.model || `Model ${i + 1}`}</Tag>} size="small">
                    <div style={{ marginBottom: 8, fontSize: 12, color: '#888' }}>
                      Tokens: {toNum(r.total_tokens) || '-'} | Latency: {r.latency ? `${toNum(r.latency)}ms` : '-'} | Cost: ${r.cost?.toFixed(4) || '-'}
                    </div>
                    <div style={{ whiteSpace: 'pre-wrap', fontSize: 13 }}>{r.content || 'No output'}</div>
                  </Card>
                </Col>
              ))}
            </Row>
          )}
          {comparison && (
            <Card title="Comparison Summary" size="small" style={{ marginTop: 16 }}>
              <Space>
                {comparison.best_model && <Tag color="green">Best: {comparison.best_model}</Tag>}
                {comparison.fastest_model && <Tag color="blue">Fastest: {comparison.fastest_model}</Tag>}
                {comparison.cheapest_model && <Tag color="orange">Cheapest: {comparison.cheapest_model}</Tag>}
              </Space>
            </Card>
          )}
        </div>
      ),
    },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 24 }}>Playground</h2>
      <Tabs items={tabItems} />
    </div>
  );
}
