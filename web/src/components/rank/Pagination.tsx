import { useTranslations } from 'next-intl';
import { Link } from '@/i18n/navigation';
import { ChevronLeft, ChevronRight } from './icons';

function pageHref(basePath: string, params: Record<string, string | undefined>, page: number) {
  const next = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) if (v) next.set(k, v);
  if (page > 1) next.set('page', String(page));
  else next.delete('page');
  const qs = next.toString();
  return qs ? `${basePath}?${qs}` : basePath;
}

export function Pagination({
  page,
  perPage,
  totalCount,
  params = {},
  basePath = '/',
  href,
  cap = 1000,
  tone = 'legacy',
}: {
  page: number;
  perPage: number;
  totalCount: number;
  params?: Record<string, string | undefined>;
  /** Route the page links target; defaults to the home grid. */
  basePath?: string;
  /** Builds a page's URL instead of basePath + params. */
  href?: (page: number) => string;
  /** Hard ceiling on reachable results (GitHub search exposes only 1000; our
   * own boards are capped by the backend's top-N). */
  cap?: number;
  /** 'sr': design tokens v1 (ink outlines, 44px touch targets). */
  tone?: 'legacy' | 'sr';
}) {
  const t = useTranslations('rank');
  const link = href ?? ((n: number) => pageHref(basePath, params, n));
  const maxPage = Math.max(1, Math.min(Math.ceil(totalCount / perPage), Math.floor(cap / perPage)));
  if (maxPage <= 1) return null;

  const start = (page - 1) * perPage + 1;
  const end = Math.min(page * perPage, totalCount);
  const windowStart = Math.max(1, Math.min(page - 2, maxPage - 4));
  const numbers = Array.from({ length: Math.min(5, maxPage) }, (_, i) => windowStart + i);

  const sr = tone === 'sr';
  const navBtn = sr
    ? 'flex h-11 min-w-11 items-center justify-center rounded border border-rule text-ink-2 hover:border-ink-3 hover:text-ink md:h-9'
    : 'flex items-center gap-1 rounded-lg border border-border bg-surface px-[11px] py-1.5 text-xs font-semibold text-fg';
  const numClass = (on: boolean) =>
    sr
      ? `flex h-11 min-w-11 items-center justify-center rounded border text-sr-small tabular-nums md:h-9 ${
          on ? 'border-ink font-semibold text-ink' : 'border-rule text-ink-2 hover:border-ink-3 hover:text-ink'
        }`
      : `min-w-[32px] rounded-lg border py-1.5 text-center text-xs font-bold ${
          on ? 'border-accent bg-accent text-accent-fg' : 'border-border bg-surface text-fg hover:border-accent'
        }`;

  return (
    <div className={`flex flex-wrap items-center justify-between gap-3 ${sr ? 'py-5' : 'mt-[22px]'}`}>
      <span className={sr ? 'text-sr-small tabular-nums text-ink-2' : 'text-xs text-muted'}>
        {t('rangeText', { start, end, total: totalCount.toLocaleString() })}
      </span>
      <div className={`flex items-center ${sr ? 'gap-2' : 'gap-1.5'}`}>
        {page > 1 ? (
          <Link href={link(page - 1)} className={navBtn} aria-label="Previous page">
            <ChevronLeft size={13} />
          </Link>
        ) : (
          <span className={`${navBtn} opacity-40`}>
            <ChevronLeft size={13} />
          </span>
        )}
        {numbers.map((n) => (
          <Link
            key={n}
            href={link(n)}
            aria-current={n === page ? 'page' : undefined}
            className={numClass(n === page)}
          >
            {n}
          </Link>
        ))}
        {page < maxPage ? (
          <Link href={link(page + 1)} className={navBtn} aria-label="Next page">
            <ChevronRight size={13} />
          </Link>
        ) : (
          <span className={`${navBtn} opacity-40`}>
            <ChevronRight size={13} />
          </span>
        )}
      </div>
    </div>
  );
}
