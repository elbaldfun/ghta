import { Suspense } from 'react';
import { useTranslations } from 'next-intl';
import { Link } from '@/i18n/navigation';
import { NavLink } from './NavLink';
import { PageShell } from './PageShell';
import { SearchBox } from './SearchBox';
import { LocaleMenu } from './LocaleMenu';
import { ThemePill } from './ThemePill';

/**
 * Site header in design tokens v1 ("数据报刊"): paper ground, ink type, the
 * brand green only in the logo dot. Pages that still use the legacy palette
 * sit under it unchanged; paper and the legacy background are a shade apart.
 */
export function RankHeader() {
  const t = useTranslations('rank');
  const tb = useTranslations('blog');

  return (
    <header className="border-b border-rule bg-paper font-text text-ink">
      {/* Mobile: logo + controls, then a scrolling nav row, then search.
          From lg up everything sits on one line. */}
      <PageShell className="flex flex-wrap items-center gap-x-6 gap-y-2 pt-3 lg:flex-nowrap lg:py-3">
        <Link href="/" className="order-1 flex h-11 items-center gap-2 text-[20px] font-bold tracking-[-0.02em] text-ink">
          <span className="h-2.5 w-2.5 rounded-full bg-brand" aria-hidden="true" />
          StarRank
        </Link>
        <nav
          aria-label={t('navMain')}
          className="order-3 -mx-4 flex w-[calc(100%+2rem)] items-center gap-5 overflow-x-auto px-4 text-sr-small font-medium lg:order-2 lg:mx-0 lg:w-auto lg:gap-4 lg:overflow-visible lg:px-0 xl:gap-7"
        >
          <NavLink href="/" label={t('navToday')} />
          <NavLink href="/ai" label={t('navAi')} alsoOn={['/ecosystem']} />
          <NavLink href="/github" label="GitHub" alsoOn={['/breakout', '/topics', '/map', '/repo']} />
          <NavLink href="/apps" label={t('navApps')} alsoOn={['/alternatives']} />
          {/* A brand name, the same in every locale. */}
          <NavLink href="/ios" label="App Store" />
          <NavLink href="/models" label={t('navModels')} />
          <NavLink href="/developers" label={t('navDevelopers')} />
          <NavLink href="/blog" label={tb('nav')} />
        </nav>
        <div className="order-4 w-full pb-3 lg:order-3 lg:ml-auto lg:w-auto lg:pb-0 xl:max-w-[360px] xl:flex-1">
          <Suspense>
            <SearchBox />
          </Suspense>
        </div>
        <div className="order-2 ml-auto flex items-center gap-2 lg:order-4 lg:ml-0">
          <Suspense>
            <LocaleMenu />
          </Suspense>
          <ThemePill />
        </div>
      </PageShell>
    </header>
  );
}
