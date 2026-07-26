'use client';

import { useState } from 'react';
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

function Spark({ values, color, w, h }: { values: number[]; color: string; w: number; h: number }) {
  const gid = `sp-${Math.random().toString(36).slice(2, 9)}`;
  return (
    <svg viewBox={`0 0 ${w} ${h}`} width={w} height={h} aria-hidden="true" className="block">
      <defs>
        <linearGradient id={gid} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={color} stopOpacity="0.22" />
          <stop offset="100%" stopColor={color} stopOpacity="0" />
        </linearGradient>
      </defs>
      <path d={`${sparkPath(values, w, h)} L${w - 2},${h} L2,${h} Z`} fill={`url(#${gid})`} />
      <path d={sparkPath(values, w, h)} fill="none" stroke={color} strokeWidth="1.4" strokeLinejoin="round" strokeLinecap="round" />
    </svg>
  );
}

export interface RankEntry {
  item: EcoItem;
  rank: number;
  series?: TrendPoint[];
}

/**
 * The dense "moving now" leaderboard. Clicking a row expands its detail — chart,
 * stats, and an "open on GitHub" action — in place, so exploring a dozen repos
 * never costs a page navigation and a scroll-position loss.
 */
export function RankTable({ entries, header }: { entries: RankEntry[]; header: React.ReactNode }) {
  const t = useTranslations('rank');
  const [open, setOpen] = useState<string | null>(null);

  return (
    <div className="overflow-hidden rounded-card border border-border bg-surface">
      {header}
      {entries.map(({ item, rank, series }) => {
        const [owner = '', name = ''] = item.externalId.split('/');
        const pct = item.stars > 0 ? (item.growth / item.stars) * 100 : 0;
        const color = heatColor(pct);
        const values = (series ?? []).map((p) => p.v);
        const hasTrend = values.length >= 3;
        const isOpen = open === item.externalId;

        const badges: string[] = [];
        if (item.isSkill) badges.push(t('ecoSkill'));
        if (item.isMcp) badges.push('MCP');
        if (item.isAgent) badges.push(t('ecoAgent'));
        const weekly = Math.round(item.growth * 7);

        return (
          <div key={item.externalId} className="border-b border-border last:border-b-0">
            <button
              onClick={() => setOpen(isOpen ? null : item.externalId)}
              aria-expanded={isOpen}
              className={`grid w-full grid-cols-[36px_1fr_86px] items-center gap-2.5 px-3 py-2.5 text-left transition-colors hover:bg-surface2 sm:gap-3.5 sm:px-4 md:grid-cols-[40px_1fr_92px_100px_84px] ${
                isOpen ? 'bg-surface2' : ''
              }`}
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
                    <span key={b} className="rounded-[3px] border border-accent bg-surface2 px-1.5 py-px font-mono text-[9.5px] text-accent">
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

              <span className="hidden md:block">{hasTrend && <Spark values={values} color={color} w={92} h={30} />}</span>

              <span className="text-right">
                <span className="block font-mono text-[16px] font-bold leading-none tabular-nums tracking-tight" style={{ color }}>
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
            </button>

            {isOpen && (
              <div className="grid gap-4 bg-surface2 px-3 pb-4 pt-1 sm:px-4 md:grid-cols-[1fr_260px] md:pl-[74px]">
                <div>
                  {item.description && <p className="mb-3 max-w-[62ch] text-[12.5px] text-muted">{item.description}</p>}
                  <div className="flex flex-wrap gap-x-6 gap-y-2">
                    <span>
                      <span className="block font-mono text-[9.5px] uppercase tracking-[0.08em] text-muted">{t('mapColVelocity')}</span>
                      <span className="font-mono text-[15px] font-bold tabular-nums" style={{ color }}>+{formatCompact(item.growth)} ★/{t('mapPerDay')}</span>
                    </span>
                    <span>
                      <span className="block font-mono text-[9.5px] uppercase tracking-[0.08em] text-muted">{t('detailWeekly')}</span>
                      <span className="font-mono text-[15px] font-bold tabular-nums">+{formatCompact(weekly)}</span>
                    </span>
                    <span>
                      <span className="block font-mono text-[9.5px] uppercase tracking-[0.08em] text-muted">{t('mapColStars')}</span>
                      <span className="font-mono text-[15px] font-bold tabular-nums">{formatCompact(item.stars)}</span>
                    </span>
                  </div>
                  <div className="mt-3.5 flex flex-wrap gap-2">
                    <a
                      href={`https://github.com/${item.externalId}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="rounded-md bg-accent px-3 py-1.5 text-[12px] font-bold text-accent-fg"
                    >
                      {t('detailOpenGithub')}
                    </a>
                    <Link
                      href={`/repo/${owner}/${name}`}
                      className="rounded-md border border-border bg-surface px-3 py-1.5 text-[12px] font-bold text-fg"
                    >
                      {t('detailDetailPage')}
                    </Link>
                  </div>
                </div>
                {hasTrend && (
                  <div className="rounded-card border border-border bg-surface p-2.5">
                    <div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.08em] text-muted">
                      {values.length}
                      {t('trendDaysSuffix')} · {t('mapColTrend')}
                    </div>
                    <Spark values={values} color={color} w={236} h={80} />
                  </div>
                )}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
