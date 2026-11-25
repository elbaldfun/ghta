import type { Metadata } from 'next';
import { notFound, redirect } from 'next/navigation';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import { PageShell } from '@/components/rank/PageShell';
import { RankMove } from '@/components/rank/RankMove';
import { GrowthChart } from '@/components/rank/GrowthChart';
import { StatBlock, fmtInt, srButton } from '@/components/ui';
import {
  IOS_CHARTS,
  IOS_COUNTRIES,
  appPath,
  appStoreUrl,
  chartPath,
  countryName,
  formatPrice,
  getIOSApp,
  getIOSHistory,
  iconAt,
  isCountry,
  type ChartPlace,
} from '@/lib/api/ios';
import { alternatesFor } from '@/lib/seo';

export const revalidate = 3600;

type Params = { locale: string; slug: string };

/** "id6448311069" → "6448311069"; null for anything else. */
function trackIdOf(slug: string): string | null {
  const m = /^id(\d{1,15})$/.exec(slug);
  return m ? m[1] : null;
}

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const id = trackIdOf(params.slug);
  if (!id) return {};
  const res = await getIOSApp(id);
  if (res.error !== null) return {};
  const a = res.data;
  const t = await getTranslations({ locale: params.locale, namespace: 'ios' });
  return {
    title: `${a.name} · App Store`,
    description: t('detailDescription', {
      name: a.name,
      developer: a.developer ?? '',
      rating: a.ratings > 0 ? a.rating.toFixed(1) : '—',
      country: countryName(a.homeCountry ?? 'us', params.locale),
    }),
    alternates: alternatesFor(params.locale, appPath(id)),
    // Off every chart for a week, or never rated: a frozen, thin listing.
    robots: a.listed && a.ratings > 0 ? undefined : { index: false, follow: true },
  };
}

