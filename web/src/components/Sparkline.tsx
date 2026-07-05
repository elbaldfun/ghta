// Pure SVG line chart for a metric's history. Theme-aware via currentColor.

export interface Point {
  t: number; // unix ms
  v: number;
}

export function Sparkline({ points, height = 160 }: { points: Point[]; height?: number }) {
  if (points.length < 2) return null;
  const width = 640;
  const pad = 8;

  const xs = points.map((p) => p.t);
  const vs = points.map((p) => p.v);
  const minX = Math.min(...xs);
  const maxX = Math.max(...xs);
  const minV = Math.min(...vs);
  const maxV = Math.max(...vs);
  const spanX = maxX - minX || 1;
  const spanV = maxV - minV || 1;

  const coord = (p: Point) => {
    const x = pad + ((p.t - minX) / spanX) * (width - 2 * pad);
    const y = height - pad - ((p.v - minV) / spanV) * (height - 2 * pad);
    return `${x.toFixed(1)},${y.toFixed(1)}`;
  };

  const path = points.map(coord).join(' ');

  return (
    <svg
      viewBox={`0 0 ${width} ${height}`}
      className="w-full text-accent"
      role="img"
      aria-label="Metric history line chart"
      preserveAspectRatio="none"
    >
      <polyline
        points={path}
        fill="none"
        stroke="currentColor"
        strokeWidth={2}
        strokeLinejoin="round"
        strokeLinecap="round"
      />
    </svg>
  );
}
