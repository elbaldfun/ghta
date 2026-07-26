import { useTranslations } from 'next-intl';
import { Link } from '@/i18n/navigation';
import type { EcoItem, TrendPoint } from '@/lib/data';
import { formatCompact } from '@/lib/rank-data';

function heatColor(pct: number): string {
  if (pct >= 4) return 'rgb(var(--heat4))';
  if (pct >= 2) return 'rgb(var(--heat3))';
  if (pct >= 0.8) return 'rgb(var(--heat2))';
  if (pct >= 0.2) return 'rgb(var(--heat1))';
  return 'rgb(var(--heat0))';
}

function sparkPath(data: number[], w: number, h: number, pad = 2) {
  const max = Math.max(...data);
  const min = Math.min(...data);
  const rng = max - min || 1;
  return data
    .map((d, i) => {
      const x = pad + (i / (data.length - 1)) * (w - pad * 2);
      const y = h - pad - ((d - min) / rng) * (h - pad * 2);
      return `${i ? 'L' : 'M'}${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(' ');
}

/**
 * One dense leaderboard row: rank, identity, and velocity as the visually
 * dominant number — the signal every competing directory lacks — with total
 * stars secondary. The trend column shows the repo's real snapshot series when
 * there are at least three points; too few and it's left blank rather than
 * faked into a line.
 */
export function RankRow({ item, rank, series }: { item: EcoItem; rank: number; series?: TrendPoint[] }) {
  const t = useTranslations('rank');
  const [owner = '', name = ''] = item.externalId.split('/');
  const pct = item.stars > 0 ? (item.growth / item.stars) * 100 : 0;
  const color = heatColor(pct);

  const badges: string[] = [];
  if (item.isSkill) badges.push(t('ecoSkill'));
  if (item.isMcp) badges.push('MCP');
  if (item.isAgent) badges.push(t('ecoAgent'));

  const values = (series ?? []).map((p) => p.v);
  const hasTrend = values.length >= 3;
  const w = 92;
  const h = 30;
  const gid = `sp-${item.externalId.replace(/[^a-zA-Z0-9]/g, '-')}`;

  return (
    <Link
      href={`/repo/${owner}/${name}`}
      className="grid grid-cols-[36px_1fr_86px] items-center gap-2.5 border-b border-border px-3 py-2.5 transition-colors last:border-b-0 hover:bg-surface2 sm:gap-3.5 sm:px-4 md:grid-cols-[40px_1fr_92px_100px_84px]"
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

      <span className="hidden md:block" title={hasTrend ? `${values.length}${t('trendDaysSuffix')}` : ''}>
        {hasTrend && (
          <svg viewBox={`0 0 ${w} ${h}`} width={w} height={h} aria-hidden="true" className="block">
            <defs>
              <linearGradient id={gid} x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor={color} stopOpacity="0.22" />
                <stop offset="100%" stopColor={color} stopOpacity="0" />
              </linearGradient>
            </defs>
            <path d={`${sparkPath(values, w, h)} L${w - 2},${h} L2,${h} Z`} fill={`url(#${gid})`} />
            <path
              d={sparkPath(values, w, h)}
              fill="none"
              stroke={color}
              strokeWidth="1.4"
              strokeLinejoin="round"
              strokeLinecap="round"
            />
          </svg>
        )}
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

      <span className="hidden text-right font-mono text-[13px] tabular-nums text-fg md:block">
        {formatCompact(item.stars)}
        <span className="text-[9.5px] text-muted"> ★</span>
      </span>
    </Link>
  );
}
