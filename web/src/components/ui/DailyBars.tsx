/**
 * Daily star increments as bars — the replacement for cumulative sparklines,
 * which draw nearly the same rising line for every repo. History is the
 * neutral `bar` tone; only the most recent day is the signal colour. A short
 * series (a repo tracked for 3 days) is right-aligned so "today" always sits
 * at the right edge; negative days (unstars, data fixes) are a stub in ink-3.
 *
 * Renders an SVG that stretches to its container's width; set the height.
 */
export function DailyBars({
  values,
  days = 31,
  highlightLast = true,
  height = 28,
  label,
  className = '',
}: {
  /** Oldest first; one value per day. */
  values: number[];
  days?: number;
  highlightLast?: boolean;
  /** Pixels, or any CSS length ("100%" to fill a sized parent). */
  height?: number | string;
  /** Spoken description, e.g. "Daily new stars over 31 days, falling". */
  label: string;
  className?: string;
}) {
  const series = values.slice(-days);
  const pad = days - series.length;
  const max = Math.max(1, ...series.map((v) => Math.abs(v)));
  const slot = 10; // viewBox units per day
  const gap = 2;

  return (
    <svg
      role="img"
      aria-label={label}
      viewBox={`0 0 ${days * slot} 100`}
      preserveAspectRatio="none"
      className={`block w-full ${className}`}
      style={{ height }}
    >
      {series.map((v, i) => {
        if (v === 0) return null;
        const h = v < 0 ? 3 : Math.max(3, (v / max) * 100);
        const isLast = i === series.length - 1;
        const tone = v < 0 ? 'fill-ink-3' : isLast && highlightLast ? 'fill-signal-bar' : 'fill-bar';
        return (
          <rect
            key={i}
            x={(pad + i) * slot + gap / 2}
            width={slot - gap}
            y={100 - h}
            height={h}
            className={tone}
          />
        );
      })}
    </svg>
  );
}
