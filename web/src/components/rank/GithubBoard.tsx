import type { Metadata } from 'next';
import { getTranslations } from 'next-intl/server';
import { permanentRedirect } from 'next/navigation';
import { Link } from '@/i18n/navigation';
import {
  categoryLabel,
  findCategory,
  getCategoryTree,
  getFacets,
  getHot,
  getTotalCount,
  formatTotal,
  searchRepos,
  type CategoryNode,
  type HotRepo,
  type TypeFacet,
} from '@/lib/data';
import { SORT_OPTIONS, type SortOption } from '@/lib/rank-data';
import { boardHref, boardPath, type BoardFilters } from '@/lib/board-url';
import { alternatesFor } from '@/lib/seo';
import { CategoryTree } from './CategoryTree';
import { ErrorState } from './ErrorState';
import { FilterBar } from './FilterBar';
import { HotBar } from './HotBar';
import { Pagination } from './Pagination';
import { RepoRow } from './RepoRow';
import { PageTabs } from './PageTabs';
import { PageShell } from './PageShell';

const PER_PAGE = 24;

/**
 * Sends a request to the one URL that shows these filters, so every board has
 * a single indexable address. `current` is the path being served.
 */
export function redirectToCanonical(locale: string, current: string, f: BoardFilters) {
  if (boardPath(f) !== current) permanentRedirect(`/${locale}${boardHref(f)}`);
}

function facetLabel(t: Awaited<ReturnType<typeof getTranslations>>, facets: TypeFacet[], key: string) {
  if (t.has(`typeFacet.${key}`)) return t(`typeFacet.${key}`);
  return facets.find((f) => f.key === key)?.name ?? key;
}

/** What the board is about, for its heading and title; null for all repos. */
function subject(
  t: Awaited<ReturnType<typeof getTranslations>>,
  locale: string,
  tree: CategoryNode[],
  facets: TypeFacet[],
  f: BoardFilters,
): string | null {
  if (f.category) {
    const node = findCategory(tree, f.category);
    return node ? categoryLabel(node, locale) : null;
  }
  if (f.lang) return f.lang;
  if (f.type) return facetLabel(t, facets, f.type);
  return null;
}

export async function boardMetadata(locale: string, f: BoardFilters): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'rank' });
  const [tree, facets] = await Promise.all([
    f.category ? getCategoryTree() : Promise.resolve([]),
    f.type ? getFacets() : Promise.resolve([]),
  ]);
  const name = subject(t, locale, tree, facets, f);
  return {
    title: name ? t('boardTitle', { name }) : t('homeTitle'),
    description: name ? t('boardDescription', { name }) : t('homeSubtitle'),
    alternates: alternatesFor(locale, boardPath(f)),
  };
}

/** The GitHub ranking: domain tree, filters and the paginated list. */
export async function GithubBoard({ locale, filters: f }: { locale: string; filters: BoardFilters }) {
  const t = await getTranslations('rank');

  // Searches default to relevance; browsing (no query) has no relevance to
  // rank by, so it falls back to stars.
  let sort: SortOption = SORT_OPTIONS.includes(f.sort as SortOption)
    ? (f.sort as SortOption)
    : f.q
      ? 'relevance'
      : 'stars';
  if (sort === 'relevance' && !f.q) sort = 'stars';
  const page = f.page ?? 1;

  // The "hot this week" strip only makes sense on the unfiltered landing view.
  const isLanding = !f.category && !f.type && !f.q && !f.lang && !f.license && page === 1;

  const [tree, facets, total, searchRes, hotWeekly, hotMonthly] = await Promise.all([
    getCategoryTree(),
    getFacets(),
    getTotalCount(),
    searchRepos({
      category: f.category,
      type: f.type,
      q: f.q,
      language: f.lang,
      license: f.license,
      sort,
      page,
      perPage: PER_PAGE,
    }),
    isLanding ? getHot('weekly', 12) : Promise.resolve([] as HotRepo[]),
    isLanding ? getHot('monthly', 12) : Promise.resolve([] as HotRepo[]),
  ]);

  const heading = subject(t, locale, tree, facets, f) ?? t('homeTitle');
  // Breadcrumb: for a leaf ("a/b"), link its parent.
  const parentPath = f.category?.includes('/') ? f.category.split('/')[0] : null;
  const parentNode = parentPath ? findCategory(tree, parentPath) : undefined;

  return (
    <>
      {isLanding && <HotBar weekly={hotWeekly} monthly={hotMonthly} />}
      {/* gutter=false: the sidebar owns its background up to the container
          edge; the content column applies the gutter itself. */}
      <PageShell gutter={false} className="grid min-h-[620px] grid-cols-1 lg:grid-cols-[250px_1fr]">
        <CategoryTree tree={tree} total={total} filters={f} />

        <div className="px-4 py-[22px] lg:px-gutter">
          <PageTabs items={[{ href: '/github', label: t('tabList') }, { href: '/map', label: t('navMap') }]} />
          <div className="mb-[18px] flex flex-wrap items-end justify-between gap-x-5 gap-y-4">
            <div>
              {parentNode && (
                <Link
                  href={boardHref({ category: parentNode.path })}
                  className="mb-[5px] block text-[11px] font-semibold tracking-wide text-muted hover:text-fg"
                >
                  {categoryLabel(parentNode, locale)}
                </Link>
              )}
              <div className="flex items-baseline gap-2.5">
                <h1 className="font-display text-lg font-extrabold">{heading}</h1>
                <span className="text-xs text-muted">
                  {f.q && `${t('searchResultsFor')} "${f.q}" · `}
                  {searchRes.data ? `${formatTotal(searchRes.data.totalCount)} ${t('results')}` : ''}
                </span>
              </div>
            </div>
            <FilterBar types={facets} filters={f} />
          </div>

          {searchRes.error !== null ? (
            <ErrorState />
          ) : searchRes.data.items.length === 0 ? (
            <div className="flex flex-col items-center gap-3 py-10 text-center">
              <p className="text-[13px] text-muted">{t('noResults')}</p>
              <Link
                href="/github"
                className="rounded-full border border-border px-4 py-1.5 text-[12.5px] font-bold text-fg transition-colors hover:bg-surface2"
              >
                {t('clearFilters')}
              </Link>
            </div>
          ) : (
            <>
              <div className="overflow-hidden rounded-card border border-border bg-surface">
                {searchRes.data.items.map((repo, i) => (
                  <RepoRow
                    key={repo.fullName}
                    repo={repo}
                    rank={(page - 1) * PER_PAGE + i + 1}
                    showUpdated={sort === 'updated'}
                    growthWindow={sort === 'weekly' ? 'weekly' : 'daily'}
                  />
                ))}
              </div>
              <Pagination
                page={page}
                perPage={PER_PAGE}
                totalCount={searchRes.data.totalCount}
                href={(p) => boardHref({ ...f, page: p })}
              />
            </>
          )}
        </div>
      </PageShell>
    </>
  );
}
