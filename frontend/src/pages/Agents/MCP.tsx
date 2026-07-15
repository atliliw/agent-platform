import { useState, useEffect, useCallback } from 'react';
import {
  Card, Table, Tag, Button, Space, Input, Collapse, message,
  Form, Select, Modal, Popconfirm, Badge, Divider, Tooltip,
} from 'antd';
import {
  PlayCircleOutlined, ToolOutlined, PlusOutlined,
  LinkOutlined, DisconnectOutlined, ReloadOutlined,
  DeleteOutlined, ApiOutlined,
} from '@ant-design/icons';
import {
  connectMCP, disconnectMCP, listMCPConnections,
  listMCPTools, callMCPTool,
  type MCPToolItem,
} from '../../api/mcp';
import type { MCPConnection } from '../../types';

// ── Category helpers ──────────────────────────────────────────

const TOOL_CATEGORY_COLORS: Record<string, string> = {
  knowledge_search: 'blue',
  web_search: 'cyan',
  quick_fetch: 'geekblue',
  calculator: 'green',
  code_execute: 'lime',
  data_analysis: 'purple',
  visualization: 'magenta',
  browser_execute: 'orange',
  browser_navigate: 'orange',
  browser_click: 'volcano',
  browser_type: 'volcano',
  browser_extract: 'volcano',
  browser_scroll: 'volcano',
  browser_wait: 'volcano',
  weather: 'gold',
  time: 'default',
  csdn_publish: 'red',
};

function getToolCategory(name: string): string {
  if (name.includes('browser')) return '🌐 浏览器';
  if (name.includes('search') || name.includes('knowledge') || name.includes('fetch')) return '🔍 搜索与知识';
  if (name.includes('exec') || name.includes('calc') || name.includes('analysis') || name.includes('visual')) return '💻 计算与分析';
  if (name.includes('publish')) return '📤 发布';
  return '🔧 工具';
}

function isRemoteTool(name: string): boolean {
  return name.includes('__');
}

