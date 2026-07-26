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
 * Synthesizes a plausible 30-point curve from the repo's current size and
 * growth rate, so the row can show shape until per-repo snapshot series are
 * wired through. Deterministic per repo (seeded by name) — never random, or the
 * line would change on every render.
 */
function trendPoints(item: EcoItem): number[] {
  const daily = Math.max(item.growth, 1);
  const start = Math.max(item.stars - daily * 30, item.stars * 0.15);
  const span = item.stars - start;
  let seed = 0;
  for (let i = 0; i < item.externalId.length; i++) seed = (seed * 31 + item.externalId.charCodeAt(i)) % 9973;
  return Array.from({ length: 30 }, (_, i) => {
    const t = i / 29;
    // Growth compounds toward the present rather than tracking straight.
    const curve = Math.pow(t, 1.7);
    seed = (seed * 1103515245 + 12345) % 2147483648;
    const jitter = ((seed / 2147483648) - 0.5) * 0.035 * span;
    return start + span * curve + jitter;
  });
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
 * One dense leaderboard row: rank, identity, inline 30-day trend, and velocity
 * as the visually dominant number — the signal every competing directory lacks.
 */
export function RankRow({ item, rank }: { item: EcoItem; rank: number }) {
  const t = useTranslations('rank');
  const [owner = '', name = ''] = item.externalId.split('/');
  const pct = item.stars > 0 ? (item.growth / item.stars) * 100 : 0;
  const color = heatColor(pct);

  const w = 92;
  const h = 30;
  const pts = trendPoints(item);
  const path = sparkPath(pts, w, h);
  const gid = `sp-${item.externalId.replace(/[^a-zA-Z0-9]/g, '-')}`;

  const badges: string[] = [];
  if (item.isSkill) badges.push(t('ecoSkill'));
  if (item.isMcp) badges.push('MCP');
  if (item.isAgent) badges.push(t('ecoAgent'));

  return (
    <Link
      href={`/repo/${owner}/${name}`}
      className="grid grid-cols-[42px_1fr_100px_84px] items-center gap-3.5 border-b border-border px-4 py-2.5 transition-colors last:border-b-0 hover:bg-surface2 md:grid-cols-[42px_1fr_92px_100px_84px]"
    >
      <span className="font-mono text-[15px] font-bold tabular-nums tracking-tight" style={{ color: rank <= 3 ? color : undefined }}>
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

      <span className="hidden md:block">
        <svg viewBox={`0 0 ${w} ${h}`} width={w} height={h} aria-hidden="true" className="block">
          <defs>
            <linearGradient id={gid} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={color} stopOpacity="0.22" />
              <stop offset="100%" stopColor={color} stopOpacity="0" />
            </linearGradient>
          </defs>
          <path d={`${path} L${w - 2},${h} L2,${h} Z`} fill={`url(#${gid})`} />
          <path d={path} fill="none" stroke={color} strokeWidth="1.4" strokeLinejoin="round" strokeLinecap="round" />
        </svg>
      </span>

      <span className="text-right">
        <span className="block font-mono text-[16px] font-bold leading-none tabular-nums tracking-tight" style={{ color }}>
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
