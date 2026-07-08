import { useState, useCallback, useRef, useEffect, useMemo } from 'react';
import {
  ReactFlow,
  Controls,
  MiniMap,
  Background,
  addEdge,
  useNodesState,
  useEdgesState,
  type Connection,
  type Edge,
  type Node,
  type OnNodesChange,
  type OnEdgesChange,
  type NodeChange,
  type EdgeChange,
  Panel,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import {
  Card,
  Button,
  Space,
  Input,
  Modal,
  Form,
  Select,
  InputNumber,
  message,
  List,
  Typography,
  Drawer,
  Tag,
  Divider,
  Empty,
  Table,
} from 'antd';
import {
  PlusOutlined,
  SaveOutlined,
  PlayCircleOutlined,
  DeleteOutlined,
  FolderOutlined,
  RobotOutlined,
  ToolOutlined,
  QuestionCircleOutlined,
  BranchesOutlined,
  MergeCellsOutlined,
  HistoryOutlined,
  StopOutlined,
} from '@ant-design/icons';
import { useParams, useNavigate } from 'react-router-dom';
import { workflowApi, type Workflow, type WorkflowNode, type WorkflowEdge, type WorkflowExecutionResult, type WorkflowExecution } from '../../api/workflow';
import { AgentNode } from './nodes/AgentNode';
import { ConditionNode } from './nodes/ConditionNode';
import { ParallelNode } from './nodes/ParallelNode';
import { MergeNode } from './nodes/MergeNode';
import { ToolNode } from './nodes/ToolNode';

const nodeTypes = {
  agent: AgentNode,
  condition: ConditionNode,
  parallel: ParallelNode,
  merge: MergeNode,
  tool: ToolNode,
};

const NODE_COLORS: Record<string, string> = {
  agent: '#1677ff',
  tool: '#13c2c2',
  condition: '#fa8c16',
  parallel: '#52c41a',
  merge: '#722ed1',
};

const NODE_TYPE_OPTIONS = [
  { value: 'agent', label: 'Agent', icon: <RobotOutlined />, color: '#1677ff' },
  { value: 'tool', label: 'Tool', icon: <ToolOutlined />, color: '#13c2c2' },
  { value: 'condition', label: 'Condition', icon: <QuestionCircleOutlined />, color: '#fa8c16' },
  { value: 'parallel', label: 'Parallel', icon: <BranchesOutlined />, color: '#52c41a' },
  { value: 'merge', label: 'Merge', icon: <MergeCellsOutlined />, color: '#722ed1' },
];

const { Text } = Typography;

function workflowNodeToFlowNode(node: WorkflowNode): Node {
  return {
    id: node.id,
    type: node.type,
    position: node.position ?? { x: 0, y: 0 },
    data: {
      label: node.name,
      agent_id: node.agent_id,
      tool_name: node.tool_name,
      condition: node.condition,
      config: node.config,
    },
  };
}

function workflowEdgeToFlowEdge(edge: WorkflowEdge): Edge {
  return {
    id: edge.id,
    source: edge.from,
    target: edge.to,
    label: edge.label,
    data: { condition: edge.condition },
  };
}

function flowNodeToWorkflowNode(node: Node): WorkflowNode {
  return {
    id: node.id,
    type: node.type ?? 'agent',
    name: (node.data?.label as string) ?? '',
    agent_id: node.data?.agent_id as string | undefined,
    tool_name: node.data?.tool_name as string | undefined,
    condition: node.data?.condition as string | undefined,
    config: node.data?.config as Record<string, unknown> | undefined,
    position: node.position,
  };
}

function flowEdgeToWorkflowEdge(edge: Edge): WorkflowEdge {
  return {
    id: edge.id,
    from: edge.source,
    to: edge.target,
    condition: edge.data?.condition as string | undefined,
    label: edge.label as string | undefined,
  };
}

export default function WorkflowEditor() {
  const { id } = useParams<{ id?: string }>();
  const navigate = useNavigate();
  const reactFlowWrapper = useRef<HTMLDivElement>(null);

  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);

  const [workflowName, setWorkflowName] = useState('');
  const [workflowDescription, setWorkflowDescription] = useState('');
  const [selectedNode, setSelectedNode] = useState<Node | null>(null);
  const [workflowList, setWorkflowList] = useState<Workflow[]>([]);
  const [listModalOpen, setListModalOpen] = useState(false);
  const [executeModalOpen, setExecuteModalOpen] = useState(false);
  const [executeInput, setExecuteInput] = useState('');
  const [executeResult, setExecuteResult] = useState<WorkflowExecutionResult | null>(null);
  const [executionId, setExecutionId] = useState<string | null>(null);
  const [executions, setExecutions] = useState<WorkflowExecution[]>([]);
  const [loading, setLoading] = useState(false);

  const currentEntryNodeId = useMemo(() => {
    if (nodes.length > 0) return nodes[0].id;
    return '';
  }, [nodes]);

  // Load workflow if id is provided
  useEffect(() => {
    if (id) {
      loadWorkflow(id);
    }
  }, [id]);

  // Track selected node
  useEffect(() => {
    const selectedNodes = nodes.filter((n) => n.selected);
    setSelectedNode(selectedNodes.length > 0 ? selectedNodes[0] : null);
  }, [nodes]);

  const onConnect = useCallback(
    (params: Connection) => setEdges((eds) => addEdge(params, eds)),
    [setEdges],
  );

  const addNode = useCallback(
    (type: string) => {
      const newNode: Node = {
        id: `node_${Date.now()}`,
        type,
        position: { x: Math.random() * 400 + 100, y: Math.random() * 400 + 100 },
        data: {
          label: NODE_TYPE_OPTIONS.find((o) => o.value === type)?.label ?? type,
        },
      };
      setNodes((nds) => [...nds, newNode]);
    },
    [setNodes],
  );

  const loadWorkflow = async (workflowId: string) => {
    setLoading(true);
    try {
      const res = await workflowApi.get(workflowId);
      const wf = res as unknown as Workflow;
      setWorkflowName(wf.name);
      setWorkflowDescription(wf.description ?? '');
      setNodes(wf.nodes.map(workflowNodeToFlowNode));
      setEdges(wf.edges.map(workflowEdgeToFlowEdge));
      message.success('Workflow loaded');
    } catch (err) {
      message.error('Failed to load workflow');
    } finally {
      setLoading(false);
    }
  };

  const saveWorkflow = async () => {
    if (!workflowName) {
      message.warning('Please enter a workflow name');
      return;
    }

    setLoading(true);
    try {
      const wfNodes = nodes.map(flowNodeToWorkflowNode);
      const wfEdges = edges.map(flowEdgeToWorkflowEdge);

      const payload = {
        name: workflowName,
        description: workflowDescription,
        nodes: wfNodes,
        edges: wfEdges,
        entry_node_id: currentEntryNodeId,
      };

      if (id) {
        // Update existing workflow via PUT
        await workflowApi.update(id, payload);
        message.success('Workflow saved');
      } else {
        const res = await workflowApi.create(payload);
        const created = res as unknown as Workflow;
        navigate(`/workflow/${created.id}`);
        message.success('Workflow created');
      }
    } catch (err) {
      message.error('Failed to save workflow');
    } finally {
      setLoading(false);
    }
  };

  const deleteWorkflow = async () => {
    if (!id) {
      message.warning('No workflow loaded');
      return;
    }
    setLoading(true);
    try {
      await workflowApi.delete(id);
      message.success('Workflow deleted');
      setNodes([]);
      setEdges([]);
      setWorkflowName('');
      setWorkflowDescription('');
      navigate('/workflow');
    } catch (err) {
      message.error('Failed to delete workflow');
    } finally {
      setLoading(false);
    }
  };

  const loadWorkflowList = async () => {
    try {
      const res = await workflowApi.list();
      const data = res as unknown as { workflows: Workflow[] };
      setWorkflowList(data.workflows ?? []);
      setListModalOpen(true);
    } catch (err) {
      message.error('Failed to load workflows');
    }
  };

  const executeWorkflow = async () => {
    if (!id) {
      message.warning('Save the workflow first before executing');
      return;
    }
    setExecuteModalOpen(true);
    setExecuteResult(null);
  };

  const doExecute = async () => {
    if (!id) return;
    setLoading(true);
    setExecuteResult(null);
    setExecutionId(null);
    try {
      const res = await workflowApi.execute(id, executeInput);
      const result = res as unknown as WorkflowExecutionResult;
      setExecuteResult(result);
      if (result.execution_id) {
        setExecutionId(result.execution_id);
      }
      if (result.error) {
        message.error(`Workflow failed: ${result.error}`);
      } else {
        message.success('Workflow executed');
      }
      // Refresh execution history
      loadExecutionHistory();
    } catch (err) {
      message.error('Workflow execution failed');
    } finally {
      setLoading(false);
    }
  };

  const cancelExecution = async () => {
    if (!executionId) return;
    try {
      await workflowApi.cancelExecution(executionId);
      message.info('Execution cancelled');
      setExecutionId(null);
      loadExecutionHistory();
    } catch (err) {
      message.error('Failed to cancel execution');
    }
  };

  const loadExecutionHistory = async () => {
    if (!id) return;
    try {
      const res = await workflowApi.listExecutions(id, 10);
      const data = res as unknown as { executions: WorkflowExecution[] };
      setExecutions(data.executions ?? []);
    } catch (err) {
      console.error('Failed to load execution history', err);
    }
  };

  const updateNodeData = (key: string, value: string) => {
    if (!selectedNode) return;
    setNodes((nds) =>
      nds.map((n) => {
        if (n.id === selectedNode.id) {
          return { ...n, data: { ...n.data, [key]: value } };
        }
        return n;
      }),
    );
  };

  const minimapNodeColor = (node: Node) => {
    return NODE_COLORS[node.type ?? 'agent'] ?? '#999';
  };

  return (
    <div style={{ height: 'calc(100vh - 64px - 48px)', margin: '-24px', display: 'flex', flexDirection: 'column' }}>
      {/* Top toolbar */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          padding: '8px 16px',
          background: '#fff',
          borderBottom: '1px solid #f0f0f0',
          gap: 8,
        }}
      >
        <Input
          placeholder="Workflow Name"
          value={workflowName}
          onChange={(e) => setWorkflowName(e.target.value)}
          style={{ width: 200 }}
        />
        <Input
          placeholder="Description"
          value={workflowDescription}
          onChange={(e) => setWorkflowDescription(e.target.value)}
          style={{ width: 300 }}
        />
        <Space>
          <Button
            type="primary"
            icon={<SaveOutlined />}
            onClick={saveWorkflow}
            loading={loading}
          >
            Save
          </Button>
          <Button icon={<FolderOutlined />} onClick={loadWorkflowList}>
            Load
          </Button>
          <Button
            icon={<PlayCircleOutlined />}
            onClick={executeWorkflow}
            disabled={!id}
            loading={loading}
          >
            Execute
          </Button>
          {executionId && (
            <Button
              icon={<StopOutlined />}
              danger
              onClick={cancelExecution}
            >
              Cancel
            </Button>
          )}
          <Button
            icon={<HistoryOutlined />}
            onClick={loadExecutionHistory}
            disabled={!id}
          >
            History
          </Button>
          <Button
            icon={<DeleteOutlined />}
            danger
            onClick={deleteWorkflow}
            disabled={!id}
          >
            Delete
          </Button>
        </Space>
      </div>

      <div style={{ flex: 1, display: 'flex' }}>
        {/* Left sidebar: node type panel */}
        <div
          style={{
            width: 200,
            background: '#fff',
            borderRight: '1px solid #f0f0f0',
            padding: 12,
            overflowY: 'auto',
          }}
        >
          <Text strong style={{ marginBottom: 8 }}>Node Types</Text>
          <List
            size="small"
            dataSource={NODE_TYPE_OPTIONS}
            renderItem={(item) => (
              <List.Item
                style={{
                  cursor: 'pointer',
                  padding: '8px 12px',
                  borderRadius: 6,
                  marginBottom: 4,
                  border: `1px solid ${item.color}40`,
                  background: `${item.color}10`,
                }}
                onClick={() => addNode(item.value)}
              >
                <Space>
                  <span style={{ color: item.color }}>{item.icon}</span>
                  <Text>{item.label}</Text>
                </Space>
              </List.Item>
            )}
          />
          <Divider style={{ margin: '8px 0' }} />
          <Text type="secondary" style={{ fontSize: 12 }}>
            Click a node type above to add it to the canvas. Drag nodes to rearrange. Click a node to edit properties.
          </Text>
        </div>

        {/* Center: React Flow canvas */}
        <div ref={reactFlowWrapper} style={{ flex: 1 }}>
          <ReactFlow
            nodes={nodes}
            edges={edges}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            onConnect={onConnect}
            nodeTypes={nodeTypes}
            fitView
            style={{ background: '#fafafa' }}
          >
            <Controls />
            <MiniMap nodeColor={minimapNodeColor} maskColor="rgba(0,0,0,0.05)" />
            <Background />
            <Panel position="top-right">
              <Tag color={id ? 'green' : 'orange'}>
                {id ? `ID: ${id}` : 'New Workflow'}
              </Tag>
            </Panel>
          </ReactFlow>
        </div>

        {/* Right sidebar: property panel */}
        <div
          style={{
            width: 280,
            background: '#fff',
            borderLeft: '1px solid #f0f0f0',
            padding: 12,
            overflowY: 'auto',
          }}
        >
          {selectedNode ? (
            <>
              <Text strong style={{ marginBottom: 8 }}>Node Properties</Text>
              <Form layout="vertical" size="small">
                <Form.Item label="ID">
                  <Input value={selectedNode.id} disabled />
                </Form.Item>
                <Form.Item label="Type">
                  <Tag color={NODE_COLORS[selectedNode.type ?? 'agent']}>
                    {selectedNode.type ?? 'unknown'}
                  </Tag>
                </Form.Item>
                <Form.Item label="Name">
                  <Input
                    value={(selectedNode.data?.label as string) ?? ''}
                    onChange={(e) => updateNodeData('label', e.target.value)}
                  />
                </Form.Item>

                {selectedNode.type === 'agent' && (
                  <Form.Item label="Agent ID">
                    <Input
                      value={(selectedNode.data?.agent_id as string) ?? ''}
                      onChange={(e) => updateNodeData('agent_id', e.target.value)}
                      placeholder="e.g., researcher, coder"
                    />
                  </Form.Item>
                )}

                {selectedNode.type === 'tool' && (
                  <Form.Item label="Tool Name">
                    <Input
                      value={(selectedNode.data?.tool_name as string) ?? ''}
                      onChange={(e) => updateNodeData('tool_name', e.target.value)}
                      placeholder="e.g., browser_navigate, search"
                    />
                  </Form.Item>
                )}

                {selectedNode.type === 'condition' && (
                  <Form.Item label="Condition Expression">
                    <Input
                      value={(selectedNode.data?.condition as string) ?? ''}
                      onChange={(e) => updateNodeData('condition', e.target.value)}
                      placeholder="e.g., contains:error"
                    />
                  </Form.Item>
                )}
              </Form>
            </>
          ) : (
            <Empty description="Select a node to edit properties" />
          )}
        </div>
      </div>

      {/* Workflow list modal */}
      <Modal
        title="Load Workflow"
        open={listModalOpen}
        onCancel={() => setListModalOpen(false)}
        footer={null}
        width={600}
      >
        <List
          dataSource={workflowList}
          renderItem={(wf) => (
            <List.Item
              actions={[
                <Button
                  type="link"
                  onClick={() => {
                    setListModalOpen(false);
                    navigate(`/workflow/${wf.id}`);
                  }}
                >
                  Load
                </Button>,
              ]}
            >
              <List.Item.Meta
                title={wf.name}
                description={`${wf.description ?? 'No description'} - ${wf.nodes.length} nodes, ${wf.edges.length} edges`}
              />
            </List.Item>
          )}
          locale={{ emptyText: 'No workflows found' }}
        />
      </Modal>

      {/* Execute modal */}
      <Modal
        title="Execute Workflow"
        open={executeModalOpen}
        onCancel={() => setExecuteModalOpen(false)}
        onOk={doExecute}
        okText="Execute"
        confirmLoading={loading}
        width={700}
      >
        <Form layout="vertical">
          <Form.Item label="Input">
            <Input.TextArea
              rows={4}
              value={executeInput}
              onChange={(e) => setExecuteInput(e.target.value)}
              placeholder="Enter input for the workflow"
            />
          </Form.Item>
          {executeResult && (
            <>
              <Form.Item label="Status">
                <Space>
                  <Tag color={executeResult.status === 'completed' ? 'green' : executeResult.status === 'failed' ? 'red' : 'blue'}>
                    {executeResult.status || 'unknown'}
                  </Tag>
                  {executionId && <Text type="secondary">ID: {executionId}</Text>}
                </Space>
              </Form.Item>
              <Form.Item label="Final Output">
                <Input.TextArea rows={4} value={executeResult.final_output || 'No output'} readOnly />
              </Form.Item>
              {executeResult.nodes && executeResult.nodes.length > 0 && (
                <Form.Item label="Node Results">
                  <Table
                    size="small"
                    pagination={false}
                    dataSource={executeResult.nodes}
                    rowKey="node_id"
                    columns={[
                      { title: 'Node', dataIndex: 'node_id', key: 'node_id', width: 120 },
                      { title: 'Type', dataIndex: 'node_type', key: 'node_type', width: 80, render: (t: string) => t ? <Tag>{t}</Tag> : '-' },
                      { title: 'Output', dataIndex: 'output', key: 'output', ellipsis: true },
                      { title: 'Error', dataIndex: 'error', key: 'error', width: 100, render: (e: string) => e ? <Tag color="red">{e.substring(0, 30)}</Tag> : '-' },
                    ]}
                  />
                </Form.Item>
              )}
            </>
          )}
        </Form>
      </Modal>

      {/* Execution history modal */}
      <Modal
        title="Execution History"
        open={executions.length > 0}
        onCancel={() => setExecutions([])}
        footer={null}
        width={800}
      >
        <Table
          size="small"
          pagination={false}
          dataSource={executions}
          rowKey="id"
          columns={[
            { title: 'ID', dataIndex: 'id', key: 'id', width: 120, render: (id: string) => id.substring(0, 12) + '...' },
            { title: 'Status', dataIndex: 'status', key: 'status', width: 100, render: (s: string) => (
              <Tag color={s === 'completed' ? 'green' : s === 'failed' ? 'red' : s === 'running' ? 'blue' : 'default'}>{s}</Tag>
            )},
            { title: 'Input', dataIndex: 'input', key: 'input', ellipsis: true, width: 150 },
            { title: 'Duration', dataIndex: 'duration_ms', key: 'duration_ms', width: 80, render: (d: number) => d ? `${d}ms` : '-' },
            { title: 'Started', dataIndex: 'started_at', key: 'started_at', width: 150, render: (t: number) => t ? new Date(t * 1000).toLocaleString() : '-' },
          ]}
        />
      </Modal>
    </div>
  );
}
