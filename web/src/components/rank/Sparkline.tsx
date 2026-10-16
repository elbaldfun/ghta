import type { TrendPoint } from '@/lib/data';

/**
 * A row's recent star curve: one line plus a faint area, scaled to the row's
 * own range so a steady climb and a one-day spike look different at a glance.
 * Draws nothing with fewer than two points.
 */
export function Sparkline({
  points,
  label,
  width = 88,
  height = 26,
  className = '',
}: {
  points: TrendPoint[] | undefined;
  /** Spoken description, e.g. "+4.5k stars in 7 days". */
  label: string;
  width?: number;
  height?: number;
  className?: string;
}) {
  if (!points || points.length < 2) return <span className={className} style={{ width, height }} aria-hidden="true" />;

  const pad = 2;
  const vs = points.map((p) => p.v);
  const lo = Math.min(...vs);
  const hi = Math.max(...vs);
  const span = hi - lo || 1;
  const step = (width - pad * 2) / (points.length - 1);
  const xy = points.map((p, i) => {
    const x = pad + i * step;
    // A flat series sits mid-height rather than on the floor.
    const y = hi === lo ? height / 2 : height - pad - ((p.v - lo) / span) * (height - pad * 2);
    return `${x.toFixed(1)} ${y.toFixed(1)}`;
  });
  const line = `M${xy.join(' L')}`;
  const area = `${line} L${(width - pad).toFixed(1)} ${height - pad} L${pad} ${height - pad}Z`;
  const rising = vs[vs.length - 1] > vs[0];

  return (
    <svg
      viewBox={`0 0 ${width} ${height}`}
      width={width}
      height={height}
      role="img"
      aria-label={label}
      className={`shrink-0 ${rising ? 'text-up' : 'text-muted'} ${className}`}
    >
      <path d={area} fill="currentColor" opacity={0.12} />
      <path d={line} fill="none" stroke="currentColor" strokeWidth={1.6} strokeLinejoin="round" strokeLinecap="round" />
    </svg>
  );
}
