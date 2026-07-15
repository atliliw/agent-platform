import { Empty, Button } from 'antd';

interface EmptyStateProps {
  description?: string;
  action?: {
    text: string;
    onClick: () => void;
  };
}

export default function EmptyState({ description = '暂无数据', action }: EmptyStateProps) {
  return (
    <div style={{ textAlign: 'center', padding: '40px 0' }}>
      <Empty description={description}>
        {action && (
          <Button type="primary" onClick={action.onClick}>
            {action.text}
          </Button>
        )}
      </Empty>
    </div>
  );
}