function parseRemoteToolName(name: string): { connID: string; toolName: string } {
  const idx = name.indexOf('__');
  if (idx === -1) return { connID: '', toolName: name };
  return { connID: name.substring(0, idx), toolName: name.substring(idx + 2) };
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

const STATUS_MAP: Record<string, { color: string; text: string }> = {
  connecting:    { color: 'processing', text: '连接中' },
  connected:     { color: 'success',    text: '已连接' },
  disconnected:  { color: 'default',    text: '已断开' },
  error:         { color: 'error',      text: '错误' },
};

// ── Component ─────────────────────────────────────────────────

export default function MCPManagement() {
  const [tools, setTools] = useState<MCPToolItem[]>([]);
  const [connections, setConnections] = useState<MCPConnection[]>([]);
  const [loading, setLoading] = useState(false);
  const [testTool, setTestTool] = useState('');
  const [testArgs, setTestArgs] = useState('{}');
  const [testResult, setTestResult] = useState('');
  const [connectModalOpen, setConnectModalOpen] = useState(false);
  const [connectLoading, setConnectLoading] = useState(false);
  const [connectForm] = Form.useForm();

  // ── Data loading ──

  const loadTools = useCallback(async () => {
    setLoading(true);
    try {
      const res = await listMCPTools();
      setTools((res as any)?.tools || []);
    } catch (error) {
      console.error('Load tools failed:', error);
    } finally {
      setLoading(false);
    }
  }, []);

  const loadConnections = useCallback(async () => {
    try {
      const res = await listMCPConnections();
      setConnections((res as any)?.connections || []);
    } catch (error) {
      console.error('Load connections failed:', error);
    }
  }, []);

  useEffect(() => {
    loadTools();
    loadConnections();
  }, [loadTools, loadConnections]);

  // ── Connection management ──

  const handleConnect = async () => {
    try {
      const values = await connectForm.validateFields();
      setConnectLoading(true);

      // Parse env vars from key-value pairs
      const envPairs: Record<string, string> = {};
      if (values.envPairs) {
        for (const pair of values.envPairs) {
          if (pair?.key && pair?.value) {
            envPairs[pair.key] = pair.value;
          }
        }
      }

      await connectMCP({
        name: values.name,
        type: values.type,
        command: values.type === 'stdio' ? values.command : undefined,
        url: values.type === 'streamable-http' ? values.url : undefined,
        env: Object.keys(envPairs).length > 0 ? envPairs : undefined,
      });

      message.success('连接成功');
      setConnectModalOpen(false);
      connectForm.resetFields();
      loadConnections();
      loadTools();
    } catch (error: any) {
      message.error(`连接失败: ${error?.message || error}`);
    } finally {
      setConnectLoading(false);
    }
  };

  const handleDisconnect = async (connID: string) => {
    try {
      await disconnectMCP(connID);
      message.success('已断开连接');
      loadConnections();
      loadTools();
    } catch (error: any) {
      message.error(`断开失败: ${error?.message || error}`);
    }
  };

  // ── Tool test ──

  const handleTestTool = async () => {
    try {
      const res = await callMCPTool({
        name: testTool,
        arguments: JSON.parse(testArgs),
      });
      setTestResult(JSON.stringify(res, null, 2));
    } catch (error: any) {
      setTestResult(`执行失败: ${error?.message || error}`);
    }
  };

  // ── Group tools ──

  const builtinTools = tools.filter(t => !isRemoteTool(t.name));
  const remoteTools = tools.filter(t => isRemoteTool(t.name));

  // Group remote tools by connection
  const remoteByConn = remoteTools.reduce<Record<string, MCPToolItem[]>>((acc, tool) => {
    const { connID } = parseRemoteToolName(tool.name);
    if (!acc[connID]) acc[connID] = [];
    acc[connID].push(tool);
    return acc;
  }, {});

  // ── Columns ──

  const toolColumns = [
    {
      title: '工具名称',
      dataIndex: 'name',
      key: 'name',
      width: 200,
      render: (name: string) => {
        if (isRemoteTool(name)) {
          const { toolName } = parseRemoteToolName(name);
          return (
            <Space>
              <Tag color="purple" icon={<ApiOutlined />}>{toolName}</Tag>
              <span style={{ fontSize: 11, color: '#999' }}>远程</span>
            </Space>
          );
        }
        return (
          <Tag color={TOOL_CATEGORY_COLORS[name] || 'default'} icon={<ToolOutlined />}>
            {name}
          </Tag>
        );
      },
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
      render: (_: unknown, record: MCPToolItem) => getToolCategory(record.name),
    },
    {
      title: '参数',
      key: 'params',
      width: 280,
      render: (_: unknown, record: MCPToolItem) => (
        <span style={{ fontSize: 12, color: '#666' }}>
          {parseSchemaSummary(record.input_schema)}
        </span>
      ),
    },
    {
      title: '来源',
      key: 'source',
      width: 80,
      render: (_: unknown, record: MCPToolItem) =>
        isRemoteTool(record.name)
          ? <Tag color="purple">远程</Tag>
          : <Tag color="blue">内置</Tag>,
    },
  ];

  const connColumns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      width: 150,
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      width: 130,
      render: (type: string) => (
        <Tag color={type === 'stdio' ? 'cyan' : 'geekblue'}>
          {type === 'stdio' ? 'Stdio' : 'Streamable HTTP'}
        </Tag>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: string) => {
        const s = STATUS_MAP[status] || { color: 'default', text: status };
        return <Badge status={s.color as any} text={s.text} />;
      },
    },
    {
      title: '服务器',
      key: 'server',
      width: 180,
      render: (_: unknown, record: MCPConnection) => {
        if (record.server_name) {
          return (
            <Tooltip title={`v${record.server_version || '?'}`}>
              {record.server_name}
            </Tooltip>
          );
        }
        return record.type === 'stdio' ? record.command : record.url;
      },
    },
    {
      title: '工具数',
      key: 'toolCount',
      width: 80,
      render: (_: unknown, record: MCPConnection) => record.tool_count ?? 0,
    },
    {
      title: '错误',
      dataIndex: 'error_msg',
      key: 'error_msg',
      ellipsis: true,
      render: (msg: string) => msg ? <span style={{ color: '#ff4d4f' }}>{msg}</span> : '-',
    },
    {
      title: '操作',
      key: 'action',
      width: 100,
      render: (_: unknown, record: MCPConnection) => (
        <Popconfirm
          title="确定断开此连接？"
          onConfirm={() => handleDisconnect(record.id)}
          okText="断开"
          cancelText="取消"
        >
          <Button
            size="small"
            danger
            icon={<DisconnectOutlined />}
            disabled={record.status === 'disconnected'}
          >
            断开
          </Button>
        </Popconfirm>
      ),
    },
  ];

  // ── Render ──

  return (
    <div>
      {/* 连接管理 */}
      <Card
        title={
          <span>
            <LinkOutlined style={{ marginRight: 8 }} />
            MCP 连接 ({connections.length})
          </span>
        }
        extra={
          <Space>
            <Button
              icon={<ReloadOutlined />}
              onClick={() => { loadConnections(); loadTools(); }}
            >
              刷新
            </Button>
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => setConnectModalOpen(true)}
            >
              添加连接
            </Button>
          </Space>
        }
        style={{ marginBottom: 24 }}
      >
        <Table
          columns={connColumns}
          dataSource={connections}
          rowKey="id"
          pagination={false}
          size="small"
          locale={{ emptyText: '暂无连接，点击"添加连接"开始' }}
        />
      </Card>

      {/* 内置工具 */}
      <Card
        title={
          <span>
            <ToolOutlined style={{ marginRight: 8 }} />
            内置工具 ({builtinTools.length})
          </span>
        }
        style={{ marginBottom: 24 }}
      >
        <Table
          columns={toolColumns.filter(c => c.key !== 'source')}
          dataSource={builtinTools}
          rowKey="name"
          loading={loading}
          pagination={false}
          size="small"
        />
      </Card>

      {/* 远程工具（按连接分组） */}
      {Object.entries(remoteByConn).length > 0 && (
        <Card
          title={
            <span>
              <ApiOutlined style={{ marginRight: 8 }} />
              远程工具 ({remoteTools.length})
            </span>
          }
          style={{ marginBottom: 24 }}
        >
          <Collapse
            items={Object.entries(remoteByConn).map(([connID, connTools]) => {
              const conn = connections.find(c => c.id === connID);
              const label = conn?.name || connID;
              return {
                key: connID,
                label: (
                  <span>
                    {label}
                    <Tag color="purple" style={{ marginLeft: 8 }}>{connTools.length} 工具</Tag>
                    {conn && (
                      <Badge
                        status={(STATUS_MAP[conn.status]?.color as any) || 'default'}
                        style={{ marginLeft: 8 }}
                      />
                    )}
                  </span>
                ),
                children: (
                  <Table
                    columns={[
                      {
                        title: '工具', dataIndex: 'name', key: 'name', width: 200,
                        render: (name: string) => {
                          const { toolName } = parseRemoteToolName(name);
                          return <Tag color="purple" icon={<ApiOutlined />}>{toolName}</Tag>;
                        },
                      },
                      { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
                      {
                        title: '参数', key: 'params', width: 300,
                        render: (_: unknown, r: MCPToolItem) =>
                          <span style={{ fontSize: 12, color: '#666' }}>{parseSchemaSummary(r.input_schema)}</span>,
                      },
                    ]}
                    dataSource={connTools}
                    rowKey="name"
                    pagination={false}
                    size="small"
                  />
                ),
              };
            })}
          />
        </Card>
      )}

      {/* 工具测试 */}
      <Card title="🔧 工具测试">
        <Space direction="vertical" style={{ width: '100%' }}>
          <Space wrap>
            <Input
              placeholder="工具名称 (如 web_search 或 connID__toolName)"
              value={testTool}
              onChange={(e) => setTestTool(e.target.value)}
              style={{ width: 280 }}
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

      {/* 添加连接 Modal */}
      <Modal
        title="添加 MCP 连接"
        open={connectModalOpen}
        onOk={handleConnect}
        onCancel={() => { setConnectModalOpen(false); connectForm.resetFields(); }}
        confirmLoading={connectLoading}
        okText="连接"
        cancelText="取消"
        width={560}
      >
        <Form form={connectForm} layout="vertical" initialValues={{ type: 'stdio' }}>
          <Form.Item
            name="name"
            label="连接名称"
            rules={[{ required: true, message: '请输入连接名称' }]}
          >
            <Input placeholder="如: GitHub MCP Server" />
          </Form.Item>

          <Form.Item
            name="type"
            label="传输类型"
            rules={[{ required: true }]}
          >
            <Select>
              <Select.Option value="stdio">Stdio (子进程)</Select.Option>
              <Select.Option value="streamable-http">Streamable HTTP</Select.Option>
            </Select>
          </Form.Item>

          <Form.Item
            noStyle
            shouldUpdate={(prev, cur) => prev.type !== cur.type}
          >
            {({ getFieldValue }) => {
              const type = getFieldValue('type');
              if (type === 'stdio') {
                return (
                  <Form.Item
                    name="command"
                    label="启动命令"
                    rules={[{ required: true, message: '请输入启动命令' }]}
                    extra="如: npx @modelcontextprotocol/server-github"
                  >
                    <Input placeholder="npx @modelcontextprotocol/server-github" />
                  </Form.Item>
                );
              }
              return (
                <Form.Item
                  name="url"
                  label="服务器 URL"
                  rules={[{ required: true, message: '请输入服务器 URL' }]}
                  extra="如: https://mcp.example.com/sse"
                >
                  <Input placeholder="https://mcp.example.com/sse" />
                </Form.Item>
              );
            }}
          </Form.Item>

          <Divider plain>环境变量（可选）</Divider>

          <Form.List name="envPairs">
            {(fields, { add, remove }) => (
              <>
                {fields.map(({ key, name, ...restField }) => (
                  <Space key={key} style={{ display: 'flex', marginBottom: 8 }} align="baseline">
                    <Form.Item
                      {...restField}
                      name={[name, 'key']}
                      style={{ marginBottom: 0 }}
                    >
                      <Input placeholder="变量名 (如 GITHUB_TOKEN)" style={{ width: 200 }} />
                    </Form.Item>
                    <Form.Item
                      {...restField}
                      name={[name, 'value']}
                      style={{ marginBottom: 0 }}
                    >
                      <Input placeholder="变量值" style={{ width: 200 }} />
                    </Form.Item>
                    <DeleteOutlined
                      onClick={() => remove(name)}
                      style={{ color: '#ff4d4f' }}
                    />
                  </Space>
                ))}
                <Form.Item>
                  <Button type="dashed" onClick={() => add()} block icon={<PlusOutlined />}>
                    添加环境变量
                  </Button>
                </Form.Item>
              </>
            )}
          </Form.List>
        </Form>
      </Modal>
    </div>
  );
}
