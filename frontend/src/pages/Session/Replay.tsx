import { useState, useEffect, useRef, useCallback } from "react";
import { useParams, useSearchParams } from "react-router-dom";
import {
  Card, Tabs, Timeline, Button, Space, Slider, Tag, Badge, Descriptions, Empty,
  Spin, Drawer, Collapse, Progress, Tooltip
} from "antd";
import {
  PlayCircleOutlined, PauseCircleOutlined, StepForwardOutlined, StepBackwardOutlined,
  DownloadOutlined, ReloadOutlined
} from "@ant-design/icons";
import dayjs from "dayjs";
import client from "../../api/client";
import type { Session, SessionStep, ExecutionGraph, ReplayState } from "../../api/session";

const { Panel } = Collapse;

const STEP_TYPE_LABELS: Record<string, string> = {
  think: "Think",
  tool_call: "Tool Call",
  action: "Action",
  observation: "Observation",
  decision: "Decision",
  llm_call: "LLM Call",
};

function formatStepType(step_type?: string): string {
  if (!step_type) return "Unknown";
  return STEP_TYPE_LABELS[step_type] || step_type;
}

export default function SessionReplayPage() {
  const { session_id: sessionId } = useParams<{ session_id: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const [session, setSession] = useState<Session | null>(null);
  const [steps, setSteps] = useState<SessionStep[]>([]);
  const [graph, setGraph] = useState<ExecutionGraph | null>(null);
  const [loading, setLoading] = useState(true);
  const [currentStepIndex, setCurrentStepIndex] = useState(0);
  const [isPlaying, setIsPlaying] = useState(false);
  const [playbackSpeed, setPlaybackSpeed] = useState(1);
  const [selectedStep, setSelectedStep] = useState<SessionStep | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const loadReplayData = async () => {
    if (!sessionId) return;
    setLoading(true);
    try {
      // Option A: Make 3 separate calls and combine results
      const [sessionRes, replayRes, graphRes] = await Promise.all([
        client.get("/api/v2/harness/session/" + sessionId) as any,
        client.post("/api/v2/harness/session/" + sessionId + "/replay") as any,
        client.get("/api/v2/harness/session/" + sessionId + "/graph") as any,
      ]);
      setSession(sessionRes?.session || sessionRes);
      setSteps(replayRes?.steps || []);
      setGraph(graphRes?.graph || graphRes);
    } catch {
      // Mock data for development - uses camelCase matching protobuf JSON
      const mockSession: any = {
        id: sessionId,
        agent_id: "browser-agent",
        status: "completed",
        total_tokens: 2500,
        total_cost: 0.12,
        duration: 65000,
        start_time: 1705312200000,
        end_time: 1705312265000,
      };
      const mockSteps: any[] = [
        { id: "step1", session_id: sessionId, step_number: 1, step_type: "action", input: JSON.stringify({ url: "https://example.com" }), output: JSON.stringify({ status: "success" }), status: "completed", duration: 15000, timestamp: 1705312200000 },
        { id: "step2", session_id: sessionId, step_number: 2, step_type: "tool_call", input: JSON.stringify({ selector: "#login-btn" }), output: JSON.stringify({ status: "success" }), status: "completed", duration: 5000, timestamp: 1705312215000 },
        { id: "step3", session_id: sessionId, step_number: 3, step_type: "action", input: JSON.stringify({ selector: "#username", text: "user@example.com" }), output: JSON.stringify({ status: "success" }), status: "completed", duration: 5000, timestamp: 1705312220000 },
        { id: "step4", session_id: sessionId, step_number: 4, step_type: "action", input: JSON.stringify({ selector: "#password", text: "***" }), output: JSON.stringify({ status: "success" }), status: "completed", duration: 5000, timestamp: 1705312225000 },
        { id: "step5", session_id: sessionId, step_number: 5, step_type: "tool_call", input: JSON.stringify({ selector: "#submit-btn" }), output: JSON.stringify({ status: "success" }), status: "completed", duration: 35000, timestamp: 1705312230000 },
      ];
      const mockGraph: any = {
        nodes: [
          { id: "start", type: "start", label: "Start", status: "completed" },
          { id: "step1", type: "action", label: "Navigate", status: "completed" },
          { id: "step2", type: "action", label: "Click Login", status: "completed" },
          { id: "step3", type: "action", label: "Type Username", status: "completed" },
          { id: "step4", type: "action", label: "Type Password", status: "completed" },
          { id: "step5", type: "action", label: "Submit", status: "completed" },
          { id: "end", type: "end", label: "End", status: "completed" },
        ],
        edges: [
          { from: "start", to: "step1" },
          { from: "step1", to: "step2" },
          { from: "step2", to: "step3" },
          { from: "step3", to: "step4" },
          { from: "step4", to: "step5" },
          { from: "step5", to: "end" },
        ],
      };
      setSession(mockSession);
      setSteps(mockSteps);
      setGraph(mockGraph);
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

  const renderGraph = () => {
    if (!graph) return <Empty description="No graph data" />;
    const nodeWidth = 160;
    const nodeHeight = 60;
    const nodeSpacing = 40;
    const startX = 50;
    const startY = 100;
    const getNodeColor = (status: string) => {
      const map: Record<string, string> = {
        pending: "#d9d9d9",
        running: "#1677ff",
        completed: "#52c41a",
        failed: "#ff4d4f",
      };
      return map[status] || "#d9d9d9";
    };
    const totalWidth = graph.nodes.length * (nodeWidth + nodeSpacing) + startX;
    return (
      <svg width={totalWidth} height={200} style={{ overflow: "visible" }}>
        {graph.edges.map((edge, index) => {
          const sourceIndex = graph.nodes.findIndex((n) => n.id === edge.from);
          const targetIndex = graph.nodes.findIndex((n) => n.id === edge.to);
          if (sourceIndex < 0 || targetIndex < 0) return null;
          const sourceX = startX + sourceIndex * (nodeWidth + nodeSpacing) + nodeWidth;
          const sourceY = startY + nodeHeight / 2;
          const targetX = startX + targetIndex * (nodeWidth + nodeSpacing);
          const targetY = startY + nodeHeight / 2;
          return (
            <line key={edge.from + "-" + edge.to + "-" + index} x1={sourceX} y1={sourceY} x2={targetX} y2={targetY} stroke="#b1b1b7" strokeWidth="2" markerEnd="url(#arrowhead)" />
          );
        })}
        <defs>
          <marker id="arrowhead" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">
            <polygon points="0 0, 10 3.5, 0 7" fill="#b1b1b7" />
          </marker>
        </defs>
        {graph.nodes.map((node, index) => {
          const x = startX + index * (nodeWidth + nodeSpacing);
          const y = startY;
          const isActive = index <= currentStepIndex;
          return (
            <g key={node.id} onClick={() => goToStep(index)} style={{ cursor: "pointer" }}>
              <rect
                x={x}
                y={y}
                width={nodeWidth}
                height={nodeHeight}
                rx={8}
                fill={isActive ? getNodeColor(node.status || "") : "#f0f0f0"}
                stroke={isActive ? getNodeColor(node.status || "") : "#d9d9d9"}
                strokeWidth={2}
              />
              <text x={x + nodeWidth / 2} y={y + nodeHeight / 2} textAnchor="middle" dominantBaseline="middle" fill={isActive ? "#fff" : "#666"} fontSize={12}>
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

  const renderTimeline = () => {
    return (
      <Timeline
        items={steps.map((step, index) => ({
          key: step.id,
          color: index <= currentStepIndex ? getStatusColor(step.status || "") : "gray",
          dot: index === currentStepIndex ? <Spin /> : undefined,
          children: (
            <Card
              size="small"
              style={{ cursor: "pointer" }}
              onClick={() => { setSelectedStep(step); goToStep(index); }}
            >
              <Space direction="vertical" size="small">
                <Space>
                  <Tag color="blue">Step {step.step_number}</Tag>
                  <Tag color="purple">{formatStepType(step.step_type)}</Tag>
                  <Badge status={getStatusColor(step.status || "") as "success" | "processing" | "error" | "warning" | "default"} />
                </Space>
                <div style={{ fontSize: 12, color: "#666" }}>
                  {step.timestamp ? dayjs(step.timestamp).format("HH:mm:ss") : "-"}
                </div>
                {index <= currentStepIndex && (
                  <div style={{ fontSize: 12 }}>
                    Duration: {formatDuration(step.duration)}
                  </div>
                )}
              </Space>
            </Card>
          ),
        }))}
      />
    );
  };

  const renderPlaybackControls = () => {
    return (
      <Card style={{ marginBottom: 16 }}>
        <Space split={<div style={{ width: 1, height: 32, background: "#d9d9d9" }} />}>
          <Space>
            <Tooltip title="Step Backward">
              <Button icon={<StepBackwardOutlined />} onClick={stepBackward} disabled={currentStepIndex === 0} />
            </Tooltip>
            {isPlaying ? (
              <Tooltip title="Pause">
                <Button type="primary" icon={<PauseCircleOutlined />} onClick={pause} />
              </Tooltip>
            ) : (
              <Tooltip title="Play">
                <Button type="primary" icon={<PlayCircleOutlined />} onClick={play} disabled={currentStepIndex >= steps.length - 1} />
              </Tooltip>
            )}
            <Tooltip title="Step Forward">
              <Button icon={<StepForwardOutlined />} onClick={stepForward} disabled={currentStepIndex >= steps.length - 1} />
            </Tooltip>
          </Space>
          <Space>
            <span style={{ fontSize: 12 }}>Speed:</span>
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
            <Progress percent={((currentStepIndex + 1) / steps.length) * 100} showInfo={false} style={{ width: 150 }} />
            <span style={{ fontSize: 12 }}>Step {currentStepIndex + 1} / {steps.length}</span>
          </Space>
          <Space>
            <Tooltip title="Reload">
              <Button icon={<ReloadOutlined />} onClick={loadReplayData} />
            </Tooltip>
            <Tooltip title="Export">
              <Button icon={<DownloadOutlined />} onClick={() => {
                const baseUrl = import.meta.env.VITE_API_URL || "http://192.168.10.100:9000";
                window.open(baseUrl + "/api/v2/harness/session/" + sessionId + "/export?format=json", "_blank");
              }} />
            </Tooltip>
          </Space>
        </Space>
      </Card>
    );
  };

  const renderStepDetail = () => {
    if (!selectedStep) return null;
    return (
      <Drawer
        title="Step Details"
        placement="right"
        width={500}
        open={true}
        onClose={() => setSelectedStep(null)}
      >
        <Descriptions bordered column={1}>
          <Descriptions.Item label="Step Number">{selectedStep.step_number}</Descriptions.Item>
          <Descriptions.Item label="Step Type"><Tag color="purple">{formatStepType(selectedStep.step_type)}</Tag></Descriptions.Item>
          <Descriptions.Item label="Status"><Badge status={getStatusColor(selectedStep.status || "") as "success" | "processing" | "error" | "warning" | "default"} text={selectedStep.status} /></Descriptions.Item>
          <Descriptions.Item label="Timestamp">{selectedStep.timestamp ? dayjs(selectedStep.timestamp).format("YYYY-MM-DD HH:mm:ss") : "-"}</Descriptions.Item>
          <Descriptions.Item label="Duration">{formatDuration(selectedStep.duration)}</Descriptions.Item>
        </Descriptions>
        <Collapse style={{ marginTop: 16 }}>
          <Panel header="Input" key="input">
            <pre style={{ fontSize: 12, overflow: "auto", maxHeight: 200 }}>
              {selectedStep.input
                ? (typeof selectedStep.input === "string"
                  ? (() => { try { return JSON.stringify(JSON.parse(selectedStep.input), null, 2); } catch { return selectedStep.input; } })()
                  : JSON.stringify(selectedStep.input, null, 2))
                : "No input"}
            </pre>
          </Panel>
          <Panel header="Output" key="output">
            <pre style={{ fontSize: 12, overflow: "auto", maxHeight: 200 }}>
              {selectedStep.output
                ? (typeof selectedStep.output === "string"
                  ? (() => { try { return JSON.stringify(JSON.parse(selectedStep.output), null, 2); } catch { return selectedStep.output; } })()
                  : JSON.stringify(selectedStep.output, null, 2))
                : "No output"}
            </pre>
          </Panel>
          {selectedStep.metadata && (
            <Panel header="Metadata" key="metadata">
              <pre style={{ fontSize: 12, overflow: "auto", maxHeight: 200 }}>
                {typeof selectedStep.metadata === "string"
                  ? (() => { try { return JSON.stringify(JSON.parse(selectedStep.metadata), null, 2); } catch { return selectedStep.metadata; } })()
                  : JSON.stringify(selectedStep.metadata, null, 2)}
              </pre>
            </Panel>
          )}
        </Collapse>
      </Drawer>
    );
  };

  if (loading) {
    return <Spin size="large" style={{ display: "flex", justifyContent: "center", alignItems: "center", height: "100vh" }} />;
  }

  if (!session) {
    return <Empty description="Session not found" />;
  }

  const tabItems = [
    {
      key: "graph",
      label: "Execution Graph",
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
      label: "Step Timeline",
      children: <Card>{renderTimeline()}</Card>,
    },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 24 }}>Session Replay: {session.id}</h2>
      <Card style={{ marginBottom: 16 }}>
        <Descriptions column={6}>
          <Descriptions.Item label="Agent">{session.agent_id}</Descriptions.Item>
          <Descriptions.Item label="Status"><Badge status={getStatusColor(session.status || "") as "success" | "processing" | "error" | "warning" | "default"} text={session.status} /></Descriptions.Item>
          <Descriptions.Item label="Tokens">{session.total_tokens?.toLocaleString()}</Descriptions.Item>
          <Descriptions.Item label="Cost">${session.total_cost?.toFixed(4)}</Descriptions.Item>
          <Descriptions.Item label="Duration">{formatDuration(session.duration)}</Descriptions.Item>
          <Descriptions.Item label="Started">{session.start_time ? dayjs(session.start_time).format("YYYY-MM-DD HH:mm:ss") : "-"}</Descriptions.Item>
        </Descriptions>
      </Card>
      {renderPlaybackControls()}
      <Tabs defaultActiveKey={searchParams.get("tab") || "graph"} items={tabItems} onChange={(key) => setSearchParams({ tab: key })} />
      {renderStepDetail()}
    </div>
  );
}
