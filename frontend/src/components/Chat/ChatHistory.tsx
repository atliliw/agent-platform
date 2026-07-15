import { List, Button, Popconfirm } from 'antd';
import { DeleteOutlined, MessageOutlined } from '@ant-design/icons';
import type { Session } from '../../types';
import dayjs from 'dayjs';

interface ChatHistoryProps {
  sessions: Session[];
  currentSessionId: string | null;
  onSelect: (id: string) => void;
  onDelete: (id: string) => void;
  onNew: () => void;
}

export default function ChatHistory({
  sessions,
  currentSessionId,
  onSelect,
  onDelete,
  onNew,
}: ChatHistoryProps) {
  return (
    <div className="chat-history">
      <Button
        type="primary"
        icon={<MessageOutlined />}
        onClick={onNew}
        block
        style={{ marginBottom: 16 }}
      >
        新建对话
      </Button>

      <List
        dataSource={sessions}
        renderItem={(session) => (
          <List.Item
            className={`chat-history-item ${currentSessionId === session.id ? 'active' : ''}`}
            onClick={() => onSelect(session.id)}
            actions={[
              <Popconfirm
                key="delete"
                title="确定删除此会话？"
                onConfirm={(e) => {
                  e?.stopPropagation();
                  onDelete(session.id);
                }}
                onCancel={(e) => e?.stopPropagation()}
              >
                <Button
                  type="text"
                  danger
                  size="small"
                  icon={<DeleteOutlined />}
                  onClick={(e) => e.stopPropagation()}
                />
              </Popconfirm>,
            ]}
          >
            <List.Item.Meta
              title={session.title || '新对话'}
              description={dayjs.unix(session.updated_at).format('MM-DD HH:mm')}
            />
          </List.Item>
        )}
      />
    </div>
  );
}