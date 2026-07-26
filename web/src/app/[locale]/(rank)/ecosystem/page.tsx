import type { Metadata } from 'next';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import { getEcosystem, getTrends, type EcoPillar, type EcoSort } from '@/lib/data';
import { RankRow } from '@/components/rank/RankRow';
import { Pagination } from '@/components/rank/Pagination';

export async function generateMetadata({
  params: { locale },
}: {
  params: { locale: string };
}): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'rank' });
  return { title: t('ecoTitle'), description: t('ecoSubtitle') };
}

const PILLARS: EcoPillar[] = ['all', 'skill', 'mcp', 'agent'];
// The backend computes a top-300 board per pillar+sort; page within that.
const PER_PAGE = 30;
const BOARD_CAP = 300;

export default async function EcosystemPage({
  params: { locale },
  searchParams,
}: {
  params: { locale: string };
  searchParams: { pillar?: string; sort?: string; page?: string };
}) {
  setRequestLocale(locale);
  const t = await getTranslations('rank');

  const pillar: EcoPillar = PILLARS.includes(searchParams.pillar as EcoPillar)
    ? (searchParams.pillar as EcoPillar)
    : 'all';
  const sort: EcoSort = searchParams.sort === 'popular' ? 'popular' : 'hot';
  const page = Math.max(1, Number(searchParams.page) || 1);
  const { items, total } = await getEcosystem(pillar, sort, PER_PAGE, page);
  const trends = await getTrends(items.map((i) => i.externalId), 30);

  const pillarLabel: Record<EcoPillar, string> = {
    all: t('ecoAll'),
    skill: t('ecoSkill'),
    mcp: 'MCP',
    agent: t('ecoAgent'),
  };

  const pillarTab = (key: EcoPillar) => (
    <Link
      key={key}
      href={`/ecosystem?pillar=${key}&sort=${sort}`}
      className={`rounded-lg border px-[13px] py-[7px] text-[12.5px] font-bold ${
        pillar === key
          ? 'border-accent bg-accent/10 text-accent'
          : 'border-transparent text-muted hover:text-fg'
      }`}
    >
      {pillarLabel[key]}
    </Link>
  );

  const sortTab = (key: EcoSort, label: string) => (
    <Link
      href={`/ecosystem?pillar=${pillar}&sort=${key}`}
      className={`rounded-md px-2.5 py-1 text-[11.5px] font-semibold ${
        sort === key ? 'text-accent' : 'text-muted hover:text-fg'
      }`}
    >
      {label}
    </Link>
  );

  return (
    <div className="px-[26px] py-[22px]">
      <div className="mb-1 flex items-baseline gap-2.5">
        <h1 className="font-display text-lg font-extrabold">{t('ecoTitle')}</h1>
        <span className="text-xs text-muted">{t('ecoSubtitle')}</span>
      </div>
      <div className="mb-[18px] mt-3 flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-1">{PILLARS.map(pillarTab)}</div>
        <div className="flex items-center gap-1">
          {sortTab('hot', t('ecoSortHot'))}
          {sortTab('popular', t('ecoSortPopular'))}
        </div>
      </div>

      {items.length === 0 ? (
        <div className="py-10 text-center text-[13px] text-muted">{t('loadError')}</div>
      ) : (
        <div className="overflow-hidden rounded-card border border-border bg-surface">
          <div className="grid grid-cols-[36px_1fr_86px] items-center gap-2.5 border-b border-border px-3 py-2 font-mono text-[10px] uppercase tracking-[0.08em] text-muted sm:gap-3.5 sm:px-4 md:grid-cols-[40px_1fr_92px_100px_84px]">
            <div>#</div>
            <div>{t('mapColRepo')}</div>
            <div className="hidden md:block">{t('mapColTrend')}</div>
            <div className="text-right">{t('mapColVelocity')}</div>
            <div className="hidden text-right md:block">{t('mapColStars')}</div>
          </div>
          {items.map((item, i) => (
            <RankRow
              key={item.externalId}
              item={item}
              rank={(page - 1) * PER_PAGE + i + 1}
              series={trends[item.externalId]}
            />
          ))}
        </div>
      )}
      {items.length > 0 && (
        <Pagination
          page={page}
          perPage={PER_PAGE}
          totalCount={total}
          basePath="/ecosystem"
          cap={BOARD_CAP}
          params={{ pillar, sort }}
        />
      )}
    </div>
  );
}
