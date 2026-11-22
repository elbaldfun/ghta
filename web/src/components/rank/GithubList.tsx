import { getTranslations } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import {
  categoryLabel,
  findCategory,
  formatTotal,
  getCategoryTree,
  getFacets,
  getTrends,
  searchRepos,
  type CategoryNode,
  type RepoSummary,
  type TypeFacet,
} from '@/lib/data';
import { FILTER_LANGS, FILTER_LICENSES, SORT_OPTIONS, type SortOption } from '@/lib/rank-data';
import { boardHref, type BoardFilters } from '@/lib/board-url';
import { dailySeries } from '@/lib/front-page';
import { DailyBars, Delta, Dropdown, Segmented, fmtTotal, srButton } from '@/components/ui';
import { ErrorState } from './ErrorState';
import { Pagination } from './Pagination';
import { PageShell } from './PageShell';
import { RankMove } from './RankMove';

const PER_PAGE = 24;
// The sorts the segmented control offers. Forks stays reachable by URL.
const SEGMENT_SORTS = ['stars', 'daily', 'weekly', 'updated'] as const;

type T = Awaited<ReturnType<typeof getTranslations>>;

function facetLabel(t: T, facets: TypeFacet[], key: string): string {
  if (t.has(`typeFacet.${key}`)) return t(`typeFacet.${key}`);
  return facets.find((f) => f.key === key)?.name ?? key;
}

/** Whole days since an ISO time, at least 0. */
function daysAgo(iso: string, now: number): number | null {
  const ms = Date.parse(iso);
  if (!Number.isFinite(ms)) return null;
  return Math.max(0, Math.floor((now - ms) / 86_400_000));
}

/**
 * The GitHub ranking without a domain: every repository, or one language or
 * form. Design tokens v1. A domain (/github/c/…) still renders GithubBoard
 * with its category tree until the domain page is redesigned.
 */
