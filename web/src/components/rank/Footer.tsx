import { useTranslations } from 'next-intl';
import { Link } from '@/i18n/navigation';
import { PageShell } from './PageShell';

/** Single-row footer in design tokens v1: a 1px rule, ink-3 type on paper. */
export function RankFooter() {
  const t = useTranslations('rank');
  const tp = useTranslations('privacy');
  const tm = useTranslations('mcp');
  const year = new Date().getFullYear();

  return (
    <footer className="mt-10 border-t border-rule bg-paper font-text">
      <PageShell className="flex flex-wrap items-center justify-between gap-x-5 gap-y-2 py-6 text-sr-caption text-ink-3">
        <span>
          © {year} <span className="font-semibold text-ink-2">StarRank</span> · {t('footerTagline')}
        </span>
        <nav className="flex flex-wrap items-center gap-x-4 gap-y-1">
          <Link href="/" className="inline-flex min-h-11 items-center hover:text-ink md:min-h-0">
            {t('navRankings')}
          </Link>
          <Link href="/mcp" className="inline-flex min-h-11 items-center hover:text-ink md:min-h-0">
            {tm('footerLink')}
          </Link>
          <Link href="/privacy" className="inline-flex min-h-11 items-center hover:text-ink md:min-h-0">
            {tp('title')}
          </Link>
          <a
            href="https://github.com"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex min-h-11 items-center hover:text-ink md:min-h-0"
          >
            {t('footerDataSource')}
          </a>
        </nav>
      </PageShell>
    </footer>
  );
}
