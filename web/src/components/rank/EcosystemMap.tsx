'use client';

import { useMemo, useState } from 'react';
import { useTranslations, useLocale } from 'next-intl';
import { Link } from '@/i18n/navigation';
import type { HeatCell } from '@/lib/data';
import { formatCompact } from '@/lib/rank-data';

type ColorBy = 'heat' | 'growth' | 'scale';

const RAMP = ['var(--heat0)', 'var(--heat1)', 'var(--heat2)', 'var(--heat3)', 'var(--heat4)'];
const RAMP_FG = ['#0b0f0d', '#04140f', '#1a1305', '#fff', '#fff'];

const metricOf = (c: HeatCell, by: ColorBy): number =>
  by === 'heat' ? c.intensity : by === 'growth' ? c.growth : c.repos;

/**
 * Colour is assigned by where a cell falls among its siblings on the chosen
 * metric (quantile), not by a fixed threshold — so the same ramp reads sensibly
 * whether it's mapping stars-per-repo, absolute growth, or repo count. Area
 * always tracks repo count, so the layout stays put and only the meaning of the
 * colour changes.
 */
function ramp(value: number, sorted: number[]): { bg: string; fg: string } {
  const below = sorted.filter((v) => v < value).length;
  const frac = sorted.length > 1 ? below / (sorted.length - 1) : 1;
  const i = frac >= 0.85 ? 4 : frac >= 0.6 ? 3 : frac >= 0.35 ? 2 : frac >= 0.12 ? 1 : 0;
  return { bg: `rgb(${RAMP[i]})`, fg: RAMP_FG[i] };
}

function pack(cells: HeatCell[], rowCounts: number[]): HeatCell[][] {
  const sorted = [...cells].sort((a, b) => b.repos - a.repos);
  const rows: HeatCell[][] = [];
  let i = 0;
  for (const n of rowCounts) {
    if (i >= sorted.length) break;
    rows.push(sorted.slice(i, i + n));
    i += n;
  }
  if (i < sorted.length) rows.push(sorted.slice(i));
  return rows;
}

