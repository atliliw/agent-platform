import { Handle, Position, type NodeProps } from '@xyflow/react';

const STATUS_COLORS: Record<string, string> = {
  running: '#1677ff',
  completed: '#52c41a',
  failed: '#ff4d4f',
  timed_out: '#fa8c16',
};

export function MergeNode({ data, selected }: NodeProps) {
  const status = data.status as string | undefined;
  return (
    <div
      style={{
        padding: '12px 20px',
        borderRadius: 8,
        border: `2px solid ${selected ? '#722ed1' : '#b37feb'}`,
        background: '#f9f0ff',
        minWidth: 160,
        textAlign: 'center',
        fontSize: 13,
        boxShadow: selected ? '0 0 0 2px rgba(114,46,209,0.2)' : 'none',
      }}
    >
      {status && status !== 'idle' && (
        <div style={{
          height: 3,
          borderRadius: '6px 6px 0 0',
          background: STATUS_COLORS[status] ?? '#d9d9d9',
          margin: '-12px -20px 8px',
        }} />
      )}
      <div style={{ fontWeight: 600, marginBottom: 4, color: '#722ed1' }}>
        Merge
      </div>
      <div style={{ color: '#333' }}>{(data.label as string) || 'Merge Node'}</div>
      <Handle type="target" position={Position.Top} id="merge-1" style={{ left: '25%' }} />
      <Handle type="target" position={Position.Top} id="merge-2" style={{ left: '50%' }} />
      <Handle type="target" position={Position.Top} id="merge-3" style={{ left: '75%' }} />
      <Handle type="source" position={Position.Bottom} />
    </div>
  );
}