export async function GithubList({ locale, filters: f }: { locale: string; filters: BoardFilters }) {
  const t = await getTranslations('rank');
  const b = await getTranslations('board');
  const now = Date.now();

  // Searches default to relevance; browsing has none, so it falls back to stars.
  let sort: SortOption = SORT_OPTIONS.includes(f.sort as SortOption)
    ? (f.sort as SortOption)
    : f.q
      ? 'relevance'
      : 'stars';
  if (sort === 'relevance' && !f.q) sort = 'stars';
  const page = f.page ?? 1;

  const [tree, facets, res] = await Promise.all([
    getCategoryTree(),
    getFacets(),
    searchRepos({ type: f.type, q: f.q, language: f.lang, license: f.license, sort, page, perPage: PER_PAGE }),
  ]);
  const items = res.data?.items ?? [];
  // 31 days of readings for the daily bars (the endpoint takes up to 60 ids).
  const trends = items.length ? await getTrends(items.map((r) => r.fullName), 31) : {};

  const subject = f.lang ? f.lang : f.type ? facetLabel(t, facets, f.type) : null;
  const heading = subject ? b('titleFor', { name: subject }) : b('title');
  // The move since yesterday is in the ranking the filters name, as the API
  // picks it (the language, else overall). It is the row's own number only in
  // star order and without a search.
  const moveScope = sort === 'stars' && !f.q ? (f.lang ?? t('rankOverall')) : undefined;
  const filtered = Boolean(f.lang || f.type || f.license || f.q);

  // A repo's domain leaf, for the meta line ("TypeScript · 逆向与分析").
  const leafLabel = (r: RepoSummary): string | null => {
    const p = r.categoryPath[0];
    const node = p ? findCategory(tree, p) : undefined;
    return node ? categoryLabel(node, locale) : null;
  };
  const meta = (r: RepoSummary) =>
    [r.language, leafLabel(r) ?? (r.type && r.type !== 'software' ? facetLabel(t, facets, r.type) : null)]
      .filter(Boolean)
      .join(' · ');

  const all = b('all');
  const groups: CategoryNode[] = tree;
  const dropdowns = [
    {
      key: 'category',
      label: b('filterCategory', { value: all }),
      active: false,
      items: [
        { key: 'all', label: all, href: boardHref({ ...f, category: undefined, page: undefined }), active: true },
        ...groups.map((g) => ({
          key: g.path,
          label: categoryLabel(g, locale),
          href: boardHref({ ...f, category: g.path, page: undefined }),
        })),
      ],
    },
    {
      key: 'lang',
      label: f.lang ? b('filterLang', { value: f.lang }) : b('filterLangAll'),
      active: Boolean(f.lang),
      items: [
        { key: 'all', label: all, href: boardHref({ ...f, lang: undefined, page: undefined }), active: !f.lang },
        ...FILTER_LANGS.map((l) => ({
          key: l,
          label: l,
          href: boardHref({ ...f, lang: l, page: undefined }),
          active: f.lang === l,
        })),
      ],
    },
    {
      key: 'type',
      label: f.type ? b('filterType', { value: facetLabel(t, facets, f.type) }) : b('filterTypeAll'),
      active: Boolean(f.type),
      items: [
        { key: 'all', label: all, href: boardHref({ ...f, type: undefined, page: undefined }), active: !f.type },
        ...facets
          .filter((x) => x.count > 0)
          .map((x) => ({
            key: x.key,
            label: facetLabel(t, facets, x.key),
            href: boardHref({ ...f, type: x.key, page: undefined }),
            active: f.type === x.key,
          })),
      ],
    },
    {
      key: 'license',
      label: f.license ? b('filterLicense', { value: f.license.toUpperCase() }) : b('filterLicenseAll'),
      active: Boolean(f.license),
      items: [
        { key: 'all', label: all, href: boardHref({ ...f, license: undefined, page: undefined }), active: !f.license },
        ...FILTER_LICENSES.map((l) => ({
          key: l,
          label: l.toUpperCase(),
          href: boardHref({ ...f, license: l, page: undefined }),
          active: f.license === l,
        })),
      ],
    },
  ];

  const sortLabel: Record<string, string> = {
    relevance: b('sortRelevance'),
    stars: b('sortStars'),
    daily: b('sortDaily'),
    weekly: b('sortWeekly'),
    updated: b('sortUpdated'),
  };
  const sorts = [...(f.q ? (['relevance'] as const) : []), ...SEGMENT_SORTS].map((s) => ({
    key: s,
    label: sortLabel[s],
    // The default sort is left out of the URL so each view has one address.
    href: boardHref({ ...f, sort: s === (f.q ? 'relevance' : 'stars') ? undefined : s, page: undefined }),
    active: sort === s,
  }));

  const updatedView = sort === 'updated';
  const rulesKey = sort === 'daily' || sort === 'weekly' || sort === 'updated' ? `rules_${sort}` : 'rules_stars';
  const cols =
    'md:grid-cols-[44px_40px_minmax(0,1fr)_150px_76px_84px_76px] lg:grid-cols-[44px_40px_minmax(0,1fr)_210px_88px_96px_88px]';

  return (
    <PageShell className="pb-12">
      <section className="flex flex-col gap-2 pb-4 pt-6 md:pb-6 md:pt-10">
        <p className="text-sr-caption text-ink-3">
          GitHub · {subject ?? b('allCategories')}
        </p>
        <div className="flex flex-wrap items-end justify-between gap-x-4 gap-y-2">
          <h1 className="break-words font-news text-[30px] font-medium leading-[1.1] tracking-[-0.015em] [overflow-wrap:anywhere] md:text-sr-h1">
            {heading}
          </h1>
          <span className="text-sr-small text-ink-3">
            {f.q && (
              <>
                {b('searchFor', { q: f.q })} ·{' '}
              </>
            )}
            {res.data ? b('count', { n: formatTotal(res.data.totalCount) }) : ''}
            {' · '}
            {b('updated')}
          </span>
        </div>
      </section>

      <div className="flex flex-col gap-3 border-y border-rule py-3 md:flex-row md:items-center md:gap-2">
        <div role="toolbar" aria-label={b('filtersLabel')} className="flex flex-wrap items-center gap-2">
          {dropdowns.map((d) => (
            <Dropdown key={d.key} label={d.label} active={d.active} items={d.items} />
          ))}
          {filtered && (
            <Link href="/github" className="px-1 text-sr-small text-ink-2 underline underline-offset-4 hover:text-ink">
              {b('clear')}
            </Link>
          )}
        </div>
        <span className="hidden flex-1 md:block" />
        {/* Its own scroll row on phones: the page itself never scrolls sideways. */}
        <div className="-mx-4 overflow-x-auto px-4 md:mx-0 md:overflow-visible md:px-0">
          <Segmented items={sorts} label={b('sortLabel')} />
        </div>
      </div>

      {res.error !== null ? (
        <ErrorState />
      ) : items.length === 0 ? (
        <div className="flex flex-col items-start gap-3 py-10">
          <p className="text-sr-body text-ink-2">{t('noResults')}</p>
          <Link href="/github" className={srButton('outline')}>
            {t('clearFilters')}
          </Link>
        </div>
      ) : (
        <>
          <div role="table" aria-label={heading}>
            <div
              role="row"
              className={`sticky top-0 z-10 hidden gap-x-5 border-b border-ink bg-paper py-2.5 text-sr-caption text-ink-3 md:grid ${cols}`}
            >
              <span role="columnheader">{b('colRank')}</span>
              <span role="columnheader">{b('colMove')}</span>
              <span role="columnheader">{b('colProject')}</span>
              <span role="columnheader">{b('colBars')}</span>
              <span role="columnheader" className="text-right">
                {b('colToday')}
              </span>
              <span role="columnheader" className="text-right">
                {updatedView ? b('colPushed') : b('col7d')}
              </span>
              <span role="columnheader" className="text-right">
                {b('colStars')}
              </span>
            </div>
            {items.map((r, i) => {
              const rank = (page - 1) * PER_PAGE + i + 1;
              const series = dailySeries(trends[r.fullName]);
              const m = meta(r);
              const pushed = daysAgo(r.pushedAt, now);
              const bars = (
                <DailyBars
                  values={series}
                  height="100%"
                  label={b('barsAria', { name: r.name, n: series.length })}
                />
              );
              return (
                <Link
                  key={r.fullName}
                  role="row"
                  href={`/repo/${r.owner}/${r.name}`}
                  className={`grid grid-cols-[32px_minmax(0,1fr)_auto] items-baseline gap-x-2.5 gap-y-1.5 border-b border-rule py-3.5 hover:bg-sunk md:items-center md:gap-x-5 ${cols}`}
                >
                  <span role="cell" className="text-[17px] font-semibold tabular-nums tracking-[-0.02em] md:text-[20px]">
                    {rank}
                  </span>
                  <span role="cell" className="hidden md:block">
                    {moveScope && <RankMove change={r.rankChange} isNew={r.rankNew} scope={moveScope} />}
                  </span>
                  <span role="cell" className="flex min-w-0 flex-col gap-1">
                    <span className="flex min-w-0 items-baseline gap-2">
                      <span className="truncate text-[15px] font-semibold">
                        <span className="font-normal text-ink-3">{r.owner}/</span>
                        {r.name}
                      </span>
                      {moveScope && (
                        <RankMove change={r.rankChange} isNew={r.rankNew} scope={moveScope} className="shrink-0 md:hidden" />
                      )}
                    </span>
                    <span className="flex min-w-0 items-baseline gap-2.5 text-[13px] text-ink-2">
                      {m && <span className="min-w-0 truncate text-ink-3 md:shrink-0">{m}</span>}
                      {r.description && <span className="hidden truncate md:inline">{r.description}</span>}
                    </span>
                  </span>
                  <span role="cell" className="order-last col-start-2 h-[22px] md:order-none md:col-start-auto md:h-[30px] md:border-b md:border-rule">
                    {bars}
                  </span>
                  <span role="cell" className="order-last text-right text-[13px] font-semibold md:order-none md:text-[15px]">
                    <Delta value={r.dailyIncrease} />
                  </span>
                  <span role="cell" className="hidden text-right text-[15px] tabular-nums text-ink-2 md:block">
                    {updatedView
                      ? pushed == null
                        ? '—'
                        : b('daysAgo', { n: pushed })
                      : <Delta value={r.weeklyIncrease} hot={false} />}
                  </span>
                  <span role="cell" className="text-right text-[15px] font-semibold tabular-nums">
                    {fmtTotal(r.stars)}
                  </span>
                </Link>
              );
            })}
          </div>
          <Pagination
            tone="sr"
            page={page}
            perPage={PER_PAGE}
            totalCount={res.data.totalCount}
            href={(p) => boardHref({ ...f, page: p })}
          />
        </>
      )}

      <footer className="border-t-2 border-ink pt-4 text-[13px] leading-relaxed text-ink-2">
        <span className="font-semibold text-ink">{b('rulesLabel')}</span> {b(rulesKey)}{' '}
        {b('rulesCommon')}
      </footer>
    </PageShell>
  );
}