export function EcosystemMap({ cells }: { cells: HeatCell[] }) {
  const t = useTranslations('rank');
  const locale = useLocale();
  const [open, setOpen] = useState<string | null>(null);
  const [colorBy, setColorBy] = useState<ColorBy>('heat');

  const topSorted = useMemo(
    () => cells.map((c) => metricOf(c, colorBy)).sort((a, b) => a - b),
    [cells, colorBy],
  );

  if (cells.length === 0) return null;

  const label = (c: HeatCell) => (locale === 'zh' ? c.name : c.nameEn || c.name);
  const rows = pack(cells, [4, 4, 4]);
  const total = cells.reduce((s, c) => s + c.repos, 0);
  const active = open ? cells.find((c) => c.path === open) : null;

  const colorLegend =
    colorBy === 'heat' ? t('mapColorHeat') : colorBy === 'growth' ? t('mapColorGrowth') : t('mapColorScale');

  const modes: [ColorBy, string][] = [
    ['heat', t('mapByHeat')],
    ['growth', t('mapByGrowth')],
    ['scale', t('mapByScale')],
  ];

  return (
    <div>
      <div className="mb-3 flex flex-wrap items-center gap-x-4 gap-y-2">
        <span className="text-[10px] font-semibold uppercase tracking-[0.07em] text-muted">
          {t('mapAreaLegend')}
        </span>
        <span className="text-[10px] font-semibold uppercase tracking-[0.07em] text-muted">
          {t('mapColorBy')}：{colorLegend}
        </span>

        <div className="flex items-center gap-3 sm:ml-auto">
          {/* colour-metric switch — area stays scale, only the colour's meaning changes */}
          <div className="flex overflow-hidden rounded-[7px] border border-border-strong">
            {modes.map(([key, txt]) => (
              <button
                key={key}
                onClick={() => setColorBy(key)}
                aria-pressed={colorBy === key}
                className={`px-2.5 py-1 text-[11.5px] font-semibold transition-colors ${
                  colorBy === key ? 'bg-accent/10 text-accent' : 'text-muted hover:text-fg'
                }`}
              >
                {txt}
              </button>
            ))}
          </div>
          <span className="flex items-center gap-1">
            <span className="text-[9.5px] text-muted">{t('mapCold')}</span>
            {RAMP.map((v) => (
              <i key={v} className="block h-[7px] w-4 rounded-[1px]" style={{ background: `rgb(${v})` }} />
            ))}
            <span className="text-[9.5px] text-muted">{t('mapHot')}</span>
          </span>
        </div>
      </div>

      {/* Wide screens: a treemap where area encodes scale. */}
      <div className="hidden h-[340px] flex-col gap-[3px] sm:flex">
        {rows.map((row, ri) => {
          const rowArea = row.reduce((s, c) => s + c.repos, 0);
          const small = ri === rows.length - 1 && rows.length > 2;
          return (
            <div key={ri} className="flex gap-[3px]" style={{ flex: `${rowArea / total} 1 0` }}>
              {row.map((c) => {
                const col = ramp(metricOf(c, colorBy), topSorted);
                const isOpen = open === c.path;
                return (
                  <button
                    key={c.path}
                    onClick={() => setOpen(isOpen ? null : c.path)}
                    aria-expanded={isOpen}
                    className="flex flex-col justify-between overflow-hidden rounded-[5px] px-2.5 py-2 text-left transition-[filter,transform] hover:brightness-[1.07] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-fg"
                    style={{ flex: `${c.repos / rowArea} 1 0`, background: col.bg, color: col.fg }}
                  >
                    <span>
                      <span className={`block font-bold leading-tight ${small ? 'text-[11.5px]' : 'text-[13px]'}`}>
                        {label(c)}
                      </span>
                      {!small && (
                        <span className="mt-0.5 block font-mono text-[10px] tabular-nums opacity-80">
                          {formatCompact(c.repos)} · {c.intensity} ★/{t('mapPerRepo')}
                        </span>
                      )}
                    </span>
                    <span className={`font-mono font-bold tabular-nums tracking-tight ${small ? 'text-[11.5px]' : 'text-[14px]'}`}>
                      +{formatCompact(c.growth)}
                      <span className="text-[9.5px] font-medium opacity-80"> ★/{t('mapPerDay')}</span>
                    </span>
                  </button>
                );
              })}
            </div>
          );
        })}
      </div>

      {/* Narrow screens: a treemap would crush the labels, so collapse to a
          one-per-row bar list — width encodes scale, colour the chosen metric,
          sorted by growth. */}
      <div className="flex flex-col gap-1.5 sm:hidden">
        {[...cells]
          .sort((a, b) => b.growth - a.growth)
          .map((c) => {
            const col = ramp(metricOf(c, colorBy), topSorted);
            const isOpen = open === c.path;
            return (
              <button
                key={c.path}
                onClick={() => setOpen(isOpen ? null : c.path)}
                aria-expanded={isOpen}
                className="rounded-[6px] border border-border bg-surface px-3 py-2 text-left"
              >
                <div className="flex items-baseline justify-between gap-2">
                  <span className="text-[13px] font-bold">{label(c)}</span>
                  <span className="font-mono text-[12px] font-bold tabular-nums" style={{ color: col.bg }}>
                    +{formatCompact(c.growth)}
                    <span className="text-[9px] font-medium text-muted"> ★/{t('mapPerDay')}</span>
                  </span>
                </div>
                <div className="mt-1.5 h-[6px] overflow-hidden rounded-full bg-surface2">
                  <i
                    className="block h-full rounded-full"
                    style={{ width: `${Math.max(4, (c.repos / total) * 100).toFixed(1)}%`, background: col.bg }}
                  />
                </div>
                <div className="mt-1 font-mono text-[10px] tabular-nums text-muted">
                  {formatCompact(c.repos)} · {c.intensity} ★/{t('mapPerRepo')}
                </div>
              </button>
            );
          })}
      </div>

      {active && (active.children?.length ?? 0) > 0 && (
        <div className="mt-3 rounded-card border border-border bg-surface p-3.5">
          <div className="mb-2.5 flex items-baseline gap-2">
            <span className="text-[13px] font-bold">{label(active)}</span>
            <span className="text-[11px] text-muted">{t('mapDrill')}</span>
          </div>
          <div className="grid gap-[3px] sm:grid-cols-2 lg:grid-cols-3">
            {(() => {
              const kidsSorted = active.children!.map((k) => metricOf(k, colorBy)).sort((a, b) => a - b);
              return active.children!.map((k) => {
                const col = ramp(metricOf(k, colorBy), kidsSorted);
                return (
                  <Link
                    key={k.path}
                    href={`/?category=${encodeURIComponent(k.path)}`}
                    className="flex items-center justify-between gap-2 rounded-[5px] px-2.5 py-2 transition-[filter] hover:brightness-[1.07]"
                    style={{ background: col.bg, color: col.fg }}
                  >
                    <span className="min-w-0">
                      <span className="block truncate text-[12px] font-bold">{label(k)}</span>
                      <span className="font-mono text-[9.5px] tabular-nums opacity-80">
                        {formatCompact(k.repos)} · {k.intensity} ★/{t('mapPerRepo')}
                      </span>
                    </span>
                    <span className="shrink-0 font-mono text-[12px] font-bold tabular-nums">
                      +{formatCompact(k.growth)}
                    </span>
                  </Link>
                );
              });
            })()}
          </div>
        </div>
      )}
    </div>
  );
}
