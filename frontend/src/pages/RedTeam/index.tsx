import { useState, useEffect } from "react";
import { Card, Table, Tag, Button, Space, message, Badge, Row, Col, Statistic, Alert, Popconfirm, Tooltip } from "antd";
import { SecurityScanOutlined, PlusOutlined, ReloadOutlined, DeleteOutlined, FileTextOutlined, PlayCircleOutlined } from "@ant-design/icons";
import { useNavigate } from "react-router-dom";
import { redteamApi } from "../../api/redteam";

export default function RedTeamPage() {
  const navigate = useNavigate();
  const [tests, setTests] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [stats, setStats] = useState({ total: 0, running: 0, completed: 0 });

  useEffect(() => { loadTests(); }, []);

  const loadTests = async () => {
    setLoading(true);
    try {
      const res = await redteamApi.listTests() as any;
      const testList = res?.tests || [];
      setTests(testList);
      setStats({
        total: testList.length,
        running: testList.filter((t: any) => t.status === "running").length,
        completed: testList.filter((t: any) => t.status === "completed").length,
      });
    } catch (e) {
      console.error("Failed to load red team tests:", e);
      setTests([]);
    } finally {
      setLoading(false);
    }
  };

  const deleteTest = async (id: string) => {
    try {
      await redteamApi.deleteTest(id);
      message.success("Deleted");
      loadTests();
    } catch (e) {
      message.error("Delete failed");
    }
  };

  const rerunTest = async (id: string) => {
    try {
      await redteamApi.runTest(id);
      message.success("Test restarted");
      loadTests();
    } catch (e) {
      message.error("Failed to start test");
    }
  };

  const columns = [
    { title: "Name", dataIndex: "name", key: "name", render: (name: string, record: any) => <Button type="link" onClick={() => navigate(`/redteam/report/${record.id}`)}>{name}</Button> },
    { title: "Agent", dataIndex: "agent_id", key: "agent_id", render: (id: string) => <code>{id}</code> },
    { title: "Model", dataIndex: "model", key: "model", render: (m: string) => <Tag color="blue">{m}</Tag> },
    { title: "Category", dataIndex: "category", key: "category", render: (c: string) => <Tag color="orange">{c}</Tag> },
    { title: "Status", dataIndex: "status", key: "status", render: (s: string) => <Badge status={s === "completed" ? "success" : s === "running" ? "processing" : "default"} text={s} /> },
    { title: "Actions", key: "action", render: (_: unknown, record: any) => (
      <Space>
        <Tooltip title="Report"><Button size="small" icon={<FileTextOutlined />} onClick={() => navigate(`/redteam/report/${record.id}`)}>Report</Button></Tooltip>
        {record.status === "completed" && <Tooltip title="Rerun"><Button size="small" icon={<PlayCircleOutlined />} onClick={() => rerunTest(record.id)}>Rerun</Button></Tooltip>}
        <Popconfirm title="Delete this test?" onConfirm={() => deleteTest(record.id)}><Button size="small" danger icon={<DeleteOutlined />}>Delete</Button></Popconfirm>
      </Space>
    ) },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 24 }}><SecurityScanOutlined style={{ marginRight: 8 }} />Red Team Testing</h2>
      <Alert message="Red team testing simulates attacker perspectives to proactively discover security vulnerabilities in Agent systems" type="info" showIcon style={{ marginBottom: 24 }} />
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={8}><Card><Statistic title="Total Tests" value={stats.total} /></Card></Col>
        <Col span={8}><Card><Statistic title="Running" value={stats.running} valueStyle={{ color: "#1677ff" }} /></Card></Col>
        <Col span={8}><Card><Statistic title="Completed" value={stats.completed} valueStyle={{ color: "#52c41a" }} /></Card></Col>
      </Row>
      <Card title="Test List" extra={<Space><Button icon={<ReloadOutlined />} onClick={loadTests}>Refresh</Button><Button type="primary" icon={<PlusOutlined />} onClick={() => navigate("/redteam/create")}>Create Test</Button></Space>}>
        <Table columns={columns} dataSource={tests} rowKey="id" loading={loading} />
      </Card>
    </div>
  );
}
