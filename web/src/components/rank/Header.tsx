import { Suspense } from 'react';
import { useTranslations } from 'next-intl';
import { Link } from '@/i18n/navigation';
import { SearchBox } from './SearchBox';
import { LocalePills } from './LocalePills';
import { ThemePill } from './ThemePill';

/** Persistent 2a header: brand + nav, search pill, locale pills, theme toggle. */
export function RankHeader() {
  const t = useTranslations('rank');

  return (
    <header className="flex items-center justify-between gap-5 border-b border-border px-7 py-5">
      <div className="flex items-center gap-3.5 whitespace-nowrap">
        <Link href="/" className="flex items-baseline gap-2.5">
          <span className="font-display text-[21px] font-extrabold text-accent">StarRank</span>
          <span className="text-xs text-muted">Explorer</span>
        </Link>
        <nav className="flex items-center gap-1">
          <Link
            href="/"
            className="rounded-lg bg-accent px-[13px] py-[7px] text-[12.5px] font-bold text-accent-fg"
          >
            {t('navRankings')}
          </Link>
        </nav>
      </div>
      <Suspense>
        <SearchBox />
      </Suspense>
      <div className="flex items-center gap-1.5">
        <Suspense>
          <LocalePills />
        </Suspense>
        <ThemePill />
      </div>
    </header>
  );
}
