import { useState, useEffect, useCallback } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { Card, Table, Tag, Button, Space, message, Badge, Popconfirm } from "antd";
import { ArrowLeftOutlined, CheckCircleOutlined, SwapOutlined } from "@ant-design/icons";
import { promptApi } from "../../api/prompt";

export default function VersionHistory() {
  const { key } = useParams<{ key: string }>();
  const navigate = useNavigate();
  const [versions, setVersions] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);

  const loadVersions = useCallback(async () => {
    if (!key) return;
    setLoading(true);
    try {
      const data = await promptApi.listVersions(key) as any;
      setVersions(data?.versions || []);
    } catch {
      setVersions([]);
    } finally { setLoading(false); }
  }, [key]);

  useEffect(() => { loadVersions(); }, [loadVersions]);

  const handleActivate = async (version_id: string) => {
    try {
      await promptApi.activateVersion({ version_id: version_id });
      message.success("Version activated");
      loadVersions();
    } catch { message.error("Activation failed"); }
  };

  const columns = [
    { title: "Version", dataIndex: "version", key: "version", render: (v: string) => <Tag color="blue">{v}</Tag> },
    { title: "Status", dataIndex: "is_active", key: "status", render: (a: boolean) => a ? <Badge status="success" text="Active" /> : <Badge status="default" text="Inactive" /> },
    { title: "Created By", dataIndex: "created_by", key: "created_by" },
    { title: "Created At", dataIndex: "created_at", key: "created_at", render: (t: number) => t ? new Date(t * 1000).toLocaleString() : "-" },
    { title: "Actions", key: "actions", render: (_: unknown, record: any) => (
      <Space>
        {!record.is_active && <Popconfirm title="Activate this version?" onConfirm={() => handleActivate(record.id)}><Button size="small" type="primary" icon={<CheckCircleOutlined />}>Activate</Button></Popconfirm>}
        <Button size="small" icon={<SwapOutlined />} onClick={() => navigate(`/prompt/compare/${key}`)}>Compare</Button>
      </Space>
    ) },
  ];

  return (
    <div>
      <Button icon={<ArrowLeftOutlined />} onClick={() => navigate("/prompt")} style={{ marginBottom: 16 }}>Back to Prompts</Button>
      <h2>Version History: {key}</h2>
      <Card>
        <Table columns={columns} dataSource={versions} rowKey="id" loading={loading} />
      </Card>
    </div>
  );
}
