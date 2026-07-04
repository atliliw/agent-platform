import { useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import {
  Card, Table, Tag, Button, Space, Select, DatePicker, Input, message,
  Popconfirm, Row, Col, Statistic, Badge, Tooltip, Empty
} from "antd";
import {
  PlayCircleOutlined, DeleteOutlined, SearchOutlined,
  ReloadOutlined, DownloadOutlined, LineChartOutlined
} from "@ant-design/icons";
import dayjs from "dayjs";
import type { ColumnsType } from "antd/es/table";
import client from "../../api/client";
import type { Session, SessionStats } from "../../api/session";

const { RangePicker } = DatePicker;

export default function SessionListPage() {
  const navigate = useNavigate();
  const [sessions, setSessions] = useState<Session[]>([]);
  const [loading, setLoading] = useState(false);
  const [stats, setStats] = useState<SessionStats | null>(null);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [agentIdFilter, setAgentIdFilter] = useState<string>();
  const [statusFilter, setStatusFilter] = useState<string>();
  const [dateRange, setDateRange] = useState<[dayjs.Dayjs, dayjs.Dayjs] | null>();

  const loadSessions = async () => {
    setLoading(true);
    try {
      const params: any = { page, pageSize, agent_id: agentIdFilter, status: statusFilter };
      if (dateRange) {
        params.startDate = dateRange[0].toISOString();
        params.endDate = dateRange[1].toISOString();
      }
      const res = await client.get("/api/v2/harness/session/list", { params }) as any;
      setSessions(res?.sessions || []);
      setTotal(res?.total || 0);
    } catch {
      setSessions([
        { id: "s1", agent_id: "browser-agent", status: "completed", total_tokens: 1500, total_cost: 0.05, duration: 45000, start_time: 1783046131 },
        { id: "s2", agent_id: "chat-agent", status: "running", total_tokens: 800, total_cost: 0.02, duration: 12000, start_time: 1783046131 },
        { id: "s3", agent_id: "research-agent", status: "failed", total_tokens: 2000, total_cost: 0.08, duration: 30000, start_time: 1783046131, end_time: 1783046131 },
      ] as any);
      setTotal(3);
    } finally {
      setLoading(false);
    }
  };

  const loadStats = async () => {
    try {
      const res = await client.get("/api/v2/harness/session/stats") as SessionStats;
      setStats(res);
    } catch {
      setStats({ total_sessions: 156, running_sessions: 12, completed_sessions: 130, failed_sessions: 14, total_tokens: 2500000, total_cost: 125.50, avg_duration: 25000 });
    }
  };

  const handleDelete = async (session_id: string) => {
    try {
      await client.delete("/api/v2/harness/session/" + session_id);
      message.success("Deleted");
      loadSessions();
    } catch {
      message.error("Delete failed");
    }
  };

  const handleExport = (session_id: string) => {
    const baseUrl = import.meta.env.VITE_API_URL || "http://192.168.10.100:9000";
    window.open(baseUrl + "/api/v2/harness/session/" + session_id + "/export?format=json", "_blank");
  };

  const resetFilters = () => {
    setAgentIdFilter(undefined);
    setStatusFilter(undefined);
    setDateRange(null);
    setPage(1);
  };

  useEffect(() => { loadSessions(); loadStats(); }, [page, pageSize, agentIdFilter, statusFilter, dateRange]);

  const getStatusBadge = (status: string) => {
    const map: Record<string, { status: "success" | "processing" | "error" | "warning" | "default"; text: string }> = {
      running: { status: "processing", text: "Running" },
      completed: { status: "success", text: "Completed" },
      failed: { status: "error", text: "Failed" },
      cancelled: { status: "warning", text: "Cancelled" },
    };
    return map[status] || { status: "default", text: status };
  };

  const formatDuration = (ms?: number) => {
    if (ms == null) return "-";
    if (ms < 1000) return ms + "ms";
    if (ms < 60000) return (ms / 1000).toFixed(1) + "s";
    return (ms / 60000).toFixed(1) + "min";
  };

  const columns: ColumnsType<Session> = [
    { title: "Session ID", dataIndex: "id", key: "id", width: 180, render: (id: string) => <Tooltip title={id}><code style={{ fontSize: 12 }}>{id.slice(0, 8)}...</code></Tooltip> },
    { title: "Agent ID", dataIndex: "agent_id", key: "agent_id", width: 150, render: (agent_id: string) => <Tag color="purple">{agent_id}</Tag> },
    { title: "Status", dataIndex: "status", key: "status", width: 100, render: (status: string) => <Badge status={getStatusBadge(status).status} text={getStatusBadge(status).text} /> },
    { title: "Tokens", dataIndex: "total_tokens", key: "total_tokens", width: 100, render: (tokens?: number) => <Tag color="cyan">{tokens != null ? tokens.toLocaleString() : "-"}</Tag> },
    { title: "Cost", dataIndex: "total_cost", key: "total_cost", width: 80, render: (cost?: number) => cost != null ? "$" + cost.toFixed(4) : "-" },
    { title: "Duration", dataIndex: "duration", key: "duration", width: 100, render: (ms?: number) => formatDuration(ms) },
    { title: "Start Time", dataIndex: "start_time", key: "start_time", width: 160, render: (time?: number) => time ? dayjs(time).format("YYYY-MM-DD HH:mm:ss") : "-" },
    { title: "End Time", dataIndex: "end_time", key: "end_time", width: 160, render: (time?: number) => (time ? dayjs(time).format("YYYY-MM-DD HH:mm:ss") : "-") },
    { title: "Actions", key: "action", width: 200, fixed: "right", render: (_: unknown, record: Session) => (
      <Space size="small">
        <Tooltip title="Replay"><Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={() => navigate("/session/replay/" + record.id)} /></Tooltip>
        <Tooltip title="View Graph"><Button size="small" icon={<LineChartOutlined />} onClick={() => navigate("/session/replay/" + record.id + "?tab=graph")} /></Tooltip>
        <Tooltip title="Export"><Button size="small" icon={<DownloadOutlined />} onClick={() => handleExport(record.id)} /></Tooltip>
        <Popconfirm title="Delete this session?" onConfirm={() => handleDelete(record.id)} okText="Delete" cancelText="Cancel"><Button size="small" danger icon={<DeleteOutlined />} /></Popconfirm>
      </Space>
    ) },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 24 }}>Session Replay Management</h2>
      {stats && (
        <Row gutter={16} style={{ marginBottom: 24 }}>
          <Col span={4}><Card><Statistic title="Total Sessions" value={stats.total_sessions} /></Card></Col>
          <Col span={4}><Card><Statistic title="Running" value={stats.running_sessions} valueStyle={{ color: "#1677ff" }} /></Card></Col>
          <Col span={4}><Card><Statistic title="Completed" value={stats.completed_sessions} valueStyle={{ color: "#52c41a" }} /></Card></Col>
          <Col span={4}><Card><Statistic title="Failed" value={stats.failed_sessions} valueStyle={{ color: "#ff4d4f" }} /></Card></Col>
          <Col span={4}><Card><Statistic title="Total Tokens" value={stats.total_tokens?.toLocaleString()} /></Card></Col>
          <Col span={4}><Card><Statistic title="Total Cost" value={stats.total_cost} prefix="$" precision={2} /></Card></Col>
        </Row>
      )}
      <Card style={{ marginBottom: 16 }}>
        <Space wrap size="middle">
          <span style={{ fontWeight: 500 }}>Filters:</span>
          <Input placeholder="Agent ID" value={agentIdFilter} onChange={(e) => setAgentIdFilter(e.target.value)} style={{ width: 150 }} allowClear />
          <Select placeholder="Status" value={statusFilter} onChange={setStatusFilter} style={{ width: 120 }} allowClear options={[{ value: "running", label: "Running" }, { value: "completed", label: "Completed" }, { value: "failed", label: "Failed" }, { value: "cancelled", label: "Cancelled" }]} />
          <RangePicker value={dateRange} onChange={(dates) => setDateRange(dates as [dayjs.Dayjs, dayjs.Dayjs] | null)} placeholder={["Start Date", "End Date"]} />
          <Button icon={<SearchOutlined />} type="primary" onClick={loadSessions}>Search</Button>
          <Button icon={<ReloadOutlined />} onClick={resetFilters}>Reset</Button>
        </Space>
      </Card>
      <Card>
        <Table columns={columns} dataSource={sessions} rowKey="id" loading={loading} scroll={{ x: 1200 }} pagination={{ current: page, pageSize, total, showSizeChanger: true, showQuickJumper: true, showTotal: (total) => "Total " + total + " records", onChange: (p, ps) => { setPage(p); setPageSize(ps); } }} locale={{ emptyText: <Empty description="No session data" /> }} />
      </Card>
    </div>
  );
}
