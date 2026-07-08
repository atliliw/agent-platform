import { useState, useEffect } from 'react';
import { Card, Table, Tag, Button, Space, Input, Collapse, message } from 'antd';
import { PlayCircleOutlined, ToolOutlined } from '@ant-design/icons';
import client from '../../api/client';

interface MCPTool {
  name: string;
  description: string;
  input_schema: string; // JSON string of JSON Schema
}

const TOOL_CATEGORY_COLORS: Record<string, string> = {
  // Search & Knowledge
  knowledge_search: 'blue',
  web_search: 'cyan',
  quick_fetch: 'geekblue',
  // Computation
  calculator: 'green',
  code_execute: 'lime',
  data_analysis: 'purple',
  visualization: 'magenta',
  // Browser & Web
  browser_execute: 'orange',
  // Utility
  weather: 'gold',
  time: 'default',
  csdn_publish: 'red',
};

function getToolCategory(name: string): string {
  if (name.includes('search') || name.includes('knowledge') || name.includes('fetch')) return '🔍 搜索与知识';
  if (name.includes('exec') || name.includes('calc') || name.includes('analysis') || name.includes('visual')) return '💻 计算与分析';
  if (name.includes('browser')) return '🌐 浏览器';
  if (name.includes('publish')) return '📤 发布';
  return '🔧 工具';
}

function parseSchemaSummary(inputSchema: string): string {
  try {
    const schema = JSON.parse(inputSchema);
    const props = schema.properties || {};
    const required = schema.required || [];
    const fields = Object.keys(props);
    if (fields.length === 0) return '无参数';
    return fields.map(f => {
      const desc = props[f].description || props[f].type || '';
      const req = required.includes(f) ? '*' : '';
      return `${f}${req}: ${desc}`;
    }).join(', ');
  } catch {
    return '无法解析';
  }
}

export default function MCPManagement() {
  const [tools, setTools] = useState<MCPTool[]>([]);
  const [loading, setLoading] = useState(false);
  const [testTool, setTestTool] = useState('');
  const [testArgs, setTestArgs] = useState('{}');
  const [testResult, setTestResult] = useState('');

  const loadTools = async () => {
    setLoading(true);
    try {
      const res = await client.get('/api/v2/mcp/tools');
      setTools((res as any)?.tools || []);
    } catch (error) {
      console.error('Load tools failed:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadTools();
  }, []);

  const handleTestTool = async () => {
    try {
      const res = await client.post('/api/v2/mcp/call', {
        name: testTool,
        arguments: JSON.parse(testArgs),
      });
      setTestResult(JSON.stringify(res, null, 2));
    } catch (error: any) {
      setTestResult(`执行失败: ${error?.message || error}`);
    }
  };

  const columns = [
    {
      title: '工具名称',
      dataIndex: 'name',
      key: 'name',
      width: 180,
      render: (name: string) => (
        <Space>
          <Tag color={TOOL_CATEGORY_COLORS[name] || 'default'} icon={<ToolOutlined />}>
            {name}
          </Tag>
        </Space>
      ),
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: '分类',
      key: 'category',
      width: 120,
      render: (_: unknown, record: MCPTool) => getToolCategory(record.name),
    },
    {
      title: '参数',
      key: 'params',
      width: 280,
      render: (_: unknown, record: MCPTool) => (
        <span style={{ fontSize: 12, color: '#666' }}>
          {parseSchemaSummary(record.input_schema)}
        </span>
      ),
    },
    {
      title: '状态',
      key: 'status',
      width: 80,
      render: () => <Tag color="green">可用</Tag>,
    },
  ];

  // Group tools by category
  const categories = tools.reduce<Record<string, MCPTool[]>>((acc, tool) => {
    const cat = getToolCategory(tool.name);
    if (!acc[cat]) acc[cat] = [];
    acc[cat].push(tool);
    return acc;
  }, {});

  return (
    <div>
      {/* 工具总览 */}
      <Card
        title={
          <span>
            <ToolOutlined style={{ marginRight: 8 }} />
            内置工具 ({tools.length})
          </span>
        }
        extra={<Button onClick={loadTools} loading={loading}>刷新</Button>}
        style={{ marginBottom: 24 }}
      >
        <Table
          columns={columns}
          dataSource={tools}
          rowKey="name"
          loading={loading}
          pagination={false}
          size="small"
        />
      </Card>

      {/* 按分类展示 */}
      {Object.entries(categories).length > 0 && (
        <Card title="工具分类详情" style={{ marginBottom: 24 }}>
          <Collapse
            items={Object.entries(categories).map(([category, categoryTools]) => ({
              key: category,
              label: (
                <span>
                  {category}
                  <Tag style={{ marginLeft: 8 }}>{categoryTools.length}</Tag>
                </span>
              ),
              children: (
                <Table
                  columns={[
                    { title: '工具', dataIndex: 'name', key: 'name', width: 180,
                      render: (n: string) => <Tag color={TOOL_CATEGORY_COLORS[n] || 'default'}>{n}</Tag> },
                    { title: '描述', dataIndex: 'description', key: 'description' },
                    { title: '参数', key: 'params', width: 300,
                      render: (_: unknown, r: MCPTool) => <span style={{ fontSize: 12, color: '#666' }}>{parseSchemaSummary(r.input_schema)}</span> },
                  ]}
                  dataSource={categoryTools}
                  rowKey="name"
                  pagination={false}
                  size="small"
                />
              ),
            }))}
          />
        </Card>
      )}

      {/* 工具测试 */}
      <Card title="🔧 工具测试">
        <Space direction="vertical" style={{ width: '100%' }}>
          <Space wrap>
            <Input
              placeholder="工具名称 (如 web_search)"
              value={testTool}
              onChange={(e) => setTestTool(e.target.value)}
              style={{ width: 200 }}
            />
            <Input
              placeholder='参数 JSON (如 {"query": "test"})'
              value={testArgs}
              onChange={(e) => setTestArgs(e.target.value)}
              style={{ width: 360 }}
            />
            <Button
              type="primary"
              icon={<PlayCircleOutlined />}
              onClick={handleTestTool}
              disabled={!testTool}
            >
              执行
            </Button>
          </Space>
          {testResult && (
            <pre style={{
              background: testResult.includes('失败') ? '#fff2f0' : '#f6f8fa',
              padding: 16,
              borderRadius: 8,
              overflow: 'auto',
              maxHeight: 400,
              fontSize: 13,
              border: `1px solid ${testResult.includes('失败') ? '#ffccc7' : '#d9d9d9'}`,
            }}>
              {testResult}
            </pre>
          )}
        </Space>
      </Card>
    </div>
  );
}
