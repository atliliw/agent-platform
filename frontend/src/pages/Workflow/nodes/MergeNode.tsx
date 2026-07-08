import { Handle, Position, type NodeProps } from '@xyflow/react';

export function MergeNode({ data, selected }: NodeProps) {
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
