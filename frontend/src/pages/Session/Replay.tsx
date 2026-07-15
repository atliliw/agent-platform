import { useState, useEffect, useRef, useCallback } from "react";
import { useParams, useSearchParams } from "react-router-dom";
import {
  Card, Tabs, Timeline, Button, Space, Slider, Tag, Badge, Descriptions, Empty,
  Spin, Progress, Tooltip, Table, Modal, message,
} from "antd";
import {
  PlayCircleOutlined, PauseCircleOutlined, StepForwardOutlined, StepBackwardOutlined,
  DownloadOutlined, ReloadOutlined, SaveOutlined, RedoOutlined,
} from "@ant-design/icons";
import dayjs from "dayjs";
import client from "../../api/client";
import type { Session, SessionStep, ExecutionGraph, Checkpoint } from "../../api/session";
import { sessionApi } from "../../api/session";
import {
  decomposeMessagesToSteps,
  buildExecutionGraph,
  STEP_TYPE_LABELS,
  STEP_TYPE_COLORS,
  STEP_TYPE_ICONS,
} from "./replayTransform";
import StepDetail from "./StepDetail";

export default function SessionReplayPage() {
  const { sessionId } = useParams<{ sessionId: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const [session, setSession] = useState<Session | null>(null);
  const [steps, setSteps] = useState<SessionStep[]>([]);
  const [graph, setGraph] = useState<ExecutionGraph | null>(null);
  const [loading, setLoading] = useState(true);
  const [currentStepIndex, setCurrentStepIndex] = useState(0);
  const [isPlaying, setIsPlaying] = useState(false);
  const [playbackSpeed, setPlaybackSpeed] = useState(1);
  const [selectedStep, setSelectedStep] = useState<SessionStep | null>(null);
  const [checkpoints, setCheckpoints] = useState<Checkpoint[]>([]);
  const [selectedCheckpoint, setSelectedCheckpoint] = useState<Checkpoint | null>(null);
  const [resumeModalVisible, setResumeModalVisible] = useState(false);
  const [resuming, setResuming] = useState(false);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const loadReplayData = async () => {
    if (!sessionId) return;
    setLoading(true);
    try {
      const sessionRes = await client.get("/api/v2/sessions/" + sessionId) as any;
      const raw = sessionRes?.session || sessionRes;

      // 构建 session 对象
      const sess: Session = {
        id: raw.id,
        agent_id: "main-agent",
        status: "completed",
        total_tokens: raw.total_tokens || 0,
        total_cost: raw.total_cost || 0,
        duration: raw.updated_at && raw.created_at ? (raw.updated_at - raw.created_at) * 1000 : 0,
        start_time: raw.created_at,
        end_time: raw.updated_at,
      };

      // 使用新的分解逻辑将 messages 转为细粒度步骤
      const messages = raw.messages || [];
      const decomposedSteps = decomposeMessagesToSteps(messages, sessionId, raw.created_at);
      const executionGraph = buildExecutionGraph(decomposedSteps);

      setSession(sess);
      setSteps(decomposedSteps);
      setGraph(executionGraph);

      // 加载 checkpoints
      try {
        const cpRes = await sessionApi.listCheckpoints(sessionId) as any;
        const cpList = Array.isArray(cpRes) ? cpRes : (cpRes?.checkpoints || cpRes?.data || []);
        setCheckpoints(cpList);
      } catch {
        setCheckpoints([]);
      }
    } catch {
      setSession(null);
      setSteps([]);
      setGraph(null);
      setCheckpoints([]);
    } finally {
      setLoading(false);
    }
  };

  const play = useCallback(() => {
    setIsPlaying(true);
    const interval = 1000 / playbackSpeed;
    intervalRef.current = setInterval(() => {
      setCurrentStepIndex((prev) => {
        if (prev >= steps.length - 1) {
          setIsPlaying(false);
          return prev;
        }
        return prev + 1;
      });
    }, interval);
  }, [playbackSpeed, steps.length]);

  const pause = useCallback(() => {
    setIsPlaying(false);
    if (intervalRef.current) {
      clearInterval(intervalRef.current);
    }
  }, []);

  const stepForward = useCallback(() => {
    setCurrentStepIndex((prev) => Math.min(prev + 1, steps.length - 1));
  }, [steps.length]);

  const stepBackward = useCallback(() => {
    setCurrentStepIndex((prev) => Math.max(prev - 1, 0));
  }, []);

  const goToStep = useCallback((index: number) => {
    setCurrentStepIndex(index);
  }, []);

  const handleResumeCheckpoint = async () => {
    if (!selectedCheckpoint || !sessionId) return;
    setResuming(true);
    try {
      await sessionApi.resumeCheckpoint(sessionId, selectedCheckpoint.id);
      message.success(`已从 Step ${selectedCheckpoint.step} 恢复`);
      setResumeModalVisible(false);
      setSelectedCheckpoint(null);
      await loadReplayData();
    } catch {
      message.error("恢复失败，请重试");
    } finally {
      setResuming(false);
    }
  };

  const openResumeModal = (cp: Checkpoint) => {
    setSelectedCheckpoint(cp);
    setResumeModalVisible(true);
  };

  useEffect(() => {
    loadReplayData();
    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
      }
    };
  }, [sessionId]);

  useEffect(() => {
    if (isPlaying && intervalRef.current) {
      clearInterval(intervalRef.current);
      const interval = 1000 / playbackSpeed;
      intervalRef.current = setInterval(() => {
        setCurrentStepIndex((prev) => {
          if (prev >= steps.length - 1) {
            setIsPlaying(false);
            return prev;
          }
          return prev + 1;
        });
      }, interval);
    }
  }, [playbackSpeed, isPlaying, steps.length]);

  const getStatusColor = (status: string) => {
    const map: Record<string, string> = {
      pending: "default",
      running: "processing",
      completed: "success",
      failed: "error",
    };
    return map[status] || "default";
  };

  /** 获取步骤在 graph nodes 中的索引（用于高亮） */
  const getGraphNodeIndex = (stepId: string) => {
    if (!graph) return -1;
    // graph nodes: [start, ...stepNodes, end]
    return graph.nodes.findIndex((n) => n.id === stepId);
  };

  /** 渲染执行图 */
  const renderGraph = () => {
    if (!graph) return <Empty description="无执行图数据" />;

    const nodeWidth = 140;
    const nodeHeight = 48;
    const nodeSpacingX = 60;
    const startX = 40;

    // 布局：按节点索引分配 X 位置
    // 对于有 parent_step_id 的分支节点，计算 Y 偏移
    const layoutMap = new Map<string, { x: number; y: number }>();
    const topologicalOrder = graph.nodes;

    // 统计每个父节点下的子节点数量，用于分支布局
    const branchCount = new Map<string, number>();
    const branchIndex = new Map<string, number>();

    for (const node of topologicalOrder) {
      if (node.type === 'start' || node.type === 'end') continue;
      // 从 edges 找到这个节点的来源
      const incomingEdge = graph.edges.find(e => e.to === node.id);
      if (incomingEdge && incomingEdge.label === '展开') {
        // 这是分支子节点
        const parent = incomingEdge.from;
        const count = branchCount.get(parent) || 0;
        branchCount.set(parent, count + 1);
        branchIndex.set(node.id, count);
      }
    }

    let colIndex = 0;
    for (const node of topologicalOrder) {
      if (node.type === 'start') {
        layoutMap.set(node.id, { x: startX, y: 100 });
        colIndex = 0;
      } else if (node.type === 'end') {
        layoutMap.set(node.id, { x: startX + colIndex * (nodeWidth + nodeSpacingX), y: 100 });
      } else {
        const isBranchChild = branchIndex.has(node.id);
        const y = isBranchChild ? 100 + (branchIndex.get(node.id) || 0) * (nodeHeight + 20) : 100;
        layoutMap.set(node.id, { x: startX + colIndex * (nodeWidth + nodeSpacingX), y });
        colIndex++;
      }
    }

    const totalWidth = Math.max(topologicalOrder.length * (nodeWidth + nodeSpacingX) + startX, 600);
    const maxBranchY = Math.max(...Array.from(layoutMap.values()).map(p => p.y), 100);
    const svgHeight = maxBranchY + nodeHeight + 60;

    const getNodeColor = (status: string) => {
      const map: Record<string, string> = {
        pending: "#d9d9d9",
        running: "#1677ff",
        completed: "#52c41a",
        failed: "#ff4d4f",
      };
      return map[status] || "#d9d9d9";
    };

    return (
      <svg width={totalWidth} height={svgHeight} style={{ overflow: "visible" }}>
        <defs>
          <marker id="arrowhead" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">
            <polygon points="0 0, 10 3.5, 0 7" fill="#b1b1b7" />
          </marker>
        </defs>
        {graph.edges.map((edge, index) => {
          const sourcePos = layoutMap.get(edge.from || '');
          const targetPos = layoutMap.get(edge.to || '');
          if (!sourcePos || !targetPos) return null;
          return (
            <line
              key={`${edge.from}-${edge.to}-${index}`}
              x1={sourcePos.x + nodeWidth}
              y1={sourcePos.y + nodeHeight / 2}
              x2={targetPos.x}
              y2={targetPos.y + nodeHeight / 2}
              stroke={edge.label === '展开' ? '#722ed1' : '#b1b1b7'}
              strokeWidth={edge.label === '展开' ? 1.5 : 2}
              strokeDasharray={edge.label === '展开' ? '5,3' : undefined}
              markerEnd="url(#arrowhead)"
            />
          );
        })}
        {graph.nodes.map((node, index) => {
          const pos = layoutMap.get(node.id);
          if (!pos) return null;
          // 判断是否活跃（当前步骤及之前的步骤）
          const nodeStepIndex = steps.findIndex(s => s.id === node.id);
          const isActive = node.type === 'start' || node.type === 'end'
            ? true
            : nodeStepIndex >= 0 && nodeStepIndex <= currentStepIndex;
          const isCurrent = nodeStepIndex === currentStepIndex;
          const stepColor = STEP_TYPE_COLORS[node.type || ''] || getNodeColor(node.status || '');
          const bgColor = isCurrent ? stepColor : isActive ? stepColor : '#f0f0f0';
          const textColor = (isActive || isCurrent) ? '#fff' : '#666';

          return (
            <g key={node.id} onClick={() => {
              if (nodeStepIndex >= 0) goToStep(nodeStepIndex);
            }} style={{ cursor: nodeStepIndex >= 0 ? 'pointer' : 'default' }}>
              <rect
                x={pos.x}
                y={pos.y}
                width={nodeWidth}
                height={nodeHeight}
                rx={8}
                fill={bgColor}
                stroke={isCurrent ? '#000' : stepColor}
                strokeWidth={isCurrent ? 3 : 1.5}
              />
              <text
                x={pos.x + nodeWidth / 2}
                y={pos.y + nodeHeight / 2}
                textAnchor="middle"
                dominantBaseline="middle"
                fill={textColor}
                fontSize={11}
                fontWeight={isCurrent ? 'bold' : 'normal'}
              >
                {node.label}
              </text>
            </g>
          );
        })}
      </svg>
    );
  };

  const formatDuration = (ms?: number) => {
    if (!ms) return "-";
    if (ms < 1000) return ms + "ms";
    if (ms < 60000) return (ms / 1000).toFixed(1) + "s";
    return (ms / 60000).toFixed(1) + "min";
  };

  /** 渲染时间线 */
  const renderTimeline = () => {
    return (
      <Timeline
        items={steps.map((step, index) => {
          const stepType = step.step_type || 'unknown';
          const stepLabel = STEP_TYPE_LABELS[stepType] || stepType;
          const stepColor = STEP_TYPE_COLORS[stepType] || '#999';
          const stepIcon = STEP_TYPE_ICONS[stepType] || '📋';
          const isActive = index <= currentStepIndex;
          const isCurrent = index === currentStepIndex;

          return {
            key: step.id,
            color: isActive ? stepColor : 'gray',
            dot: isCurrent ? (
              <div style={{
                width: 20, height: 20, borderRadius: '50%',
                background: stepColor, display: 'flex',
                alignItems: 'center', justifyContent: 'center',
                fontSize: 11, lineHeight: '20px', textAlign: 'center',
              }}>
                {stepIcon}
              </div>
            ) : undefined,
            children: (
              <Card
                size="small"
                style={{
                  cursor: "pointer",
                  borderColor: isCurrent ? stepColor : undefined,
                  borderWidth: isCurrent ? 2 : 1,
                  opacity: isActive ? 1 : 0.5,
                }}
                onClick={() => {
                  setSelectedStep(step);
                  goToStep(index);
                }}
              >
                <Space direction="vertical" size={4} style={{ width: '100%' }}>
                  <Space>
                    <span>{stepIcon}</span>
                    <Tag color={isActive ? stepColor : 'default'}>Step {step.step_number}</Tag>
                    <Tag color={isActive ? stepColor : 'default'}>{stepLabel}</Tag>
                    {step.parent_step_id && (
                      <Tag color="purple" style={{ fontSize: 10 }}>子步骤</Tag>
                    )}
                    <Badge status={getStatusColor(step.status || "") as "success" | "processing" | "error" | "warning" | "default"} />
                  </Space>
                  <div style={{ fontSize: 12, color: "#666" }}>
                    {step.timestamp ? dayjs(step.timestamp * 1000).format("HH:mm:ss") : "-"}
                  </div>
                  {isActive && step.step_type === 'think' && step.input && (
                    <div style={{ fontSize: 12, color: '#722ed1', fontStyle: 'italic', marginTop: 4 }}>
                      💭 {truncate(step.input, 80)}
                    </div>
                  )}
                  {isActive && step.step_type === 'tool_call' && step.input && (
                    <div style={{ fontSize: 12, color: '#1677ff', marginTop: 4 }}>
                      🔧 {step.input}
                    </div>
                  )}
                  {isActive && step.step_type === 'observation' && step.output && (
                    <div style={{ fontSize: 12, color: '#52c41a', marginTop: 4 }}>
                      👁 {truncate(step.output, 80)}
                    </div>
                  )}
                  {isActive && step.step_type === 'decision' && step.output && (
                    <div style={{ fontSize: 12, color: '#fa8c16', marginTop: 4 }}>
                      ✅ {truncate(step.output, 80)}
                    </div>
                  )}
                  {isActive && step.step_type === 'action' && step.input && (
                    <div style={{ fontSize: 12, color: '#1890ff', marginTop: 4 }}>
                      👤 {truncate(step.input, 80)}
                    </div>
                  )}
                </Space>
              </Card>
            ),
          };
        })}
      />
    );
  };

  /** 渲染播放控制条 */
  const renderPlaybackControls = () => {
    const progressPercent = steps.length > 0 ? ((currentStepIndex + 1) / steps.length) * 100 : 0;

    return (
      <Card style={{ marginBottom: 16 }}>
        <Space split={<div style={{ width: 1, height: 32, background: "#d9d9d9" }} />}>
          <Space>
            <Tooltip title="上一步">
              <Button icon={<StepBackwardOutlined />} onClick={stepBackward} disabled={currentStepIndex === 0} />
            </Tooltip>
            {isPlaying ? (
              <Tooltip title="暂停">
                <Button type="primary" icon={<PauseCircleOutlined />} onClick={pause} />
              </Tooltip>
            ) : (
              <Tooltip title="播放">
                <Button type="primary" icon={<PlayCircleOutlined />} onClick={play} disabled={currentStepIndex >= steps.length - 1} />
              </Tooltip>
            )}
            <Tooltip title="下一步">
              <Button icon={<StepForwardOutlined />} onClick={stepForward} disabled={currentStepIndex >= steps.length - 1} />
            </Tooltip>
          </Space>
          <Space>
            <span style={{ fontSize: 12 }}>速度:</span>
            <Slider
              min={0.5}
              max={4}
              step={0.5}
              value={playbackSpeed}
              onChange={(v) => setPlaybackSpeed(v)}
              style={{ width: 100 }}
              marks={{ 0.5: "0.5x", 1: "1x", 2: "2x", 4: "4x" }}
            />
          </Space>
          <Space>
            <div style={{ position: "relative", width: 150 }}>
              <Progress percent={progressPercent} showInfo={false} style={{ width: 150 }} />
              {checkpoints.map((cp) => {
                const leftPercent = steps.length > 0 ? (cp.step / steps.length) * 100 : 0;
                return (
                  <Tooltip
                    key={cp.id}
                    title={`Step ${cp.step}${cp.timestamp ? " — " + dayjs(cp.timestamp * 1000).format("HH:mm:ss") : ""}`}
                  >
                    <div
                      onClick={() => openResumeModal(cp)}
                      style={{
                        position: "absolute",
                        left: `${leftPercent}%`,
                        top: -3,
                        width: 10,
                        height: 10,
                        borderRadius: "50%",
                        background: "#722ed1",
                        border: "2px solid #fff",
                        cursor: "pointer",
                        transform: "translateX(-50%)",
                        zIndex: 1,
                        boxShadow: "0 0 3px rgba(114, 46, 209, 0.5)",
                      }}
                    />
                  </Tooltip>
                );
              })}
            </div>
            <span style={{ fontSize: 12 }}>Step {currentStepIndex + 1} / {steps.length}</span>
          </Space>
          <Space>
            <Tooltip title="刷新">
              <Button icon={<ReloadOutlined />} onClick={loadReplayData} />
            </Tooltip>
            <Tooltip title="导出">
              <Button icon={<DownloadOutlined />} onClick={() => {
                const baseUrl = import.meta.env.VITE_API_URL || "";
                window.open(baseUrl + "/api/v2/sessions/" + sessionId + "/export?format=json", "_blank");
              }} />
            </Tooltip>
          </Space>
        </Space>
      </Card>
    );
  };

  /** 渲染 Checkpoint 列表 */
  const renderCheckpointList = () => {
    if (checkpoints.length === 0) return null;

    const columns = [
      {
        title: "Step",
        dataIndex: "step",
        key: "step",
        width: 80,
        render: (step: number) => <Tag color="purple">{step}</Tag>,
      },
      {
        title: "Agent ID",
        dataIndex: "agent_id",
        key: "agent_id",
        width: 160,
        render: (agentId: string | undefined) => agentId || "-",
      },
      {
        title: "Tokens",
        dataIndex: "tokens",
        key: "tokens",
        width: 120,
        render: (tokens: number | undefined) => tokens != null ? tokens.toLocaleString() : "-",
      },
      {
        title: "Time",
        dataIndex: "timestamp",
        key: "timestamp",
        width: 160,
        render: (ts: number | undefined) => ts ? dayjs(ts * 1000).format("HH:mm:ss") : "-",
      },
      {
        title: "操作",
        key: "action",
        width: 160,
        render: (_: unknown, record: Checkpoint) => (
          <Space>
            <Button
              size="small"
              onClick={() => {
                const idx = steps.findIndex((s) => s.step_number === record.step);
                if (idx >= 0) goToStep(idx);
              }}
            >
              跳转
            </Button>
            <Button
              size="small"
              type="primary"
              icon={<RedoOutlined />}
              onClick={() => openResumeModal(record)}
            >
              恢复
            </Button>
          </Space>
        ),
      },
    ];

    return (
      <Card
        title={
          <Space>
            <SaveOutlined style={{ color: "#722ed1" }} />
            <span>Checkpoints</span>
            <Badge count={checkpoints.length} style={{ backgroundColor: "#722ed1" }} />
          </Space>
        }
        style={{ marginBottom: 16 }}
      >
        <Table
          dataSource={checkpoints}
          columns={columns}
          rowKey="id"
          size="small"
          pagination={false}
          onRow={(record) => ({
            onClick: () => {
              const idx = steps.findIndex((s) => s.step_number === record.step);
              if (idx >= 0) goToStep(idx);
            },
            style: { cursor: "pointer" },
          })}
        />
      </Card>
    );
  };

  if (loading) {
    return <Spin size="large" style={{ display: "flex", justifyContent: "center", alignItems: "center", height: "100vh" }} />;
  }

  if (!session) {
    return <Empty description="Session 不存在" />;
  }

  if (steps.length === 0) {
    return (
      <div>
        <h2 style={{ marginBottom: 24 }}>Session 回放: {session.id}</h2>
        <Card style={{ marginBottom: 16 }}>
          <Descriptions column={6}>
            <Descriptions.Item label="Agent">{session.agent_id}</Descriptions.Item>
            <Descriptions.Item label="状态"><Badge status={getStatusColor(session.status || "") as "success" | "processing" | "error" | "warning" | "default"} text={session.status} /></Descriptions.Item>
            <Descriptions.Item label="Tokens">{session.total_tokens != null ? session.total_tokens.toLocaleString() : "-"}</Descriptions.Item>
            <Descriptions.Item label="费用">{session.total_cost != null ? "$" + session.total_cost.toFixed(4) : "-"}</Descriptions.Item>
            <Descriptions.Item label="时长">{formatDuration(session.duration)}</Descriptions.Item>
            <Descriptions.Item label="开始时间">{session.start_time ? dayjs(session.start_time * 1000).format("YYYY-MM-DD HH:mm:ss") : "-"}</Descriptions.Item>
          </Descriptions>
        </Card>
        <Empty description="此 Session 暂无消息可回放" />
      </div>
    );
  }

  const tabItems = [
    {
      key: "graph",
      label: "执行图",
      children: (
        <Card>
          <div style={{ overflowX: "auto", padding: 16 }}>
            {renderGraph()}
          </div>
        </Card>
      ),
    },
    {
      key: "timeline",
      label: "步骤时间线",
      children: <Card>{renderTimeline()}</Card>,
    },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 24 }}>Session 回放: {session.id}</h2>
      <Card style={{ marginBottom: 16 }}>
        <Descriptions column={6}>
          <Descriptions.Item label="Agent">{session.agent_id}</Descriptions.Item>
          <Descriptions.Item label="状态"><Badge status={getStatusColor(session.status || "") as "success" | "processing" | "error" | "warning" | "default"} text={session.status} /></Descriptions.Item>
          <Descriptions.Item label="Tokens">{session.total_tokens != null ? session.total_tokens.toLocaleString() : "-"}</Descriptions.Item>
          <Descriptions.Item label="费用">{session.total_cost != null ? "$" + session.total_cost.toFixed(4) : "-"}</Descriptions.Item>
          <Descriptions.Item label="时长">{formatDuration(session.duration)}</Descriptions.Item>
          <Descriptions.Item label="开始时间">{session.start_time ? dayjs(session.start_time * 1000).format("YYYY-MM-DD HH:mm:ss") : "-"}</Descriptions.Item>
        </Descriptions>
      </Card>
      {renderPlaybackControls()}
      {renderCheckpointList()}
      <Tabs defaultActiveKey={searchParams.get("tab") || "graph"} items={tabItems} onChange={(key) => setSearchParams({ tab: key })} />
      {selectedStep && (
        <StepDetail
          step={selectedStep}
          open={!!selectedStep}
          onClose={() => setSelectedStep(null)}
        />
      )}
      <Modal
        open={resumeModalVisible}
        title="恢复 Checkpoint"
        onCancel={() => {
          setResumeModalVisible(false);
          setSelectedCheckpoint(null);
        }}
        onOk={handleResumeCheckpoint}
        confirmLoading={resuming}
        okText="恢复"
        cancelText="取消"
      >
        <p>是否从 <Tag color="purple">Step {selectedCheckpoint?.step}</Tag> 恢复执行？</p>
        {selectedCheckpoint?.agent_id && (
          <p style={{ color: "#666", fontSize: 12 }}>Agent: {selectedCheckpoint.agent_id}</p>
        )}
        {selectedCheckpoint?.timestamp && (
          <p style={{ color: "#666", fontSize: 12 }}>时间: {dayjs(selectedCheckpoint.timestamp * 1000).format("YYYY-MM-DD HH:mm:ss")}</p>
        )}
      </Modal>
    </div>
  );
}

/** 截断文本 */
function truncate(text: string, maxLen: number): string {
  const clean = text.replace(/\n/g, ' ').trim();
  return clean.length > maxLen ? clean.slice(0, maxLen) + '...' : clean;
}
