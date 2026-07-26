'use client';

import { useState } from 'react';
import { useTranslations, useLocale } from 'next-intl';
import { Link } from '@/i18n/navigation';
import type { HeatCell } from '@/lib/data';
import { formatCompact } from '@/lib/rank-data';

/**
 * Intensity (daily stars per repo) drives the colour. Encoding scale in the
 * colour too would just restate the area, leaving the map with one signal;
 * keeping them separate is the whole point — the biggest field is usually not
 * the one moving.
 */
function heatToken(intensity: number): { bg: string; fg: string } {
  if (intensity >= 5) return { bg: 'rgb(var(--heat4))', fg: '#fff' };
  if (intensity >= 2.5) return { bg: 'rgb(var(--heat3))', fg: '#fff' };
  if (intensity >= 1.6) return { bg: 'rgb(var(--heat2))', fg: '#1a1305' };
  if (intensity >= 1.0) return { bg: 'rgb(var(--heat1))', fg: '#04140f' };
  return { bg: 'rgb(var(--heat0))', fg: '#0b0f0d' };
}

/**
 * Greedy row-packing treemap: cells are laid out largest-first into rows whose
 * heights are proportional to the area they carry, so each cell's area tracks
 * its repo count without needing a full squarify implementation.
 */
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

  if (cells.length === 0) return null;

  const label = (c: HeatCell) => (locale === 'zh' ? c.name : c.nameEn || c.name);
  const rows = pack(cells, [4, 4, 4]);
  const total = cells.reduce((s, c) => s + c.repos, 0);
  const active = open ? cells.find((c) => c.path === open) : null;

  return (
    <div>
      <div className="mb-3 flex flex-wrap items-center gap-x-5 gap-y-2">
        <span className="text-[10px] font-semibold uppercase tracking-[0.07em] text-muted">
          {t('mapAreaLegend')}
        </span>
        <span className="text-[10px] font-semibold uppercase tracking-[0.07em] text-muted">
          {t('mapColorLegend')}
        </span>
        <span className="ml-auto flex items-center gap-1">
          <span className="text-[9.5px] text-muted">{t('mapCold')}</span>
          {['var(--heat0)', 'var(--heat1)', 'var(--heat2)', 'var(--heat3)', 'var(--heat4)'].map((v) => (
            <i key={v} className="block h-[7px] w-4 rounded-[1px]" style={{ background: `rgb(${v})` }} />
          ))}
          <span className="text-[9.5px] text-muted">{t('mapHot')}</span>
        </span>
      </div>

      <div className="flex h-[340px] flex-col gap-[3px]">
        {rows.map((row, ri) => {
          const rowArea = row.reduce((s, c) => s + c.repos, 0);
          const small = ri === rows.length - 1 && rows.length > 2;
          return (
            <div key={ri} className="flex gap-[3px]" style={{ flex: `${rowArea / total} 1 0` }}>
              {row.map((c) => {
                const col = heatToken(c.intensity);
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

      {active && (active.children?.length ?? 0) > 0 && (
        <div className="mt-3 rounded-card border border-border bg-surface p-3.5">
          <div className="mb-2.5 flex items-baseline gap-2">
            <span className="text-[13px] font-bold">{label(active)}</span>
            <span className="text-[11px] text-muted">{t('mapDrill')}</span>
          </div>
          <div className="grid gap-[3px] sm:grid-cols-2 lg:grid-cols-3">
            {active.children!.map((k) => {
              const col = heatToken(k.intensity);
              return (
                <Link
                  key={k.path}
                  href={`/?category=${encodeURIComponent(k.path)}`}
                  className="flex items-center justify-between gap-2 rounded-[5px] px-2.5 py-2 transition-colors hover:brightness-[1.07]"
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
            })}
          </div>
        </div>
      )}
    </div>
  );
}
