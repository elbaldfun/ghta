import type { StarPoint } from '@/lib/github';

/** 540×170 star-growth area chart from the 2a detail view (accent fill 0.14 + 2.5 stroke). */
export function GrowthChart({ points }: { points: StarPoint[] }) {
  const W = 540;
  const H = 170;
  const pad = 10;
  if (points.length < 2) return null;

  const minT = points[0].t;
  const maxT = points[points.length - 1].t;
  const vs = points.map((p) => p.v);
  // Min–max scaling keeps recent-window curves readable (a full history starts at 0 anyway).
  const minV = Math.min(...vs);
  const maxV = Math.max(...vs);
  const spanT = maxT - minT || 1;
  const spanV = maxV - minV || 1;

  const coords = points.map((p) => {
    const x = pad + ((p.t - minT) / spanT) * (W - pad * 2);
    const y = H - pad - ((p.v - minV) / spanV) * (H - pad * 2);
    return `${x.toFixed(1)},${y.toFixed(1)}`;
  });
  const line = coords.join(' ');
  const area = `M${coords[0]} L${coords.join(' L')} L${W - pad},${H - pad} L${pad},${H - pad} Z`;

  return (
    <svg
      width={W}
      height={H}
      viewBox={`0 0 ${W} ${H}`}
      className="max-w-full rounded-card border border-border bg-surface text-accent"
      role="img"
      aria-label="Star growth chart"
    >
      <path d={area} fill="currentColor" opacity={0.14} />
      <polyline points={line} fill="none" stroke="currentColor" strokeWidth={2.5} />
    </svg>
  );
}
