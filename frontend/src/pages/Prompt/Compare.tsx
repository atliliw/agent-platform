import { useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { Card, Select, Button, Space, Spin, Empty } from "antd";
import { ArrowLeftOutlined, SwapOutlined } from "@ant-design/icons";
import { promptApi } from "../../api/prompt";

export default function PromptCompare() {
  const { key } = useParams<{ key: string }>();
  const navigate = useNavigate();
  const [version1, setVersion1] = useState('');
  const [version2, setVersion2] = useState('');
  const [diff, setDiff] = useState<any>(null);
  const [loading, setLoading] = useState(false);
  const [versions, setVersions] = useState<any[]>([]);

  const loadVersions = async () => {
    if (!key) return;
    try {
      const res = await promptApi.listVersions(key) as any;
      setVersions(res?.versions || []);
    } catch { setVersions([]); }
  };

  // Load versions on mount
  useState(() => { loadVersions(); });

  const handleCompare = async () => {
    if (!version1 || !version2) return;
    setLoading(true);
    try {
      const res = await promptApi.compareVersions({ version1_id: version1, version2_id: version2 }) as any;
      setDiff(res);
    } catch {
      setDiff(null);
    } finally { setLoading(false); }
  };

  return (
    <div>
      <Button icon={<ArrowLeftOutlined />} onClick={() => navigate(`/prompt/history/${key}`)} style={{ marginBottom: 16 }}>Back</Button>
      <h2>Compare Versions: {key}</h2>
      <Card>
        <Space style={{ marginBottom: 16 }}>
          <Select style={{ width: 200 }} placeholder="Version 1" value={version1} onChange={setVersion1}>
            {versions.map(v => <Select.Option key={v.id} value={v.id}>{v.version}</Select.Option>)}
          </Select>
          <SwapOutlined />
          <Select style={{ width: 200 }} placeholder="Version 2" value={version2} onChange={setVersion2}>
            {versions.map(v => <Select.Option key={v.id} value={v.id}>{v.version}</Select.Option>)}
          </Select>
          <Button type="primary" onClick={handleCompare} loading={loading} disabled={!version1 || !version2}>Compare</Button>
        </Space>
        {loading ? <Spin /> : diff ? (
          <div>
            {diff.summary && <p><strong>Summary:</strong> {diff.summary}</p>}
            {diff.content_diff?.map((line: any, i: number) => (
              <div key={i} style={{
                backgroundColor: line.type === 'add' ? '#e6ffed' : line.type === 'remove' ? '#ffeef0' : 'transparent',
                padding: '2px 8px',
                fontFamily: 'monospace',
              }}>
                <span style={{ marginRight: 8, color: '#999' }}>{line.type === 'add' ? '+' : line.type === 'remove' ? '-' : ' '}</span>
                {line.content}
              </div>
            ))}
          </div>
        ) : <Empty description="Select two versions to compare" />}
      </Card>
    </div>
  );
}
