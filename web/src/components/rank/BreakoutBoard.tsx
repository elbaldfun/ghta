'use client';

import { useEffect, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { useTranslations } from 'next-intl';
import { Link } from '@/i18n/navigation';
import type { HotRepo, HotWindow } from '@/lib/data';
import { formatCompact, langColor } from '@/lib/rank-data';
import { StarIcon } from './icons';

// Heat by the repo's position on the current window's board — the top gainers
// burn hottest — so the ramp reads the same whether the window is a day or a
// month, without pretending a 5%/day and a 5%/month gain mean the same thing.
function heatByRank(rank: number, total: number): string {
  const q = total <= 1 ? 0 : (rank - 1) / (total - 1);
  if (q < 0.05) return 'rgb(var(--heat4))';
  if (q < 0.15) return 'rgb(var(--heat3))';
  if (q < 0.4) return 'rgb(var(--heat2))';
  if (q < 0.75) return 'rgb(var(--heat1))';
  return 'rgb(var(--heat0))';
}

function Row({ h, rank, total, gainLabel }: { h: HotRepo; rank: number; total: number; gainLabel: string }) {
  const t = useTranslations('rank');
  const dot = langColor(h.repo.language);
  const color = heatByRank(rank, total);

  const inner = (
    <>
      <span className="font-mono text-[15px] font-bold tabular-nums tracking-tight" style={{ color: rank <= 3 ? color : undefined }}>
        {rank}
      </span>

      <span className="min-w-0">
        <span className="flex items-center gap-2">
          <span className="inline-block h-[8px] w-[8px] shrink-0 rounded-full" style={{ backgroundColor: dot }} />
          <span className="truncate text-[13.5px] font-semibold">
            <span className="text-muted">{h.repo.owner}/</span>
            {h.repo.name}
          </span>
          {!h.inCorpus && (
            <span className="shrink-0 rounded-[3px] border border-accent px-1 py-px font-mono text-[9px] font-bold text-accent">
              {t('hotNew')}
            </span>
          )}
        </span>
        {h.repo.description && (
          <span className="mt-0.5 block truncate pl-[16px] text-[11.5px] text-muted">{h.repo.description}</span>
        )}
        {(h.repo.language || (h.repo.type && h.repo.type !== 'software')) && (
          <span className="mt-1 flex flex-wrap gap-1 pl-[16px]">
            {h.repo.language && (
              <span
                className="rounded-[3px] px-1.5 py-px font-mono text-[9.5px] font-semibold text-white opacity-90"
                style={{ backgroundColor: dot }}
              >
                {h.repo.language}
              </span>
            )}
            {h.repo.type && h.repo.type !== 'software' && (
              <span className="rounded-[3px] border border-border bg-surface2 px-1.5 py-px font-mono text-[9.5px] text-muted">
                {h.repo.type}
              </span>
            )}
          </span>
        )}
      </span>

      <span className="text-right">
        <span className="block font-mono text-[16px] font-bold leading-none tabular-nums tracking-tight" style={{ color }}>
          +{formatCompact(h.starsGained)}
        </span>
        <span className="mt-0.5 block font-mono text-[9.5px] uppercase tracking-[0.06em] text-muted">{gainLabel}</span>
      </span>

      <span className="hidden w-[64px] items-center justify-end gap-1 font-mono text-[13px] font-bold tabular-nums sm:flex">
        <StarIcon size={12} className="text-accent2" />
        {formatCompact(h.repo.stars)}
      </span>
    </>
  );

  const cls =
    'grid grid-cols-[36px_1fr_auto] items-center gap-2.5 border-b border-border px-3 py-2.5 transition-colors last:border-b-0 hover:bg-surface2 sm:grid-cols-[40px_1fr_92px_64px] sm:gap-3.5 sm:px-4';

  return h.inCorpus ? (
    <Link href={`/repo/${h.repo.owner}/${h.repo.name}`} className={cls}>
      {inner}
    </Link>
  ) : (
    <a href={h.repo.htmlUrl} target="_blank" rel="noopener noreferrer" className={cls}>
      {inner}
    </a>
  );
}

/**
 * The dedicated breakout board: three pre-fetched windows (day / week / month)
 * with an instant client toggle, each a dense heat-ranked list of the fastest-
 * rising repos. Tracked repos link to their detail page; newcomers we don't yet
 * track are flagged NEW and link out to GitHub.
 */
export function BreakoutBoard({
  daily,
  weekly,
  monthly,
  initial = 'weekly',
}: {
  daily: HotRepo[];
  weekly: HotRepo[];
  monthly: HotRepo[];
  initial?: HotWindow;
}) {
  const t = useTranslations('rank');
  // URL is the source of truth for the active window, so a Back-navigation into
  // this page (which remounts the component from the router cache) restores the
  // board the visitor left rather than the server-baked default. Falls back to
  // the server-resolved initial when there is no ?since= yet.
  const params = useSearchParams();
  const fromUrl = params.get('since');
  const start: HotWindow = fromUrl === 'daily' || fromUrl === 'weekly' || fromUrl === 'monthly' ? fromUrl : initial;
  const [win, setWin] = useState<HotWindow>(start);

  // Mirror the active window into the URL (?since=…) without a navigation or
  // refetch — all three windows are already in props. This keeps the tab in the
  // history entry, so returning here from a repo detail page (via Back) lands on
  // the same board the visitor left. 'weekly' is the default, so it stays clean.
  useEffect(() => {
    const url = win === 'weekly' ? window.location.pathname : `${window.location.pathname}?since=${win}`;
    window.history.replaceState(window.history.state, '', url);
  }, [win]);

  const sets: Record<HotWindow, HotRepo[]> = { daily, weekly, monthly };
  const gainLabels: Record<HotWindow, string> = {
    daily: `${t('hotColGain')} / ${t('hotBarDay')}`,
    weekly: `${t('hotColGain')} / ${t('hotBarWeek')}`,
    monthly: `${t('hotColGain')} / ${t('hotBarMonth')}`,
  };
  const items = [...sets[win]].sort((a, b) => b.starsGained - a.starsGained);

  const tab = (key: HotWindow, label: string) => (
    <button
      key={key}
      onClick={() => setWin(key)}
      aria-pressed={win === key}
      className={`rounded-lg border px-[13px] py-[7px] text-[12.5px] font-bold transition-colors ${
        win === key ? 'border-accent bg-accent/10 text-accent' : 'border-transparent text-muted hover:text-fg'
      }`}
    >
      {label}
    </button>
  );

  return (
    <div>
      <div className="mb-[18px] -ml-[14px] flex items-center gap-1">
        {tab('daily', t('hotBarDay'))}
        {tab('weekly', t('hotBarWeek'))}
        {tab('monthly', t('hotBarMonth'))}
      </div>

      {items.length === 0 ? (
        <div className="py-10 text-center text-[13px] text-muted">{t('hotEmpty')}</div>
      ) : (
        <div className="overflow-hidden rounded-card border border-border bg-surface">
          <div className="grid grid-cols-[36px_1fr_auto] items-center gap-2.5 border-b border-border px-3 py-2 font-mono text-[10px] uppercase tracking-[0.08em] text-muted sm:grid-cols-[40px_1fr_92px_64px] sm:gap-3.5 sm:px-4">
            <div>#</div>
            <div>{t('mapColRepo')}</div>
            <div className="text-right">{t('hotColGain')}</div>
            <div className="hidden text-right sm:block">{t('mapColStars')}</div>
          </div>
          {items.map((h, i) => (
            <Row key={h.repo.fullName} h={h} rank={i + 1} total={items.length} gainLabel={gainLabels[win]} />
          ))}
        </div>
      )}
    </div>
  );
}
