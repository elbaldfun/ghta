import { useTranslations } from 'next-intl';
import { Link } from '@/i18n/navigation';
import { PageShell } from './PageShell';

/** Single-row footer: full-bleed top border, content aligned to the site container. */
export function RankFooter() {
  const t = useTranslations('rank');
  const tp = useTranslations('privacy');
  const year = new Date().getFullYear();

  return (
    <footer className="mt-10 border-t border-border">
      <PageShell className="flex flex-wrap items-center justify-between gap-x-5 gap-y-2 py-6 text-xs text-muted">
        <span>
          © {year} <span className="font-semibold">StarRank Explorer</span> · {t('footerTagline')}
        </span>
        <nav className="flex flex-wrap items-center gap-x-4 gap-y-1">
          <Link href="/" className="hover:text-fg">
            {t('navRankings')}
          </Link>
          <Link href="/privacy" className="hover:text-fg">
            {tp('title')}
          </Link>
          <a
            href="https://github.com"
            target="_blank"
            rel="noopener noreferrer"
            className="hover:text-fg"
          >
            {t('footerDataSource')}
          </a>
        </nav>
      </PageShell>
    </footer>
  );
}
