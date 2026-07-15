import { useState, useEffect } from 'react';
import {
  Card,
  Table,
  Button,
  Space,
  Modal,
  Form,
  Input,
  message,
  Popconfirm,
  Tag,
  Typography,
  Alert,
  Tooltip,
} from 'antd';
import {
  TrophyOutlined,
  PlusOutlined,
  DeleteOutlined,
  EyeOutlined,
  CopyOutlined,
  EditOutlined,
} from '@ant-design/icons';
import { cookieApi, type Cookie, type StoredCookie } from '../../api/cookie';

const { Text, Paragraph } = Typography;

interface CookieGroup {
  domain: string;
  cookies: StoredCookie[];
}

export default function CookiePage() {
  const [loading, setLoading] = useState(false);
  const [cookieGroups, setCookieGroups] = useState<CookieGroup[]>([]);
  const [modalVisible, setModalVisible] = useState(false);
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [editingDomain, setEditingDomain] = useState<string>('');
  const [form] = Form.useForm();
  const [editForm] = Form.useForm();
  const [detailModalVisible, setDetailModalVisible] = useState(false);
  const [selectedCookies, setSelectedCookies] = useState<StoredCookie[]>([]);

  // 加载所有 Cookie
  const loadCookies = async () => {
    setLoading(true);
    try {
      const res = await cookieApi.getAll({});
      const data = (res as any).data || res;
      const cookies: StoredCookie[] = data.cookies || [];

      // 按域名分组
      const groups: Record<string, StoredCookie[]> = {};
      cookies.forEach((cookie) => {
        const domain = cookie.domain || 'unknown';
        if (!groups[domain]) {
          groups[domain] = [];
        }
        groups[domain].push(cookie);
      });

      const groupArray = Object.entries(groups).map(([domain, cookies]) => ({
        domain,
        cookies,
      }));

      setCookieGroups(groupArray);
    } catch (error) {
      console.error('Load cookies failed:', error);
      message.error('加载 Cookie 失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadCookies();
  }, []);

  // 添加 Cookie
  const handleAdd = async (values: any) => {
    try {
      // 解析 Cookie 文本
      const lines = values.cookieText.split('\n').filter((line: string) => line.trim());
      const cookies: Cookie[] = lines.map((line: string) => {
        const parts = line.split('\t');
        return {
          name: parts[0]?.trim() || '',
          value: parts[1]?.trim() || '',
          domain: values.domain || parts[2]?.trim(),
          path: parts[3]?.trim() || '/',
        };
      }).filter((c: Cookie) => c.name && c.value);

      if (cookies.length === 0) {
        message.error('请输入有效的 Cookie');
        return;
      }

      await cookieApi.save({
        domain: values.domain,
        cookies,
      });

      message.success(`已保存 ${cookies.length} 个 Cookie`);
      setModalVisible(false);
      form.resetFields();
      loadCookies();
    } catch (error) {
      message.error('保存失败');
    }
  };

  // 编辑域名下的 Cookie
  const handleEdit = (domain: string, cookies: StoredCookie[]) => {
    setEditingDomain(domain);
    const cookieText = cookies.map(c => `${c.name}\t${c.value}\t${c.domain || ''}\t${c.path || '/'}`).join('\n');
    editForm.setFieldsValue({
      domain,
      cookieText,
    });
    setEditModalVisible(true);
  };

  // 保存编辑
  const handleEditSave = async (values: any) => {
    try {
      // 先删除旧的
      await cookieApi.delete({ domain: editingDomain });

      // 解析并保存新的
      const lines = values.cookieText.split('\n').filter((line: string) => line.trim());
      const cookies: Cookie[] = lines.map((line: string) => {
        const parts = line.split('\t');
        return {
          name: parts[0]?.trim() || '',
          value: parts[1]?.trim() || '',
          domain: values.domain || parts[2]?.trim(),
          path: parts[3]?.trim() || '/',
        };
      }).filter((c: Cookie) => c.name && c.value);

      if (cookies.length > 0) {
        await cookieApi.save({
          domain: values.domain,
          cookies,
        });
      }

      message.success('更新成功');
      setEditModalVisible(false);
      editForm.resetFields();
      loadCookies();
    } catch (error) {
      message.error('更新失败');
    }
  };

  // 删除域名下的所有 Cookie
  const handleDelete = async (domain: string) => {
    try {
      await cookieApi.delete({ domain });
      message.success('删除成功');
      loadCookies();
    } catch (error) {
      message.error('删除失败');
    }
  };

  // 查看详情
  const handleViewDetail = (cookies: StoredCookie[]) => {
    setSelectedCookies(cookies);
    setDetailModalVisible(true);
  };

  // 复制 Cookie
  const handleCopy = (cookies: StoredCookie[]) => {
    const text = cookies.map(c => `${c.name}=${c.value}`).join('; ');
    navigator.clipboard.writeText(text);
    message.success('已复制到剪贴板');
  };

  // 表格列
  const columns = [
    {
      title: '域名',
      dataIndex: 'domain',
      key: 'domain',
      render: (domain: string) => <Tag color="blue">{domain}</Tag>,
    },
    {
      title: 'Cookie 数量',
      key: 'count',
      render: (_: any, record: CookieGroup) => <Tag color="green">{record.cookies.length} 个</Tag>,
    },
    {
      title: 'Cookie 名称',
      key: 'names',
      render: (_: any, record: CookieGroup) => (
        <Space size={[4, 4]} wrap>
          {record.cookies.slice(0, 5).map((c, i) => (
            <Tag key={i}>{c.name}</Tag>
          ))}
          {record.cookies.length > 5 && (
            <Tag>+{record.cookies.length - 5}</Tag>
          )}
        </Space>
      ),
    },
    {
      title: '操作',
      key: 'actions',
      render: (_: any, record: CookieGroup) => (
        <Space>
          <Tooltip title="查看详情">
            <Button
              type="text"
              icon={<EyeOutlined />}
              onClick={() => handleViewDetail(record.cookies)}
            />
          </Tooltip>
          <Tooltip title="编辑">
            <Button
              type="text"
              icon={<EditOutlined />}
              onClick={() => handleEdit(record.domain, record.cookies)}
            />
          </Tooltip>
          <Tooltip title="复制">
            <Button
              type="text"
              icon={<CopyOutlined />}
              onClick={() => handleCopy(record.cookies)}
            />
          </Tooltip>
          <Popconfirm
            title="确定删除该域名下的所有 Cookie 吗？"
            onConfirm={() => handleDelete(record.domain)}
          >
            <Tooltip title="删除">
              <Button type="text" danger icon={<DeleteOutlined />} />
            </Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Card
        title={
          <Space>
            <TrophyOutlined />
            <span>Cookie 管理</span>
          </Space>
        }
        extra={
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => setModalVisible(true)}
          >
            添加 Cookie
          </Button>
        }
      >
        <Alert
          message="Cookie 用于浏览器自动化"
          description="在此保存的 Cookie 会在 Agent 访问对应网站时自动注入，实现免登录访问。格式：每行一个 Cookie，用 Tab 分隔：名称[TAB]值[TAB]域名[TAB]路径"
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
        />

        <Table
          columns={columns}
          dataSource={cookieGroups}
          rowKey="domain"
          loading={loading}
          pagination={false}
          locale={{
            emptyText: '暂无 Cookie，点击上方按钮添加',
          }}
        />
      </Card>

      {/* 添加 Cookie Modal */}
      <Modal
        title="添加 Cookie"
        open={modalVisible}
        onCancel={() => {
          setModalVisible(false);
          form.resetFields();
        }}
        onOk={() => form.submit()}
        width={600}
      >
        <Form form={form} layout="vertical" onFinish={handleAdd}>
          <Form.Item
            name="domain"
            label="域名"
            rules={[{ required: true, message: '请输入域名，如 .csdn.net' }]}
          >
            <Input placeholder=".csdn.net" />
          </Form.Item>
          <Form.Item
            name="cookieText"
            label="Cookie 内容"
            rules={[{ required: true, message: '请输入 Cookie' }]}
            extra="每行一个 Cookie，格式：名称[TAB]值[TAB]域名[TAB]路径"
          >
            <Input.TextArea
              rows={10}
              placeholder={`UserName\tm0_54140879\t.csdn.net\t/\nUserToken\txxx-token\t.csdn.net\t/`}
              style={{ fontFamily: 'monospace' }}
            />
          </Form.Item>
        </Form>
      </Modal>

      {/* 编辑 Cookie Modal */}
      <Modal
        title="编辑 Cookie"
        open={editModalVisible}
        onCancel={() => {
          setEditModalVisible(false);
          editForm.resetFields();
        }}
        onOk={() => editForm.submit()}
        width={600}
      >
        <Form form={editForm} layout="vertical" onFinish={handleEditSave}>
          <Form.Item
            name="domain"
            label="域名"
            rules={[{ required: true, message: '请输入域名' }]}
          >
            <Input placeholder=".csdn.net" />
          </Form.Item>
          <Form.Item
            name="cookieText"
            label="Cookie 内容"
            rules={[{ required: true, message: '请输入 Cookie' }]}
            extra="每行一个 Cookie，格式：名称[TAB]值[TAB]域名[TAB]路径"
          >
            <Input.TextArea
              rows={10}
              style={{ fontFamily: 'monospace' }}
            />
          </Form.Item>
        </Form>
      </Modal>

      {/* Cookie 详情 Modal */}
      <Modal
        title="Cookie 详情"
        open={detailModalVisible}
        onCancel={() => setDetailModalVisible(false)}
        footer={[
          <Button key="copy" onClick={() => handleCopy(selectedCookies)}>
            复制全部
          </Button>,
          <Button key="close" onClick={() => setDetailModalVisible(false)}>
            关闭
          </Button>,
        ]}
        width={700}
      >
        <Table
          dataSource={selectedCookies}
          rowKey="name"
          pagination={false}
          size="small"
          columns={[
            {
              title: '名称',
              dataIndex: 'name',
              key: 'name',
              width: 150,
            },
            {
              title: '值',
              dataIndex: 'value',
              key: 'value',
              render: (value: string) => (
                <Paragraph
                  copyable
                  ellipsis={{ rows: 2 }}
                  style={{ margin: 0, fontFamily: 'monospace', fontSize: 12 }}
                >
                  {value}
                </Paragraph>
              ),
            },
            {
              title: '路径',
              dataIndex: 'path',
              key: 'path',
              width: 80,
              render: (path: string) => <Text type="secondary">{path || '/'}</Text>,
            },
          ]}
        />
      </Modal>
    </div>
  );
}