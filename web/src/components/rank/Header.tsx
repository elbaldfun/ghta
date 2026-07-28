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
      {/* Mobile stacks three compact rows (logo+controls / nav / search);
          from lg up everything sits on one line. */}
      <div className="mx-auto flex max-w-screen-xl flex-wrap items-center gap-x-4 gap-y-3 px-4 py-4 lg:flex-nowrap lg:gap-x-5 lg:px-7 lg:py-5">
        <Link href="/" className="order-1 flex items-baseline gap-2.5">
          <span className="font-display text-[21px] font-extrabold text-accent">StarRank</span>
          <span className="hidden text-xs text-muted sm:inline">Explorer</span>
        </Link>
        <nav className="order-3 -mx-1 flex w-full flex-wrap items-center gap-1 lg:order-2 lg:mx-0 lg:w-auto">
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
        <div className="order-4 w-full lg:order-3 lg:w-auto lg:max-w-[460px] lg:flex-1">
          <Suspense>
            <SearchBox />
          </Suspense>
        </div>
        <div className="order-2 ml-auto flex items-center gap-1.5 lg:order-4 lg:ml-0">
          <Suspense>
            <LocaleMenu />
          </Suspense>
          <ThemePill />
        </div>
      </div>
    </header>
  );
}
