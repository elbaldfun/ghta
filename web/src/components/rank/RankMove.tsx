import { useTranslations } from 'next-intl';

/**
 * A ranking position's move since yesterday's ranking: ↑n in the signal colour,
 * ↓n in muted ink (a drop is not an alarm, as with Delta), a small "new" mark for a position the repo did not
 * hold yesterday, and nothing when it held still or there is no baseline.
 * `scope` names the ranking for the tooltip and screen readers ("Overall",
 * "Go", a domain's label).
 */
export function RankMove({
  change,
  isNew,
  scope,
  className = '',
}: {
  change?: number;
  isNew?: boolean;
  scope: string;
  className?: string;
}) {
  const t = useTranslations('rank');
  if (isNew) {
    const label = t('rankNewHint', { scope });
    return (
      <span
        title={label}
        aria-label={label}
        className={`rounded-[3px] bg-signal-soft px-1 py-px font-code text-[9px] font-bold uppercase leading-none text-signal-ink ${className}`}
      >
        {t('rankNew')}
      </span>
    );
  }
  if (!change) return null;
  const up = change > 0;
  const n = Math.abs(change).toLocaleString();
  const label = t(up ? 'rankUp' : 'rankDown', { scope, n });
  return (
    <span
      title={label}
      aria-label={label}
      className={`font-code text-[10px] font-bold tabular-nums leading-none ${up ? 'text-signal' : 'text-ink-3'} ${className}`}
    >
      {up ? '↑' : '↓'}
      {n}
    </span>
  );
}
