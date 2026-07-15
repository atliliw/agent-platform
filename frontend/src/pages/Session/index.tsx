import { useState, useEffect, useMemo } from "react";
import { useNavigate } from "react-router-dom";
import {
  Card, Table, Tag, Button, Space, Select, Input, message,
  Popconfirm, Row, Col, Statistic, Badge, Tooltip, Empty, Switch
} from "antd";
import {
  PlayCircleOutlined, DeleteOutlined, SearchOutlined,
  ReloadOutlined, DownloadOutlined, LineChartOutlined,
  ClearOutlined, ExclamationCircleOutlined
} from "@ant-design/icons";
import dayjs from "dayjs";
import type { ColumnsType } from "antd/es/table";
import client from "../../api/client";
import type { Session, SessionStats } from "../../api/session";

export default function SessionListPage() {
  const navigate = useNavigate();
  const [sessions, setSessions] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [stats, setStats] = useState<SessionStats | null>(null);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [agentIdFilter, setAgentIdFilter] = useState<string>();
  const [statusFilter, setStatusFilter] = useState<string>();
  const [hideEmpty, setHideEmpty] = useState(true);
  const [deletingEmpty, setDeletingEmpty] = useState(false);

  const loadSessions = async () => {
    setLoading(true);
    try {
      const res = await client.get("/api/v2/sessions", {
        params: { page, page_size: pageSize }
      }) as any;
      const raw = res?.sessions || res?.data?.sessions || [];
      const mapped = raw.map((s: any) => ({
        id: s.id,
        tenant_id: s.tenant_id || "",
        user_id: s.user_id || "",
        title: s.title || "New Chat",
        agent_id: s.agent_id || "main-agent",
        status: s.status || "completed",
        message_count: s.message_count ?? 0,
        total_tokens: s.total_tokens,
        total_cost: s.total_cost,
        duration: s.duration,
        start_time: s.created_at,
        end_time: s.updated_at,
        created_at: s.created_at,
        updated_at: s.updated_at,
      }));
      setSessions(mapped);
      setTotal(res?.pagination?.total || mapped.length);
    } catch {
      setSessions([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  };

  const loadStats = async () => {
    try {
      const res = await client.get("/api/v2/sessions", {
        params: { page: 1, page_size: 1 }
      }) as any;
      const total = res?.pagination?.total || 0;
      setStats({
        total_sessions: total,
        running_sessions: 0,
        completed_sessions: total,
        failed_sessions: 0,
        total_tokens: 0,
        total_cost: 0,
        avg_duration: 0,
      });
    } catch {
      setStats({
        total_sessions: 0,
        running_sessions: 0,
        completed_sessions: 0,
        failed_sessions: 0,
        total_tokens: 0,
        total_cost: 0,
        avg_duration: 0,
      });
    }
  };

  const handleDelete = async (session_id: string) => {
    try {
      await client.delete("/api/v2/sessions/" + session_id);
      message.success("Deleted");
      loadSessions();
    } catch {
      message.error("Delete failed");
    }
  };

  const handleDeleteEmptySessions = async () => {
    setDeletingEmpty(true);
    try {
      const res = await client.delete("/api/v2/sessions/empty") as any;
      const data = res?.data || res;
      const deleted = data?.deleted ?? 0;
      const failed = data?.failed ?? 0;
      if (deleted > 0) {
        message.success(`Deleted ${deleted} empty sessions${failed > 0 ? `, ${failed} failed` : ""}`);
      } else {
        message.info("No empty sessions found");
      }
      loadSessions();
      loadStats();
    } catch {
      message.error("Failed to delete empty sessions");
    } finally {
      setDeletingEmpty(false);
    }
  };

  const handleExport = (session_id: string) => {
    const baseUrl = import.meta.env.VITE_API_URL || "";
    window.open(baseUrl + "/api/v2/sessions/" + session_id + "/export?format=json", "_blank");
  };

  const resetFilters = () => {
    setAgentIdFilter(undefined);
    setStatusFilter(undefined);
    setPage(1);
  };

  useEffect(() => {
    loadSessions();
    loadStats();
  }, [page, pageSize]);

  // Filter sessions client-side based on hideEmpty toggle
  const filteredSessions = useMemo(() => {
    if (!hideEmpty) return sessions;
    return sessions.filter((s) => s.message_count > 0);
  }, [sessions, hideEmpty]);

  const emptyCount = useMemo(() => {
    return sessions.filter((s) => s.message_count === 0).length;
  }, [sessions]);

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

  const columns: ColumnsType<any> = [
    {
      title: "Session ID",
      dataIndex: "id",
      key: "id",
      width: 180,
      render: (id: string) => (
        <Tooltip title={id}>
          <code style={{ fontSize: 12 }}>{id.slice(0, 8)}...</code>
        </Tooltip>
      ),
    },
    {
      title: "Title",
      dataIndex: "title",
      key: "title",
      width: 160,
      ellipsis: true,
      render: (title: string, record: any) =>
        record.message_count === 0 ? (
          <span style={{ color: "#999" }}>{title}</span>
        ) : (
          title
        ),
    },
    {
      title: "Messages",
      dataIndex: "message_count",
      key: "message_count",
      width: 90,
      sorter: (a: any, b: any) => a.message_count - b.message_count,
      render: (count: number) =>
        count === 0 ? (
          <Tag color="default">Empty</Tag>
        ) : (
          <Tag color="blue">{count}</Tag>
        ),
    },
    {
      title: "Agent",
      dataIndex: "agent_id",
      key: "agent_id",
      width: 120,
      render: (agent_id: string) => <Tag color="purple">{agent_id}</Tag>,
    },
    {
      title: "Status",
      dataIndex: "status",
      key: "status",
      width: 100,
      render: (status: string) => (
        <Badge
          status={getStatusBadge(status).status}
          text={getStatusBadge(status).text}
        />
      ),
    },
    {
      title: "Tokens",
      dataIndex: "total_tokens",
      key: "total_tokens",
      width: 90,
      render: (tokens?: number) => (
        <Tag color="cyan">{tokens != null ? tokens.toLocaleString() : "-"}</Tag>
      ),
    },
    {
      title: "Created",
      dataIndex: "created_at",
      key: "created_at",
      width: 160,
      sorter: (a: any, b: any) => (a.created_at || 0) - (b.created_at || 0),
      render: (time?: number) =>
        time ? dayjs(typeof time === "number" ? (time > 1e12 ? time : time * 1000) : time).format("MM-DD HH:mm:ss") : "-",
    },
    {
      title: "Actions",
      key: "action",
      width: 200,
      fixed: "right",
      render: (_: unknown, record: any) => (
        <Space size="small">
          <Tooltip title="Replay">
            <Button
              size="small"
              type="primary"
              icon={<PlayCircleOutlined />}
              disabled={record.message_count === 0}
              onClick={() => navigate("/session/replay/" + record.id)}
            />
          </Tooltip>
          <Tooltip title="View Graph">
            <Button
              size="small"
              icon={<LineChartOutlined />}
              disabled={record.message_count === 0}
              onClick={() => navigate("/session/replay/" + record.id + "?tab=graph")}
            />
          </Tooltip>
          <Tooltip title="Export">
            <Button
              size="small"
              icon={<DownloadOutlined />}
              onClick={() => handleExport(record.id)}
            />
          </Tooltip>
          <Popconfirm
            title="Delete this session?"
            onConfirm={() => handleDelete(record.id)}
            okText="Delete"
            cancelText="Cancel"
          >
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 24 }}>Session Replay Management</h2>
      {stats && (
        <Row gutter={16} style={{ marginBottom: 24 }}>
          <Col span={4}>
            <Card>
              <Statistic title="Total Sessions" value={stats.total_sessions} />
            </Card>
          </Col>
          <Col span={4}>
            <Card>
              <Statistic
                title="With Messages"
                value={sessions.filter((s) => s.message_count > 0).length}
                valueStyle={{ color: "#52c41a" }}
              />
            </Card>
          </Col>
          <Col span={4}>
            <Card>
              <Statistic
                title="Empty Sessions"
                value={emptyCount}
                valueStyle={{ color: emptyCount > 0 ? "#ff4d4f" : "#999" }}
              />
            </Card>
          </Col>
          <Col span={4}>
            <Card>
              <Statistic title="Running" value={stats.running_sessions} valueStyle={{ color: "#1677ff" }} />
            </Card>
          </Col>
          <Col span={4}>
            <Card>
              <Statistic title="Failed" value={stats.failed_sessions} valueStyle={{ color: "#ff4d4f" }} />
            </Card>
          </Col>
          <Col span={4}>
            <Card>
              <Statistic title="Total Cost" value={stats.total_cost} prefix="$" precision={2} />
            </Card>
          </Col>
        </Row>
      )}
      <Card style={{ marginBottom: 16 }}>
        <Space wrap size="middle">
          <span style={{ fontWeight: 500 }}>Filters:</span>
          <Input
            placeholder="Agent ID"
            value={agentIdFilter}
            onChange={(e) => setAgentIdFilter(e.target.value)}
            style={{ width: 150 }}
            allowClear
          />
          <Select
            placeholder="Status"
            value={statusFilter}
            onChange={setStatusFilter}
            style={{ width: 120 }}
            allowClear
            options={[
              { value: "running", label: "Running" },
              { value: "completed", label: "Completed" },
              { value: "failed", label: "Failed" },
              { value: "cancelled", label: "Cancelled" },
            ]}
          />
          <span style={{ marginLeft: 8 }}>
            <Switch
              checked={hideEmpty}
              onChange={setHideEmpty}
              checkedChildren="Hide Empty"
              unCheckedChildren="Show All"
            />
          </span>
          <Button
            icon={<SearchOutlined />}
            type="primary"
            onClick={loadSessions}
          >
            Search
          </Button>
          <Button icon={<ReloadOutlined />} onClick={resetFilters}>
            Reset
          </Button>
          <Popconfirm
            title={`Delete all ${emptyCount} empty sessions? This cannot be undone.`}
            onConfirm={handleDeleteEmptySessions}
            okText="Delete All Empty"
            cancelText="Cancel"
            okButtonProps={{ danger: true }}
            icon={<ExclamationCircleOutlined />}
          >
            <Button
              danger
              icon={<ClearOutlined />}
              loading={deletingEmpty}
              disabled={emptyCount === 0}
            >
              Delete Empty ({emptyCount})
            </Button>
          </Popconfirm>
        </Space>
      </Card>
      <Card>
        <Table
          columns={columns}
          dataSource={filteredSessions}
          rowKey="id"
          loading={loading}
          scroll={{ x: 1200 }}
          rowClassName={(record) =>
            record.message_count === 0 ? "row-empty-session" : ""
          }
          pagination={{
            current: page,
            pageSize,
            total: hideEmpty ? filteredSessions.length : total,
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (t) => `Total ${t} records${hideEmpty ? ` (filtered from ${sessions.length})` : ""}`,
            onChange: (p, ps) => {
              setPage(p);
              setPageSize(ps);
            },
          }}
          locale={{ emptyText: <Empty description="No session data" /> }}
        />
      </Card>
      <style>{`
        .row-empty-session {
          opacity: 0.5;
          background: #fafafa;
        }
        .row-empty-session:hover td {
          background: #f5f5f5 !important;
        }
      `}</style>
    </div>
  );
}
