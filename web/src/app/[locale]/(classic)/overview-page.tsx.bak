import { getTranslations, setRequestLocale } from 'next-intl/server';
import { fetchTrending, fetchRising } from '@/lib/api';
import { ItemCard, EmptyState, ErrorState, PageHeader } from '@/components/ui';
import { itemHref, increaseFor } from '@/lib/format';

export default async function HomePage({ params: { locale } }: { params: { locale: string } }) {
  setRequestLocale(locale);
  const t = await getTranslations();
  const [top, rising] = await Promise.all([
    fetchTrending({ sort: 'stars:desc', limit: 8 }),
    fetchRising('weekly', { limit: 8 }),
  ]);

  return (
    <div>
      <PageHeader title={t('home.title')} description={t('home.subtitle')} />
      <div className="grid gap-8 md:grid-cols-2">
        <section aria-labelledby="top-heading">
          <h2 id="top-heading" className="mb-3 text-lg font-semibold">
            {t('home.topByStars')}
          </h2>
          <div className="space-y-2">
            {top.error ? (
              <ErrorState message={t('common.error')} />
            ) : !top.data || top.data.length === 0 ? (
              <EmptyState message={t('common.empty')} />
            ) : (
              top.data.map((item, i) => (
                <ItemCard key={item.id} item={item} href={itemHref(locale, item)} rank={i + 1} />
              ))
            )}
          </div>
        </section>

        <section aria-labelledby="rising-heading">
          <h2 id="rising-heading" className="mb-3 text-lg font-semibold">
            {t('home.fastestRising')}
          </h2>
          <div className="space-y-2">
            {rising.error ? (
              <ErrorState message={t('common.error')} />
            ) : !rising.data || rising.data.length === 0 ? (
              <EmptyState message={t('common.empty')} />
            ) : (
              rising.data.map((item, i) => (
                <ItemCard
                  key={item.id}
                  item={item}
                  href={itemHref(locale, item)}
                  rank={i + 1}
                  growth={increaseFor(item, 'weekly')}
                />
              ))
            )}
          </div>
        </section>
      </div>
    </div>
  );
}