export default async function IOSSlugPage({ params }: { params: Params }) {
  const { locale, slug } = params;
  // /ios/us opens that storefront's free chart.
  if (isCountry(slug)) redirect(`/${locale}${chartPath(slug, 'free')}`);
  const id = trackIdOf(slug);
  if (!id) notFound();
  setRequestLocale(locale);

  const [res, history, t] = await Promise.all([getIOSApp(id), getIOSHistory(id), getTranslations('ios')]);
  if (res.error !== null) notFound();
  const a = res.data;
  const home = a.homeCountry ?? 'us';
  const homeName = countryName(home, locale);
  const genreName = (g: string) => (g === 'all' ? t('overall') : t(`g_${g}` as never));
  const icon = iconAt(a.iconUrl, 256);
  const day = (iso?: string) => (iso ? iso.slice(0, 10) : '—');

  // country → chart → its places, overall first.
  const places = new Map<string, ChartPlace[]>();
  for (const p of a.charts) {
    const k = `${p.country}|${p.chart}`;
    places.set(k, [...(places.get(k) ?? []), p].sort((x, y) => Number(y.genre === 'all') - Number(x.genre === 'all')));
  }

  return (
    <PageShell className="flex flex-col gap-8 pb-12 pt-6 md:pt-10">
      <section className="flex flex-col gap-4 sm:flex-row sm:items-start sm:gap-6">
        {icon ? (
          <img src={icon} alt="" width={112} height={112} className="h-24 w-24 shrink-0 rounded-[22px] border border-rule sm:h-28 sm:w-28" />
        ) : null}
        <div className="flex min-w-0 flex-col gap-2">
          <p className="text-sr-caption text-ink-3">
            <Link href={chartPath(home, 'free')} className="hover:text-ink">
              {t('kicker')}
            </Link>
            {a.genre ? (
              <>
                {' · '}
                <Link href={chartPath(home, 'free', a.genre)} className="hover:text-ink">
                  {genreName(a.genre)}
                </Link>
              </>
            ) : null}
          </p>
          <h1 className="font-news text-[30px] font-medium leading-[1.1] tracking-[-0.02em] md:text-[44px]">{a.name}</h1>
          <p className="text-sr-body text-ink-2">
            {a.developer}
            {' · '}
            {formatPrice(a.price, a.currency, locale, t('free'))}
            {a.contentRating ? ` · ${a.contentRating}` : ''}
          </p>
          <div>
            <a href={appStoreUrl(a.id, home)} target="_blank" rel="noopener noreferrer" className={srButton('solid')}>
              {t('openInStore')} ↗
            </a>
          </div>
        </div>
      </section>

      <section className="grid grid-cols-2 gap-x-6 gap-y-5 border-y border-rule py-5 md:grid-cols-4">
        <StatBlock
          label={t('ratingIn', { country: homeName })}
          value={a.ratings > 0 ? `★ ${a.rating.toFixed(1)}` : '—'}
          sub={a.ratings > 0 ? t('ratingsCount', { n: fmtInt(a.ratings) }) : t('noRatings')}
        />
        <StatBlock
          label={t('ratings7d')}
          value={a.ratingsD7 === null ? '—' : `${a.ratingsD7 >= 0 ? '+' : ''}${fmtInt(a.ratingsD7)}`}
          sub={a.ratingsD7 === null ? t('pending') : homeName}
        />
        <StatBlock label={t('version')} value={a.version ?? '—'} sub={`${t('updatedAt')} ${day(a.updatedAt)}`} />
        <StatBlock label={t('releasedAt')} value={day(a.releasedAt)} />
      </section>

      <section className="flex flex-col gap-3">
        <h2 className="text-[18px] font-semibold">{t('chartsTitle')}</h2>
        {a.charts.length > 0 ? (
          <>
            <p className="text-sr-caption text-ink-3">{t('chartsHint', { n: a.charts.length, day: a.chartsDay ?? '' })}</p>
            <div className="overflow-x-auto">
              <table className="w-full min-w-[520px] border-t border-ink text-left text-[13px]">
                <thead>
                  <tr className="border-b border-rule text-sr-caption text-ink-3">
                    <th className="py-2 pr-3 font-normal">{t('countryLabel')}</th>
                    {IOS_CHARTS.map((k) => (
                      <th key={k} className="py-2 pr-3 font-normal">
                        {t(`chart_${k}`)}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {IOS_COUNTRIES.map((cty) => (
                    <tr key={cty} className="border-b border-rule align-top">
                      <th scope="row" className="py-2 pr-3 font-medium">
                        {countryName(cty, locale)}
                      </th>
                      {IOS_CHARTS.map((k) => {
                        const ps = places.get(`${cty}|${k}`) ?? [];
                        return (
                          <td key={k} className="py-2 pr-3">
                            {ps.length === 0 ? (
                              <span className="text-ink-3">—</span>
                            ) : (
                              <ul className="flex flex-col gap-1">
                                {ps.map((p) => (
                                  <li key={p.genre} className="flex items-center gap-1.5">
                                    <Link href={chartPath(cty, k, p.genre)} className="tabular-nums hover:underline">
                                      {genreName(p.genre)} #{p.rank}
                                    </Link>
                                    <RankMove
                                      change={p.change}
                                      isNew={p.new}
                                      scope={`${countryName(cty, locale)} ${t(`chart_${k}`)} ${genreName(p.genre)}`}
                                    />
                                  </li>
                                ))}
                              </ul>
                            )}
                          </td>
                        );
                      })}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        ) : a.listed && a.countries.length > 0 ? (
          // Positions not written yet (before batch 0): at least the storefronts.
          <p className="flex flex-wrap gap-2 text-[13px]">
            {a.countries.map((cty) => (
              <Link key={cty} href={chartPath(cty, 'free')} className="rounded border border-rule px-2.5 py-1 hover:border-ink-3">
                {countryName(cty, locale)}
              </Link>
            ))}
          </p>
        ) : (
          <p className="text-sr-body text-ink-3">{t('chartsNone')}</p>
        )}
      </section>

      <section className="flex flex-col gap-3">
        <h2 className="text-[18px] font-semibold">{t('curveTitle', { country: homeName })}</h2>
        {history.ratings.length >= 2 ? (
          <GrowthChart points={history.ratings} tone="sr" />
        ) : (
          <p className="text-sr-body text-ink-3">{t('curveNone')}</p>
        )}
      </section>

      {a.screenshots.length > 0 && (
        <section className="flex flex-col gap-3">
          <h2 className="text-[18px] font-semibold">{t('screenshots')}</h2>
          <div className="flex gap-3 overflow-x-auto pb-2">
            {a.screenshots.map((src) => (
              <img
                key={src}
                src={src}
                alt=""
                loading="lazy"
                className="h-[360px] w-auto shrink-0 rounded-[12px] border border-rule md:h-[420px]"
              />
            ))}
          </div>
        </section>
      )}

      {a.description && (
        <section className="flex max-w-[760px] flex-col gap-3">
          <h2 className="text-[18px] font-semibold">{t('description')}</h2>
          <p className="whitespace-pre-line text-sr-body text-ink-2">{a.description}</p>
        </section>
      )}
    </PageShell>
  );
}
