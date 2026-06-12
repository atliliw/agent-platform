import { useState, useEffect } from 'react';
import { Card, Table, Tag, Button, Space } from 'antd';
import { PlayCircleOutlined } from '@ant-design/icons';
import client from '../../api/client';

interface Tool {
  name: string;
  description: string;
}

export default function MCPManagement() {
  const [tools, setTools] = useState<Tool[]>([]);
  const [loading, setLoading] = useState(false);
  const [testTool, setTestTool] = useState('');
  const [testArgs, setTestArgs] = useState('{}');
  const [testResult, setTestResult] = useState('');

  const loadTools = async () => {
    setLoading(true);
    try {
      const res = await client.get('/api/v2/mcp/tools');
      setTools(res.data?.tools || []);
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
      setTestResult(JSON.stringify(res.data, null, 2));
    } catch (error) {
      setTestResult('执行失败');
    }
  };

  const columns = [
    {
      title: '工具名称',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
    },
    {
      title: '状态',
      key: 'status',
      render: () => <Tag color="green">可用</Tag>,
    },
  ];

  return (
    <div>
      <Card title="内置工具" style={{ marginBottom: 24 }}>
        <Table
          columns={columns}
          dataSource={tools}
          rowKey="name"
          loading={loading}
          pagination={false}
        />
      </Card>

      <Card title="工具测试">
        <Space direction="vertical" style={{ width: '100%' }}>
          <Space>
            <input
              placeholder="工具名称"
              value={testTool}
              onChange={(e) => setTestTool(e.target.value)}
              style={{ width: 200, padding: '4px 11px', border: '1px solid #d9d9d9', borderRadius: 6 }}
            />
            <input
              placeholder='{"query": "test"}'
              value={testArgs}
              onChange={(e) => setTestArgs(e.target.value)}
              style={{ width: 300, padding: '4px 11px', border: '1px solid #d9d9d9', borderRadius: 6 }}
            />
            <Button type="primary" icon={<PlayCircleOutlined />} onClick={handleTestTool}>
              执行
            </Button>
          </Space>
          {testResult && (
            <pre style={{ background: '#f5f5f5', padding: 16, borderRadius: 8, overflow: 'auto' }}>
              {testResult}
            </pre>
          )}
        </Space>
      </Card>
    </div>
  );
}