import { Spin } from 'antd';

interface LoadingProps {
  tip?: string;
  fullScreen?: boolean;
}

export default function Loading({ tip = '加载中...', fullScreen = false }: LoadingProps) {
  if (fullScreen) {
    return (
      <div style={{
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: 'rgba(255, 255, 255, 0.8)',
        zIndex: 1000,
      }}>
        <Spin size="large" tip={tip} />
      </div>
    );
  }

  return (
    <div style={{ textAlign: 'center', padding: '40px 0' }}>
      <Spin tip={tip} />
    </div>
  );
}