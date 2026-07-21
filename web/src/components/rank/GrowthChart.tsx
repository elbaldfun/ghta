import { useLocale } from 'next-intl';
import type { StarPoint } from '@/lib/data';
import { formatCompact } from '@/lib/rank-data';

const X_TICKS = 7;
const Y_TICKS = 4;
const W = 540;
const H = 170;
const PAD = 10;

/** Round a raw step up to the nearest 1/2/5×10ⁿ so axis labels land on read-friendly numbers. */
function niceStep(range: number, count: number): number {
  const raw = range / count;
  const mag = 10 ** Math.floor(Math.log10(raw || 1));
  const norm = raw / mag;
  return (norm <= 1 ? 1 : norm <= 2 ? 2 : norm <= 5 ? 5 : 10) * mag;
}

/**
 * Star-growth area chart from the 2a detail view (accent fill 0.14 + 2.5 stroke),
 * with date and star-count axes.
 *
 * The plot stretches to fill its container (preserveAspectRatio="none"), which
 * would squash any <text> inside the SVG — so both axes are HTML: star counts in
 * a fixed-width gutter on the left, dates in a row below, each aligned to the
 * plot's internal padding. Stroke widths are held constant with vector-effect.
 */
export function GrowthChart({ points, className }: { points: StarPoint[]; className?: string }) {
  const locale = useLocale();
  if (points.length < 2) return null;

  const minT = points[0].t;
  const maxT = points[points.length - 1].t;
  const vs = points.map((p) => p.v);
  // Min–max scaling keeps recent-window curves readable (a full history starts at 0 anyway).
  const minV = Math.min(...vs);
  const maxV = Math.max(...vs);
  const spanT = maxT - minT || 1;
  const spanV = maxV - minV || 1;

  const toX = (t: number) => PAD + ((t - minT) / spanT) * (W - PAD * 2);
  const toY = (v: number) => H - PAD - ((v - minV) / spanV) * (H - PAD * 2);

  const coords = points.map((p) => `${toX(p.t).toFixed(1)},${toY(p.v).toFixed(1)}`);
  const line = coords.join(' ');
  const area = `M${coords[0]} L${coords.join(' L')} L${W - PAD},${H - PAD} L${PAD},${H - PAD} Z`;

  // X ticks are evenly spaced in time, so `justify-between` lines them up.
  const spanDays = spanT / 86400000;
  const fmtDate = new Intl.DateTimeFormat(locale,
    spanDays > 400
      ? { year: 'numeric', month: 'short' }
      : spanDays > 60
        ? { month: 'short', year: '2-digit' }
        : { month: 'short', day: 'numeric' },
  );
  const xTicks = Array.from({ length: X_TICKS }, (_, i) => {
    const t = minT + (spanT * i) / (X_TICKS - 1);
    return { key: t, label: fmtDate.format(new Date(t)), x: toX(t) };
  });

  // Y ticks snap to round star counts inside the visible range.
  const step = niceStep(spanV, Y_TICKS);
  const yTicks: { v: number; y: number }[] = [];
  for (let v = Math.ceil(minV / step) * step; v <= maxV; v += step) {
    yTicks.push({ v, y: toY(v) });
  }

  return (
    <figure className={`flex min-w-0 flex-col rounded-card border border-border bg-surface ${className ?? ''}`}>
      <div className="flex min-h-0 flex-1">
        {/* Star-count gutter: labels sit at the same fraction of height as their gridline. */}
        <div className="relative w-11 shrink-0">
          {yTicks.map((t) => (
            <span
              key={t.v}
              className="absolute right-1.5 -translate-y-1/2 text-[10px] tabular-nums text-muted"
              style={{ top: `${(t.y / H) * 100}%` }}
            >
              {formatCompact(t.v)}
            </span>
          ))}
        </div>

        <svg
          viewBox={`0 0 ${W} ${H}`}
          preserveAspectRatio="none"
          className="min-h-0 min-w-0 flex-1 text-accent"
          role="img"
          aria-label={`Star growth from ${xTicks[0].label} to ${xTicks[xTicks.length - 1].label}, ${formatCompact(minV)} to ${formatCompact(maxV)} stars`}
        >
          {/* Gridlines first so the curve draws over them. */}
          {yTicks.map((t) => (
            <line
              key={`y${t.v}`}
              x1={PAD}
              x2={W - PAD}
              y1={t.y}
              y2={t.y}
              className="stroke-border"
              strokeWidth={1}
              vectorEffect="non-scaling-stroke"
            />
          ))}
          {xTicks.slice(1, -1).map((t) => (
            <line
              key={`x${t.key}`}
              x1={t.x}
              x2={t.x}
              y1={PAD}
              y2={H - PAD}
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
      </div>

      <figcaption className="flex shrink-0 pb-1.5 pt-1">
        <div className="w-11 shrink-0" aria-hidden />
        <div
          className="flex flex-1 justify-between text-[10px] tabular-nums text-muted"
          // Matches the plot's internal PAD so the first and last labels sit under their points.
          style={{ paddingInline: `${(PAD / W) * 100}%` }}
        >
          {xTicks.map((t, i) => (
            // Narrow screens can't fit every date; drop the in-between ones there.
            <span key={t.key} className={i % 2 === 1 ? 'hidden sm:inline' : undefined}>
              {t.label}
            </span>
          ))}
        </div>
      </figcaption>
    </figure>
  );
}
