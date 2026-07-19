import type { Metadata } from 'next';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { fetchTrending } from '@/lib/api';
import { ItemCard, EmptyState, ErrorState, PageHeader } from '@/components/ui';
import { itemHref } from '@/lib/format';

export async function generateMetadata({
  params: { locale },
}: {
  params: { locale: string };
}): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'trending' });
  return { title: t('title'), description: t('description') };
}

const SOURCES = ['', 'github', 'appstore', 'chrome', 'msstore'];
const SORTS = ['stars:desc', 'stars:asc', 'forks:desc', 'fetchedAt:desc'];

export default async function TrendingPage({
  params: { locale },
  searchParams,
}: {
  params: { locale: string };
  searchParams: { source?: string; language?: string; sort?: string };
}) {
  setRequestLocale(locale);
  const t = await getTranslations();
  const source = searchParams.source ?? '';
  const sort = searchParams.sort ?? 'stars:desc';
  const language = searchParams.language ?? '';

  const res = await fetchTrending({ source, language, sort, limit: 50 });

  return (
    <div>
      <PageHeader title={t('trending.title')} description={t('trending.description')} />

      <form className="mb-6 flex flex-wrap gap-3" method="get">
        <label className="text-sm">
          <span className="sr-only">{t('common.source')}</span>
          <select name="source" defaultValue={source} className="rounded-md border border-border bg-surface px-2 py-1">
            {SOURCES.map((s) => (
              <option key={s} value={s}>
                {s === '' ? t('common.allSources') : s}
              </option>
            ))}
          </select>
        </label>
        <input
          name="language"
          defaultValue={language}
          placeholder={t('common.language')}
          className="rounded-md border border-border bg-surface px-2 py-1"
        />
        <label className="text-sm">
          <span className="sr-only">{t('common.sortBy')}</span>
          <select name="sort" defaultValue={sort} className="rounded-md border border-border bg-surface px-2 py-1">
            {SORTS.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
        </label>
        <button className="rounded-md bg-accent px-3 py-1 text-sm font-medium text-accent-fg">
          {t('common.sortBy')}
        </button>
      </form>

      <div className="space-y-2">
        {res.error ? (
          <ErrorState message={t('common.error')} />
        ) : !res.data || res.data.length === 0 ? (
          <EmptyState message={t('common.empty')} />
        ) : (
          res.data.map((item, i) => (
            <ItemCard key={item.id} item={item} href={itemHref(locale, item)} rank={i + 1} />
          ))
        )}
      </div>
    </div>
  );
}
