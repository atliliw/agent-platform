import { Handle, Position, type NodeProps } from '@xyflow/react';

export function ParallelNode({ data, selected }: NodeProps) {
  return (
    <div
      style={{
        padding: '12px 20px',
        borderRadius: 8,
        border: `2px solid ${selected ? '#52c41a' : '#95de64'}`,
        background: '#f6ffed',
        minWidth: 160,
        textAlign: 'center',
        fontSize: 13,
        boxShadow: selected ? '0 0 0 2px rgba(82,196,26,0.2)' : 'none',
      }}
    >
      <div style={{ fontWeight: 600, marginBottom: 4, color: '#52c41a' }}>
        Parallel
      </div>
      <div style={{ color: '#333' }}>{(data.label as string) || 'Parallel Node'}</div>
      <Handle type="target" position={Position.Top} />
      <Handle type="source" position={Position.Bottom} id="branch-1" style={{ left: '25%' }} />
      <Handle type="source" position={Position.Bottom} id="branch-2" style={{ left: '50%' }} />
      <Handle type="source" position={Position.Bottom} id="branch-3" style={{ left: '75%' }} />
    </div>
  );
}
