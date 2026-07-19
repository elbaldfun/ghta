import type { Metadata } from 'next';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { getCategoryCounts, searchRepos } from '@/lib/github';
import { SORT_OPTIONS, type SortOption } from '@/lib/rank-data';
import { CategoryTree } from '@/components/rank/CategoryTree';
import { FilterBar } from '@/components/rank/FilterBar';
import { Pagination } from '@/components/rank/Pagination';
import { RepoCard } from '@/components/rank/RepoCard';

const PER_PAGE = 24;

export async function generateMetadata({
  params: { locale },
}: {
  params: { locale: string };
}): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'rank' });
  return { title: t('homeTitle'), description: t('homeSubtitle') };
}

interface HomeSearchParams {
  cat?: string;
  sub?: string;
  q?: string;
  lang?: string;
  license?: string;
  sort?: string;
  page?: string;
}

export default async function RankHome({
  params: { locale },
  searchParams,
}: {
  params: { locale: string };
  searchParams: HomeSearchParams;
}) {
  setRequestLocale(locale);
  const t = await getTranslations('rank');

  const sort: SortOption = SORT_OPTIONS.includes(searchParams.sort as SortOption)
    ? (searchParams.sort as SortOption)
    : 'stars';
  const page = Math.max(1, Number(searchParams.page) || 1);

  const [countsRes, searchRes] = await Promise.all([
    getCategoryCounts(),
    searchRepos({
      cat: searchParams.cat,
      sub: searchParams.sub,
      q: searchParams.q,
      language: searchParams.lang,
      license: searchParams.license,
      sort,
      page,
      perPage: PER_PAGE,
    }),
  ]);

  const heading = searchParams.sub
    ? t(`subs.${searchParams.sub}`)
    : searchParams.cat
      ? t(`cats.${searchParams.cat}`)
      : t('homeTitle');
  const breadcrumb = searchParams.sub && searchParams.cat ? t(`cats.${searchParams.cat}`) : null;

  return (
    <div className="grid min-h-[620px] grid-cols-[250px_1fr]">
      <CategoryTree counts={countsRes} />

      <div className="px-[26px] py-[22px]">
        <div className="mb-[18px] flex flex-wrap items-end justify-between gap-x-5 gap-y-4">
          <div>
            {breadcrumb && (
              <div className="mb-[5px] text-[11px] font-semibold tracking-wide text-muted">{breadcrumb}</div>
            )}
            <div className="flex items-baseline gap-2.5">
              <h1 className="font-display text-lg font-extrabold">{heading}</h1>
              <span className="text-xs text-muted">
                {searchParams.q && `${t('searchResultsFor')} "${searchParams.q}" · `}
                {searchRes.data ? `${searchRes.data.totalCount.toLocaleString()} ${t('results')}` : ''}
              </span>
            </div>
          </div>
          <FilterBar />
        </div>

        {searchRes.error !== null ? (
          <div className="py-10 text-center text-[13px] text-muted">
            {t('loadError')} ({searchRes.error})
          </div>
        ) : searchRes.data.items.length === 0 ? (
          <div className="py-10 text-center text-[13px] text-muted">{t('noResults')}</div>
        ) : (
          <>
            <div className="grid grid-cols-[repeat(auto-fill,minmax(288px,1fr))] gap-3.5">
              {searchRes.data.items.map((repo) => (
                <RepoCard key={repo.fullName} repo={repo} showUpdated={sort === 'updated'} />
              ))}
            </div>
            <Pagination
              page={page}
              perPage={PER_PAGE}
              totalCount={searchRes.data.totalCount}
              params={{
                cat: searchParams.cat,
                sub: searchParams.sub,
                q: searchParams.q,
                lang: searchParams.lang,
                license: searchParams.license,
                sort: searchParams.sort,
              }}
            />
          </>
        )}
      </div>
    </div>
  );
}
