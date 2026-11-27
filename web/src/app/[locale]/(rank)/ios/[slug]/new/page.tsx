import type { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { PageShell } from '@/components/rank/PageShell';
import { Segmented } from '@/components/ui';
import { NewRow } from '@/components/ios/NewRow';
import { RememberCountry } from '@/components/ios/RememberCountry';
import { COUNTRY_COOKIE, IOS_CHARTS, IOS_COUNTRIES, chartPath, countryName, getNew, isCountry, newPath } from '@/lib/api/ios';
import { alternatesFor } from '@/lib/seo';

// One page per storefront, refreshed hourly: the view is rebuilt once a day,
// right after the charts are read at 09:00 UTC.
export const revalidate = 3600;

type Params = { locale: string; slug: string };

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  if (!isCountry(params.slug)) return {};
  const t = await getTranslations({ locale: params.locale, namespace: 'ios' });
  const country = countryName(params.slug, params.locale);
  return {
    title: t('newPageTitle', { country }),
    description: t('newPageDescription', { country }),
    alternates: alternatesFor(params.locale, newPath(params.slug)),
  };
}

export default async function IOSNewPage({ params }: { params: Params }) {
  const { locale, slug } = params;
  if (!isCountry(slug)) notFound();
  setRequestLocale(locale);
  const t = await getTranslations('ios');
  const res = await getNew(slug);
  const page = res.data;
  const country = countryName(slug, locale);

  return (
    <PageShell className="flex flex-col gap-6 pb-12 md:gap-8">
      <RememberCountry name={COUNTRY_COOKIE} country={slug} />
      <section className="flex flex-col gap-2 pt-6 md:pt-10">
        <p className="text-sr-caption text-ink-3">
          {t('kicker')} · {country}
        </p>
        <div className="flex flex-wrap items-end justify-between gap-x-4 gap-y-2">
          <h1 className="font-news text-[32px] font-medium leading-[1.05] tracking-[-0.02em] md:text-[48px]">{t('chart_new')}</h1>
          {page && <span className="text-sr-small text-ink-3">{t('updated', { day: page.day })}</span>}
        </div>
        {/* The view is StarRank's, not Apple's: say so before the list. */}
        <p className="max-w-[760px] text-sr-body text-ink-2">{t('newLead', { days: page?.windowDays ?? 90 })}</p>
      </section>

      <section className="flex flex-col gap-3 border-y border-rule py-3 lg:flex-row lg:items-center lg:justify-between">
        <Segmented
          label={t('countryLabel')}
          items={IOS_COUNTRIES.map((k) => ({ key: k, label: countryName(k, locale), href: newPath(k), active: k === slug }))}
        />
        <Segmented
          label={t('chartLabel')}
          items={[
            ...IOS_CHARTS.map((k) => ({ key: k, label: t(`chart_${k}`), href: chartPath(slug, k) })),
            { key: 'new', label: t('chart_new'), href: newPath(slug), active: true },
          ]}
        />
      </section>

      {!page || page.entries.length === 0 ? (
        // A 404 is a storefront the ios-new job has not built yet.
        <p className="py-10 text-center text-sr-body text-ink-3">
          {page || res.error === 'backend HTTP 404' ? t('newEmpty') : t('loadError')}
        </p>
      ) : (
        <section className="flex flex-col gap-2">
          <p className="text-sr-caption text-ink-3">{t('newCount', { n: page.total, days: page.windowDays })}</p>
          <ol className="border-t border-ink">
            {page.entries.map((e) => (
              <NewRow key={e.app.id} entry={e} />
            ))}
          </ol>
        </section>
      )}
    </PageShell>
  );
}
