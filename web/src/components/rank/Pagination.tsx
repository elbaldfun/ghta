import { useTranslations } from 'next-intl';
import { Link } from '@/i18n/navigation';
import { ChevronLeft, ChevronRight } from './icons';

function pageHref(params: Record<string, string | undefined>, page: number) {
  const next = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) if (v) next.set(k, v);
  if (page > 1) next.set('page', String(page));
  else next.delete('page');
  const qs = next.toString();
  return qs ? `/?${qs}` : '/';
}

export function Pagination({
  page,
  perPage,
  totalCount,
  params,
}: {
  page: number;
  perPage: number;
  totalCount: number;
  params: Record<string, string | undefined>;
}) {
  const t = useTranslations('rank');
  // GitHub search only exposes the first 1000 results.
  const maxPage = Math.max(1, Math.min(Math.ceil(totalCount / perPage), Math.floor(1000 / perPage)));
  if (maxPage <= 1) return null;

  const start = (page - 1) * perPage + 1;
  const end = Math.min(page * perPage, totalCount);
  const windowStart = Math.max(1, Math.min(page - 2, maxPage - 4));
  const numbers = Array.from({ length: Math.min(5, maxPage) }, (_, i) => windowStart + i);

  const navBtn =
    'flex items-center gap-1 rounded-lg border border-border bg-surface px-[11px] py-1.5 text-xs font-semibold text-fg';

  return (
    <div className="mt-[22px] flex flex-wrap items-center justify-between gap-3">
      <span className="text-xs text-muted">
        {t('rangeText', { start, end, total: totalCount.toLocaleString() })}
      </span>
      <div className="flex items-center gap-1.5">
        {page > 1 ? (
          <Link href={pageHref(params, page - 1)} className={navBtn} aria-label="Previous page">
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
            href={pageHref(params, n)}
            aria-current={n === page ? 'page' : undefined}
            className={`min-w-[32px] rounded-lg border py-1.5 text-center text-xs font-bold ${
              n === page
                ? 'border-accent bg-accent text-accent-fg'
                : 'border-border bg-surface text-fg hover:border-accent'
            }`}
          >
            {n}
          </Link>
        ))}
        {page < maxPage ? (
          <Link href={pageHref(params, page + 1)} className={navBtn} aria-label="Next page">
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
