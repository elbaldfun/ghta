import { fmtInt, HOT_DAILY } from './format';

/**
 * A star increment. Direction is the glyph, never a red/green pair:
 *   hot gain   → ▲1,429 in the signal colour
 *   plain gain → +195 in ink-2
 *   loss       → ▼1,306 in ink-3 (neutral; losses are rare and not alarming)
 *   unknown    → — in ink-3
 * `hot` defaults to "at least HOT_DAILY"; pass it explicitly for 7-day or
 * headline figures that follow a different rule.
 */
export function Delta({
  value,
  hot,
  label,
  className = '',
}: {
  value: number | null | undefined;
  hot?: boolean;
  /** Spoken description, e.g. "1,429 stars today". */
  label?: string;
  className?: string;
}) {
  const base = `tabular-nums whitespace-nowrap ${className}`;
  if (value == null || Number.isNaN(value)) {
    return (
      <span aria-label={label} className={`${base} text-ink-3`}>
        —
      </span>
    );
  }
  const n = fmtInt(Math.abs(value));
  if (value < 0) {
    return (
      <span aria-label={label} className={`${base} text-ink-3`}>
        ▼{n}
      </span>
    );
  }
  const isHot = hot ?? value >= HOT_DAILY;
  return (
    <span aria-label={label} className={`${base} ${isHot ? 'text-signal' : 'text-ink-2'}`}>
      {isHot ? '▲' : '+'}
      {n}
    </span>
  );
}
