import type { Metadata } from 'next';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import { getEcosystem, getHeatmap, getTrends } from '@/lib/data';
import { formatCompact } from '@/lib/rank-data';
import { EcosystemMap } from '@/components/rank/EcosystemMap';
import { RankRow } from '@/components/rank/RankRow';

// Render per request rather than at build time: the map's data comes from the
// backend, and a build that can't reach it (e.g. a preview deploy without
// API_URL) would otherwise bake an empty page and serve it until revalidation.
export const dynamic = 'force-dynamic';

export async function generateMetadata({
  params: { locale },
}: {
  params: { locale: string };
}): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'rank' });
  return { title: t('mapTitle'), description: t('mapSubtitle') };
}

export default async function MapPage({
  params: { locale },
  searchParams,
}: {
  params: { locale: string };
  searchParams: { sort?: string };
}) {
  setRequestLocale(locale);
  const t = await getTranslations('rank');

  const sort = searchParams.sort === 'stars' || searchParams.sort === 'rel' ? searchParams.sort : 'velocity';
  const [cells, eco] = await Promise.all([getHeatmap(), getEcosystem('all', 'hot', 60)]);

  const rel = (x: { growth: number; stars: number }) => (x.stars > 0 ? x.growth / x.stars : 0);
  const items = [...eco.items]
    .sort((a, b) =>
      sort === 'stars' ? b.stars - a.stars : sort === 'rel' ? rel(b) - rel(a) : b.growth - a.growth,
    )
    .slice(0, 20);
  const trends = await getTrends(items.map((i) => i.externalId), 30);

  const totalRepos = cells.reduce((s, c) => s + c.repos, 0);
  const totalGrowth = cells.reduce((s, c) => s + c.growth, 0);

  return (
    <div className="px-[26px] py-[22px]">
      <div className="mb-1 flex flex-wrap items-baseline gap-x-3 gap-y-1">
        <h1 className="font-display text-xl font-extrabold">{t('mapTitle')}</h1>
        <span className="font-mono text-[11.5px] tabular-nums text-muted">
          {formatCompact(totalRepos)} · +{formatCompact(totalGrowth)} ★/{t('mapPerDay')}
        </span>
      </div>
      <p className="mb-6 max-w-[68ch] text-[13px] text-muted">{t('mapSubtitle')}</p>

      <EcosystemMap cells={cells} />

      {items.length > 0 && (
        <section className="mt-10">
          <div className="mb-3 flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
            <div className="flex items-baseline gap-2.5">
              <h2 className="font-display text-lg font-extrabold">{t('mapBoardTitle')}</h2>
              <span className="text-xs text-muted">{t('mapBoardSubtitle')}</span>
            </div>
            <div className="flex items-center gap-1">
              {([['velocity', t('mapSortVelocity')], ['rel', t('mapSortRelative')], ['stars', t('mapSortStars')]] as const).map(
                ([key, txt]) => (
                  <Link
                    key={key}
                    href={`/map?sort=${key}`}
                    className={`rounded-md px-2.5 py-1 text-[11.5px] font-semibold ${
                      sort === key ? 'text-accent' : 'text-muted hover:text-fg'
                    }`}
                  >
                    {txt}
                  </Link>
                ),
              )}
            </div>
          </div>

          <div className="overflow-hidden rounded-card border border-border bg-surface">
            <div className="grid grid-cols-[40px_1fr_92px_100px_84px] items-center gap-3.5 border-b border-border px-4 py-2 font-mono text-[10px] uppercase tracking-[0.08em] text-muted">
              <div>#</div>
              <div>{t('mapColRepo')}</div>
              <div>{t('mapColTrend')}</div>
              <div className="text-right">{t('mapColVelocity')}</div>
              <div className="text-right">{t('mapColStars')}</div>
            </div>
            {items.map((item, i) => (
              <RankRow key={item.externalId} item={item} rank={i + 1} series={trends[item.externalId]} />
            ))}
          </div>
        </section>
      )}
    </div>
  );
}
