'use client';

import { useState } from 'react';
import { useTranslations } from 'next-intl';
import { Link } from '@/i18n/navigation';
import type { HotRepo } from '@/lib/data';
import { formatCompact } from '@/lib/rank-data';

/**
 * A single compact "what's hot" bar for the landing page — one horizontal row of
 * breakout repos with a week/month toggle, at roughly a quarter the height of the
 * old stacked carousels, so the corpus leaderboard reaches the fold. Repos we
 * track link to their detail page; newcomers link out to GitHub.
 */
export function HotBar({ weekly, monthly }: { weekly: HotRepo[]; monthly: HotRepo[] }) {
  const t = useTranslations('rank');
  const [window, setWindow] = useState<'weekly' | 'monthly'>('weekly');
  const items = window === 'weekly' ? weekly : monthly;
  if (weekly.length === 0 && monthly.length === 0) return null;

  const tab = (key: 'weekly' | 'monthly', label: string) => (
    <button
      onClick={() => setWindow(key)}
      aria-pressed={window === key}
      className={`rounded-md px-2 py-0.5 text-[11.5px] font-semibold transition-colors ${
        window === key ? 'text-accent' : 'text-muted hover:text-fg'
      }`}
    >
      {label}
    </button>
  );

  return (
    <section className="border-b border-border px-[26px] py-3" aria-label={t('hotBarTitle')}>
      <div className="mb-2 flex items-center gap-3">
        <span className="flex items-center gap-1.5 text-[13px] font-extrabold">
          <span aria-hidden="true">🔥</span>
          {t('hotBarTitle')}
        </span>
        <div className="flex items-center gap-0.5">
          {tab('weekly', t('hotBarWeek'))}
          {tab('monthly', t('hotBarMonth'))}
        </div>
        <Link href="/ecosystem" className="ml-auto text-[11.5px] font-bold text-accent hover:underline">
          {t('seeAll')} →
        </Link>
      </div>

      <div className="scrollbar-hide flex gap-2 overflow-x-auto pb-0.5">
        {items.slice(0, 12).map((h, i) => {
          const inner = (
            <>
              <span className="w-4 shrink-0 text-right font-mono text-[11px] font-bold tabular-nums text-muted">
                {i + 1}
              </span>
              <span className="min-w-0">
                <span className="block truncate text-[12.5px] font-semibold leading-tight">
                  <span className="text-muted">{h.repo.owner}/</span>
                  {h.repo.name}
                </span>
                <span className="font-mono text-[10.5px] font-bold tabular-nums text-accent2">
                  +{formatCompact(h.starsGained)} ★
                </span>
                <span className="ml-1 font-mono text-[10px] tabular-nums text-muted">
                  {formatCompact(h.repo.stars)}
                </span>
              </span>
            </>
          );
          const cls =
            'flex w-[188px] shrink-0 items-center gap-2 rounded-[7px] border border-border bg-surface px-2.5 py-1.5 transition-colors hover:border-accent';
          return h.inCorpus ? (
            <Link key={h.repo.fullName} href={`/repo/${h.repo.owner}/${h.repo.name}`} className={cls}>
              {inner}
            </Link>
          ) : (
            <a
              key={h.repo.fullName}
              href={h.repo.htmlUrl}
              target="_blank"
              rel="noopener noreferrer"
              className={cls}
            >
              {inner}
            </a>
          );
        })}
      </div>
    </section>
  );
}
