import type { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { PageShell } from '@/components/rank/PageShell';
import { Dropdown, Segmented } from '@/components/ui';
import { ChartRow } from '@/components/ios/ChartRow';
import { RememberCountry } from '@/components/ios/RememberCountry';
import {
  COUNTRY_COOKIE,
  IOS_CHARTS,
  IOS_COUNTRIES,
  IOS_GENRES,
  chartPath,
  countryName,
  getChart,
  isChart,
  isCountry,
  isGenre,
  newPath,
  type IOSChart,
  type IOSCountry,
} from '@/lib/api/ios';
import { alternatesFor } from '@/lib/seo';

// One static page per storefront × chart × genre (390), refreshed hourly:
// the charts change once a day, at 09:00 UTC.
export const revalidate = 3600;

type Params = { locale: string; slug: string; chart: string; genre?: string[] };

/** The chart a route names, or null for one that does not exist. */
function chartOf(p: Params): { country: IOSCountry; chart: IOSChart; genre: string } | null {
  const genre = p.genre?.length === 1 ? p.genre[0] : p.genre ? null : 'all';
  if (!isCountry(p.slug) || !isChart(p.chart) || genre === null || (genre !== 'all' && !isGenre(genre))) return null;
  return { country: p.slug, chart: p.chart, genre };
}

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const c = chartOf(params);
  if (!c) return {};
  const t = await getTranslations({ locale: params.locale, namespace: 'ios' });
  const vars = {
    country: countryName(c.country, params.locale),
    chart: t(`chart_${c.chart}`),
    genre: c.genre === 'all' ? '' : t(`g_${c.genre}` as never),
  };
  const all = c.genre === 'all';
  return {
    title: t(all ? 'pageTitleAll' : 'pageTitle', vars),
    description: t(all ? 'pageDescriptionAll' : 'pageDescription', vars),
    alternates: alternatesFor(params.locale, chartPath(c.country, c.chart, c.genre)),
  };
}

export default async function IOSChartPage({ params }: { params: Params }) {
  const c = chartOf(params);
  if (!c) notFound();
  const { locale } = params;
  setRequestLocale(locale);
  const t = await getTranslations('ios');
  const res = await getChart(c.country, c.chart, c.genre);
  const page = res.data;
  const genreName = (g: string) => (g === 'all' ? t('genreAll') : t(`g_${g}` as never));
  const chartName = t(`chart_${c.chart}`);
  const country = countryName(c.country, locale);
  const heading = c.genre === 'all' ? chartName : `${genreName(c.genre)} · ${chartName}`;

  return (
    <PageShell className="flex flex-col gap-6 pb-12 md:gap-8">
      <RememberCountry name={COUNTRY_COOKIE} country={c.country} />
      <section className="flex flex-col gap-2 pt-6 md:pt-10">
        <p className="text-sr-caption text-ink-3">
          {t('kicker')} · {country}
        </p>
        <div className="flex flex-wrap items-end justify-between gap-x-4 gap-y-2">
          <h1 className="font-news text-[32px] font-medium leading-[1.05] tracking-[-0.02em] md:text-[48px]">{heading}</h1>
          {page && <span className="text-sr-small text-ink-3">{t('updated', { day: page.day })}</span>}
        </div>
        <p className="max-w-[760px] text-sr-body text-ink-2">{t('lead')}</p>
      </section>

      <section className="flex flex-col gap-3 border-y border-rule py-3 lg:flex-row lg:items-center lg:justify-between">
        <Segmented
          label={t('countryLabel')}
          items={IOS_COUNTRIES.map((k) => ({
            key: k,
            label: countryName(k, locale),
            href: chartPath(k, c.chart, c.genre),
            active: k === c.country,
          }))}
        />
        <div className="flex flex-wrap items-center gap-2">
          <Segmented
            label={t('chartLabel')}
            items={[
              ...IOS_CHARTS.map((k) => ({
                key: k,
                label: t(`chart_${k}`),
                href: chartPath(c.country, k, c.genre),
                active: k === c.chart,
              })),
              // StarRank's own view, one per storefront: it drops the genre.
              { key: 'new', label: t('chart_new'), href: newPath(c.country) },
            ]}
          />
          <Dropdown
            label={genreName(c.genre)}
            active={c.genre !== 'all'}
            align="right"
            items={['all', ...IOS_GENRES].map((g) => ({
              key: g,
              label: genreName(g),
              href: chartPath(c.country, c.chart, g),
              active: g === c.genre,
            }))}
          />
        </div>
      </section>

      {!page ? (
        <p className="py-10 text-center text-sr-body text-ink-3">{res.error === 'backend HTTP 404' ? t('empty') : t('loadError')}</p>
      ) : (
        <section className="flex flex-col gap-2">
          <p className="text-sr-caption text-ink-3">
            {page.prevDay ? t('movesSince', { day: page.prevDay }) : t('noBaseline')}
          </p>
          {page.entries.length === 0 ? (
            <p className="py-10 text-center text-sr-body text-ink-3">{t('empty')}</p>
          ) : (
            <ol className="border-t border-ink">
              {page.entries.map((e) => (
                <ChartRow
                  key={e.app.id}
                  entry={e}
                  showGenre={c.genre === 'all'}
                  moveScope={t('moveScope', { country, chart: heading })}
                />
              ))}
            </ol>
          )}
        </section>
      )}
    </PageShell>
  );
}
