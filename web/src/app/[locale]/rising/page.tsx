import type { Metadata } from 'next';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { fetchRising, type RisingWindow } from '@/lib/api';
import { Link } from '@/i18n/navigation';
import { ItemCard, EmptyState, ErrorState, PageHeader } from '@/components/ui';
import { itemHref, increaseFor } from '@/lib/format';

export async function generateMetadata({
  params: { locale },
}: {
  params: { locale: string };
}): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'rising' });
  return { title: t('title'), description: t('description') };
}

const WINDOWS: RisingWindow[] = ['daily', 'weekly', 'monthly'];

export default async function RisingPage({
  params: { locale },
  searchParams,
}: {
  params: { locale: string };
  searchParams: { window?: string };
}) {
  setRequestLocale(locale);
  const t = await getTranslations();
  const window = (WINDOWS.includes(searchParams.window as RisingWindow)
    ? searchParams.window
    : 'weekly') as RisingWindow;

  const res = await fetchRising(window, { limit: 50 });

  return (
    <div>
      <PageHeader title={t('rising.title')} description={t('rising.description')} />

      <div role="tablist" className="mb-6 inline-flex rounded-md border border-border p-1">
        {WINDOWS.map((w) => (
          <Link
            key={w}
            href={`/rising?window=${w}`}
            role="tab"
            aria-selected={w === window}
            className={`rounded px-3 py-1 text-sm ${w === window ? 'bg-accent text-accent-fg' : 'text-muted hover:text-fg'}`}
          >
            {t(`rising.${w}`)}
          </Link>
        ))}
      </div>

      <div className="space-y-2">
        {res.error ? (
          <ErrorState message={t('common.error')} />
        ) : !res.data || res.data.length === 0 ? (
          <EmptyState message={t('common.empty')} />
        ) : (
          res.data.map((item, i) => (
            <ItemCard
              key={item.id}
              item={item}
              href={itemHref(locale, item)}
              rank={i + 1}
              growth={increaseFor(item, window)}
            />
          ))
        )}
      </div>
    </div>
  );
}
