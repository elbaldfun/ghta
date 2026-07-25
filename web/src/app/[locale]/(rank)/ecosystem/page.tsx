import type { Metadata } from 'next';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import { getEcosystem, type EcoPillar, type EcoSort } from '@/lib/data';
import { EcosystemCard } from '@/components/rank/EcosystemCard';

export async function generateMetadata({
  params: { locale },
}: {
  params: { locale: string };
}): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'rank' });
  return { title: t('ecoTitle'), description: t('ecoSubtitle') };
}

const PILLARS: EcoPillar[] = ['all', 'skill', 'mcp', 'agent'];

export default async function EcosystemPage({
  params: { locale },
  searchParams,
}: {
  params: { locale: string };
  searchParams: { pillar?: string; sort?: string };
}) {
  setRequestLocale(locale);
  const t = await getTranslations('rank');

  const pillar: EcoPillar = PILLARS.includes(searchParams.pillar as EcoPillar)
    ? (searchParams.pillar as EcoPillar)
    : 'all';
  const sort: EcoSort = searchParams.sort === 'popular' ? 'popular' : 'hot';
  const items = await getEcosystem(pillar, sort, 45);

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
      className={`rounded-lg px-[13px] py-[7px] text-[12.5px] font-bold ${
        pillar === key ? 'bg-accent text-accent-fg' : 'text-muted hover:text-fg'
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
        <div className="grid grid-cols-[repeat(auto-fill,minmax(288px,1fr))] gap-3.5">
          {items.map((item) => (
            <EcosystemCard key={item.externalId} item={item} showGrowth={sort === 'hot'} />
          ))}
        </div>
      )}
    </div>
  );
}
