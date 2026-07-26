import { Suspense } from 'react';
import { useTranslations } from 'next-intl';
import { Link } from '@/i18n/navigation';
import { NavLink } from './NavLink';
import { NavDropdown } from './NavDropdown';
import { SearchBox } from './SearchBox';
import { LocaleMenu } from './LocaleMenu';
import { ThemePill } from './ThemePill';

/** Persistent 2a header: full-bleed border, content aligned to the site container. */
export function RankHeader() {
  const t = useTranslations('rank');
  const tb = useTranslations('blog');

  return (
    <header className="border-b border-border">
      <div className="mx-auto flex max-w-screen-xl items-center justify-between gap-5 px-7 py-5">
        <div className="flex flex-wrap items-center gap-3.5">
          <Link href="/" className="flex items-baseline gap-2.5">
            <span className="font-display text-[21px] font-extrabold text-accent">StarRank</span>
            <span className="text-xs text-muted">Explorer</span>
          </Link>
          <nav className="flex flex-wrap items-center gap-1">
            <NavLink href="/" label={t('navRankings')} />
            <NavLink href="/breakout" label={t('navBreakout')} />
            <NavDropdown
              label={t('navAi')}
              items={[
                { href: '/ecosystem', label: t('navEcosystem') },
                { href: '/topics', label: t('navTopics') },
              ]}
            />
            <NavLink href="/apps" label={t('navApps')} />
            <NavLink href="/developers" label={t('navDevelopers')} />
            <NavLink href="/blog" label={tb('nav')} />
          </nav>
        </div>
        <Suspense>
          <SearchBox />
        </Suspense>
        <div className="flex items-center gap-1.5">
          <Suspense>
            <LocaleMenu />
          </Suspense>
          <ThemePill />
        </div>
      </div>
    </header>
  );
}
