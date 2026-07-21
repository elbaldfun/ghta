import { useLocale } from 'next-intl';
import type { StarPoint } from '@/lib/data';

const TICKS = 4;
// Must match pad/W below: the label row is inset by the same fraction as the
// plot area so each label sits under the point it describes.
const INSET = '1.85%';

/**
 * Star-growth area chart from the 2a detail view (accent fill 0.14 + 2.5 stroke),
 * with a date axis underneath.
 *
 * The plot stretches to fill its container (preserveAspectRatio="none"), which
 * would squash any <text> inside the SVG — so the tick labels are HTML in a row
 * below it, and the stroke width is held constant with vector-effect.
 */
export function GrowthChart({ points, className }: { points: StarPoint[]; className?: string }) {
  const locale = useLocale();
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

  // Ticks are evenly spaced in time, so they line up with `justify-between`.
  const spanDays = spanT / 86400000;
  const fmt = new Intl.DateTimeFormat(locale,
    spanDays > 400
      ? { year: 'numeric', month: 'short' }
      : spanDays > 60
        ? { month: 'short', year: '2-digit' }
        : { month: 'short', day: 'numeric' },
  );
  const ticks = Array.from({ length: TICKS }, (_, i) => {
    const t = minT + (spanT * i) / (TICKS - 1);
    return { key: t, label: fmt.format(new Date(t)), x: pad + ((W - pad * 2) * i) / (TICKS - 1) };
  });

  return (
    <figure className={`flex min-w-0 flex-col rounded-card border border-border bg-surface ${className ?? ''}`}>
      <svg
        viewBox={`0 0 ${W} ${H}`}
        preserveAspectRatio="none"
        className="min-h-0 w-full flex-1 text-accent"
        role="img"
        aria-label={`Star growth from ${ticks[0].label} to ${ticks[ticks.length - 1].label}`}
      >
        {/* Gridlines first so the curve draws over them. */}
        {ticks.slice(1, -1).map((t) => (
          <line
            key={t.key}
            x1={t.x}
            x2={t.x}
            y1={pad}
            y2={H - pad}
            className="stroke-border"
            strokeWidth={1}
            vectorEffect="non-scaling-stroke"
          />
        ))}
        <path d={area} fill="currentColor" opacity={0.14} />
        <polyline
          points={line}
          fill="none"
          stroke="currentColor"
          strokeWidth={2.5}
          vectorEffect="non-scaling-stroke"
        />
      </svg>
      <figcaption
        className="flex shrink-0 justify-between pb-1.5 pt-1 text-[10px] tabular-nums text-muted"
        style={{ paddingInline: INSET }}
      >
        {ticks.map((t) => (
          <span key={t.key}>{t.label}</span>
        ))}
      </figcaption>
    </figure>
  );
}
