import { useTranslations } from 'next-intl';
import { Link } from '@/i18n/navigation';
import type { EcoItem } from '@/lib/data';
import { formatCompact } from '@/lib/rank-data';

function heatColor(pct: number): string {
  if (pct >= 4) return 'rgb(var(--heat4))';
  if (pct >= 2) return 'rgb(var(--heat3))';
  if (pct >= 0.8) return 'rgb(var(--heat2))';
  if (pct >= 0.2) return 'rgb(var(--heat1))';
  return 'rgb(var(--heat0))';
}

/**
 * One dense leaderboard row: rank, identity, and velocity as the visually
 * dominant number — the signal every competing directory lacks — with total
 * stars secondary. (A per-repo trend sparkline goes here once the snapshot
 * history is deep enough to be a real 30-day series rather than a guess.)
 */
export function RankRow({ item, rank }: { item: EcoItem; rank: number }) {
  const t = useTranslations('rank');
  const [owner = '', name = ''] = item.externalId.split('/');
  const pct = item.stars > 0 ? (item.growth / item.stars) * 100 : 0;
  const color = heatColor(pct);

  const badges: string[] = [];
  if (item.isSkill) badges.push(t('ecoSkill'));
  if (item.isMcp) badges.push('MCP');
  if (item.isAgent) badges.push(t('ecoAgent'));

  return (
    <Link
      href={`/repo/${owner}/${name}`}
      className="grid grid-cols-[40px_1fr_100px_84px] items-center gap-3.5 border-b border-border px-4 py-2.5 transition-colors last:border-b-0 hover:bg-surface2"
    >
      <span
        className="font-mono text-[15px] font-bold tabular-nums tracking-tight"
        style={{ color: rank <= 3 ? color : undefined }}
      >
        {rank}
      </span>

      <span className="min-w-0">
        <span className="block truncate text-[13.5px] font-semibold">
          <span className="text-muted">{owner}/</span>
          {name}
        </span>
        <span className="mt-0.5 block truncate text-[11.5px] text-muted">{item.description}</span>
        <span className="mt-1 flex flex-wrap gap-1">
          {badges.map((b) => (
            <span
              key={b}
              className="rounded-[3px] border border-accent bg-surface2 px-1.5 py-px font-mono text-[9.5px] text-accent"
            >
              {b}
            </span>
          ))}
          {item.language && (
            <span className="rounded-[3px] border border-border bg-surface2 px-1.5 py-px font-mono text-[9.5px] text-muted">
              {item.language}
            </span>
          )}
        </span>
      </span>

      <span className="text-right">
        <span
          className="block font-mono text-[16px] font-bold leading-none tabular-nums tracking-tight"
          style={{ color }}
        >
          +{formatCompact(item.growth)}
        </span>
        <span className="mt-0.5 block font-mono text-[10px] tabular-nums" style={{ color }}>
          {pct.toFixed(pct >= 1 ? 1 : 2)}%/{t('mapPerDay')}
        </span>
      </span>

      <span className="text-right font-mono text-[13px] tabular-nums text-fg">
        {formatCompact(item.stars)}
        <span className="text-[9.5px] text-muted"> ★</span>
      </span>
    </Link>
  );
}
