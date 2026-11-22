import { useTranslations } from 'next-intl';

/**
 * A ranking position's move since yesterday's ranking: ↑n in the up colour,
 * ↓n in the down colour, a small "new" mark for a position the repo did not
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
        className={`rounded-[3px] bg-accent/10 px-1 py-px font-mono text-[9px] font-bold uppercase leading-none text-accent ${className}`}
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
      className={`font-mono text-[10px] font-bold tabular-nums leading-none ${up ? 'text-up' : 'text-down'} ${className}`}
    >
      {up ? '↑' : '↓'}
      {n}
    </span>
  );
}
