import { useState, useEffect, useRef } from 'react';
import {
  Card,
  Table,
  Tag,
  Input,
  Button,
  Space,
  message,
  Statistic,
  Row,
  Col,
  Empty,
  Form,
  Modal,
  Select,
} from 'antd';
import {
  SearchOutlined,
  ReloadOutlined,
  PlusOutlined,
  ApartmentOutlined,
} from '@ant-design/icons';
import * as echarts from 'echarts';
import client from '../../api/client';

interface Concept {
  id: string;
  name: string;
  description: string;
  importance: number;
  confidence: number;
  type?: string;
  created_at: number;
}

interface GraphNode {
  id: string;
  type: string;
  name: string;
  properties: Record<string, unknown>;
}

interface GraphEdge {
  id: string;
  source_id: string;
  target_id: string;
  relation: string;
  properties: Record<string, unknown>;
}

export default function SemanticGraphPanel() {
  const [concepts, setConcepts] = useState<Concept[]>([]);
  const [loading, setLoading] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [addConceptModalVisible, setAddConceptModalVisible] = useState(false);
  const [addRelationModalVisible, setAddRelationModalVisible] = useState(false);
  const [conceptForm] = Form.useForm();
  const [relationForm] = Form.useForm();
  const graphRef = useRef<HTMLDivElement>(null);
  const graphChart = useRef<echarts.ECharts | null>(null);

  const loadConcepts = async () => {
    setLoading(true);
    try {
      const res = await client.get('/api/v2/memory/semantic?top_k=50') as { concepts: Concept[] };
      setConcepts(res.concepts || []);
    } catch (error) {
      console.error('Failed to load concepts', error);
      message.error('Failed to load semantic memories');
    } finally {
      setLoading(false);
    }
  };

  const searchConcepts = async () => {
    if (!searchQuery.trim()) {
      loadConcepts();
      return;
    }
    setLoading(true);
    try {
      const res = await client.get(`/api/v2/memory/semantic?query=${encodeURIComponent(searchQuery)}&top_k=20`) as { concepts: Concept[] };
      setConcepts(res.concepts || []);
    } catch (error) {
      message.error('Search failed');
    } finally {
      setLoading(false);
    }
  };

  const handleAddConcept = async () => {
    try {
      const values = await conceptForm.validateFields();
      await client.post('/api/v2/memory/semantic/concept', values);
      message.success('Concept saved');
      setAddConceptModalVisible(false);
      conceptForm.resetFields();
      loadConcepts();
    } catch (error) {
      message.error('Failed to save concept');
    }
  };

  const handleAddRelation = async () => {
    try {
      const values = await relationForm.validateFields();
      await client.post('/api/v2/memory/semantic/relation', values);
      message.success('Relation saved');
      setAddRelationModalVisible(false);
      relationForm.resetFields();
      loadGraph();
    } catch (error) {
      message.error('Failed to save relation');
    }
  };

  const loadGraph = async () => {
    try {
      const res = await client.get('/api/v2/memory-enhanced/graph') as {
        nodes: GraphNode[];
        edges: GraphEdge[];
      };

      if (graphRef.current && res.nodes && res.nodes.length > 0) {
        if (!graphChart.current) {
          graphChart.current = echarts.init(graphRef.current);
        }

        const categories = Array.from(new Set(res.nodes.map(n => n.type)));
        const categoryMap = Object.fromEntries(categories.map((c, i) => [c, i]));

        const option = {
          title: { text: 'Knowledge Graph', left: 'center', top: 10, textStyle: { fontSize: 14 } },
          tooltip: {
            formatter: (params: echarts.EChartsOption) => {
              const data = (params as { data: { name: string; category: number } }).data;
              return data?.name || '';
            },
          },
          legend: { data: categories, bottom: 0 },
          series: [{
            type: 'graph',
            layout: 'force',
            roam: true,
            draggable: true,
            label: { show: true, position: 'right', fontSize: 11 },
            force: { repulsion: 200, edgeLength: [80, 200] },
            data: res.nodes.map(n => ({
              id: n.id,
              name: n.name,
              category: categoryMap[n.type] ?? 0,
              symbolSize: 30,
            })),
            links: (res.edges || []).map(e => ({
              source: e.source_id,
              target: e.target_id,
              value: e.relation,
              lineStyle: { curveness: 0.2 },
            })),
            categories: categories.map(c => ({ name: c })),
            emphasis: { focus: 'adjacency', lineStyle: { width: 4 } },
          }],
        };

        graphChart.current.setOption(option);
      }
    } catch (error) {
      console.error('Failed to load knowledge graph', error);
    }
  };

  useEffect(() => {
    loadConcepts();
    loadGraph();

    const handleResize = () => {
      graphChart.current?.resize();
    };
    window.addEventListener('resize', handleResize);

    return () => {
      window.removeEventListener('resize', handleResize);
      graphChart.current?.dispose();
      graphChart.current = null;
    };
  }, []);

  const stats = {
    total: concepts.length,
    avgImportance: concepts.length > 0
      ? (concepts.reduce((sum, c) => sum + c.importance, 0) / concepts.length).toFixed(2)
      : '0.00',
    highConfidence: concepts.filter(c => c.confidence >= 0.8).length,
  };

  const columns = [
    {
      title: 'Name',
      dataIndex: 'name',
      key: 'name',
      width: 200,
      ellipsis: true,
      render: (name: string) => <strong>{name}</strong>,
    },
    {
      title: 'Description',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
      render: (desc: string) => desc?.substring(0, 80),
    },
    {
      title: 'Type',
      dataIndex: 'type',
      key: 'type',
      width: 100,
      render: (type: string) => type ? <Tag>{type}</Tag> : '-',
    },
    {
      title: 'Importance',
      dataIndex: 'importance',
      key: 'importance',
      width: 100,
      render: (val: number) => (
        <Tag color={val >= 0.7 ? 'green' : 'default'}>{(val * 100).toFixed(0)}%</Tag>
      ),
    },
    {
      title: 'Confidence',
      dataIndex: 'confidence',
      key: 'confidence',
      width: 100,
      render: (val: number) => (
        <Tag color={val >= 0.8 ? 'green' : val >= 0.5 ? 'blue' : 'orange'}>
          {(val * 100).toFixed(0)}%
        </Tag>
      ),
    },
    {
      title: 'Created',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 140,
      render: (ts: number) => ts ? new Date(ts * 1000).toLocaleDateString() : '-',
    },
  ];

  return (
    <div>
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={8}>
          <Card size="small">
            <Statistic title="Total Concepts" value={stats.total} />
          </Card>
        </Col>
        <Col span={8}>
          <Card size="small">
            <Statistic title="Avg Importance" value={stats.avgImportance} suffix="%" />
          </Card>
        </Col>
        <Col span={8}>
          <Card size="small">
            <Statistic title="High Confidence" value={stats.highConfidence} />
          </Card>
        </Col>
      </Row>

      <Space style={{ marginBottom: 16 }}>
        <Space.Compact>
          <Input
            placeholder="Search concepts..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            onPressEnter={searchConcepts}
            style={{ width: 300 }}
          />
          <Button type="primary" icon={<SearchOutlined />} onClick={searchConcepts} loading={loading}>
            Search
          </Button>
        </Space.Compact>
        <Button icon={<ReloadOutlined />} onClick={() => { loadConcepts(); loadGraph(); }} loading={loading}>
          Refresh
        </Button>
        <Button icon={<PlusOutlined />} onClick={() => setAddConceptModalVisible(true)}>
          Add Concept
        </Button>
        <Button icon={<ApartmentOutlined />} onClick={() => setAddRelationModalVisible(true)}>
          Add Relation
        </Button>
      </Space>

      <Row gutter={16}>
        <Col span={12}>
          <Card title="Knowledge Graph" size="small">
            <div ref={graphRef} style={{ width: '100%', height: 450 }} />
          </Card>
        </Col>
        <Col span={12}>
          <Table
            columns={columns}
            dataSource={concepts}
            rowKey="id"
            loading={loading}
            pagination={{ pageSize: 10 }}
            size="small"
            locale={{ emptyText: <Empty description="No concepts found" /> }}
          />
        </Col>
      </Row>

      <Modal
        title="Add Concept"
        open={addConceptModalVisible}
        onOk={handleAddConcept}
        onCancel={() => { setAddConceptModalVisible(false); conceptForm.resetFields(); }}
        okText="Save"
      >
        <Form form={conceptForm} layout="vertical">
          <Form.Item name="name" label="Name" rules={[{ required: true }]}>
            <Input placeholder="Concept name" />
          </Form.Item>
          <Form.Item name="type" label="Type">
            <Select placeholder="Select concept type">
              <Select.Option value="entity">Entity</Select.Option>
              <Select.Option value="fact">Fact</Select.Option>
              <Select.Option value="rule">Rule</Select.Option>
              <Select.Option value="concept">Concept</Select.Option>
              <Select.Option value="procedure">Procedure</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="description" label="Description">
            <Input.TextArea rows={3} placeholder="Describe this concept" />
          </Form.Item>
          <Form.Item name="importance" label="Importance" initialValue={0.5}>
            <Select>
              <Select.Option value={0.3}>Low (30%)</Select.Option>
              <Select.Option value={0.5}>Medium (50%)</Select.Option>
              <Select.Option value={0.7}>High (70%)</Select.Option>
              <Select.Option value={0.9}>Critical (90%)</Select.Option>
            </Select>
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="Add Relation"
        open={addRelationModalVisible}
        onOk={handleAddRelation}
        onCancel={() => { setAddRelationModalVisible(false); relationForm.resetFields(); }}
        okText="Save"
      >
        <Form form={relationForm} layout="vertical">
          <Form.Item name="from_concept" label="Source Concept" rules={[{ required: true }]}>
            <Input placeholder="Source concept ID or name" />
          </Form.Item>
          <Form.Item name="relation_type" label="Relation Type" rules={[{ required: true }]}>
            <Select placeholder="Select relation type">
              <Select.Option value="is_a">is_a</Select.Option>
              <Select.Option value="has_a">has_a</Select.Option>
              <Select.Option value="part_of">part_of</Select.Option>
              <Select.Option value="related_to">related_to</Select.Option>
              <Select.Option value="causes">causes</Select.Option>
              <Select.Option value="instance_of">instance_of</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="to_concept" label="Target Concept" rules={[{ required: true }]}>
            <Input placeholder="Target concept ID or name" />
          </Form.Item>
          <Form.Item name="weight" label="Weight" initialValue={0.5}>
            <Select>
              <Select.Option value={0.3}>Weak (30%)</Select.Option>
              <Select.Option value={0.5}>Medium (50%)</Select.Option>
              <Select.Option value={0.7}>Strong (70%)</Select.Option>
              <Select.Option value={0.9}>Very Strong (90%)</Select.Option>
            </Select>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
