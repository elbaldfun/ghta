import type { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import { getBestOfPage } from '@/lib/data';
import { PageShell } from '@/components/rank/PageShell';
import { PickBlock, RankingRule } from '@/components/rank/Verdict';
import { AppTile } from '@/components/apps/AppTile';
import { alternatesFor } from '@/lib/seo';

// v2c pilot (change 15 review): FIVE hand-picked collections only — prove
// indexing/ranking on a small, dense set before opening more. Expansion is
// gated on 90 days of Search Console data, not on enthusiasm.
const PILOT_SHELVES = [
  'productivity/notes',
  'network/download',
  'system/screenshot',
  'security/passwords',
  'ai/local-llm',
] as const;

export const dynamic = 'force-dynamic';

function shelfOf(params: { major: string; sub: string }): string | null {
  const slug = `${params.major}/${params.sub}`;
  return (PILOT_SHELVES as readonly string[]).includes(slug) ? slug : null;
}

export async function generateMetadata({
  params,
}: {
  params: { locale: string; major: string; sub: string };
}): Promise<Metadata> {
  const shelf = shelfOf(params);
  if (!shelf) return {};
  const t = await getTranslations({ locale: params.locale, namespace: 'rank' });
  const label = t(`shelf_${shelf.replace(/[/-]/g, '_')}` as never) as string;
  return {
    alternates: alternatesFor(params.locale, `/apps/best/${params.major}/${params.sub}`),
    title: t('bestTitle', { label }),
    description: t(`bestIntro_${shelf.replace(/[/-]/g, '_')}` as never) as string,
  };
}

export default async function BestOfPage({
  params,
}: {
  params: { locale: string; major: string; sub: string };
}) {
  const shelf = shelfOf(params);
  if (!shelf) notFound();
  setRequestLocale(params.locale);
  const t = await getTranslations('rank');
  const a = await getTranslations('appsPage');

  const { items, verdict, computedAt } = await getBestOfPage(shelf);
  if (items.length === 0) notFound();

  const key = shelf.replace(/[/-]/g, '_');
  const label = t(`shelf_${key}` as never) as string;
  const majorLabel = t(`shelfM_${params.major}` as never) as string;
  const now = new Date();

  return (
    <PageShell className="flex flex-col gap-6 pb-12 md:gap-8">
      <section className="flex flex-col gap-2 pt-6 md:pt-10">
        <nav aria-label={a('crumbLabel')} className="flex flex-wrap items-center gap-x-1.5 text-sr-caption text-ink-3">
          <Link href="/apps" className="hover:text-ink hover:underline">
            {a('crumbApps')}
          </Link>
          <span aria-hidden="true">›</span>
          <Link href={`/apps?shelf=${params.major}`} className="hover:text-ink hover:underline">
            {majorLabel}
          </Link>
          <span aria-hidden="true">›</span>
          <span aria-current="page" className="text-ink-2">
            {label}
          </span>
        </nav>
        <h1 className="break-words font-news text-[32px] font-medium leading-[1.08] tracking-[-0.02em] [overflow-wrap:anywhere] md:text-[52px]">
          {t('bestTitle', { label })}
        </h1>
        {/* Hand-written intro: first-party framing, not templated filler. */}
        <p className="max-w-[760px] text-sr-body text-ink-2 md:text-sr-lead">{t(`bestIntro_${key}` as never) as string}</p>
      </section>

      {/* The pick and the rule are computed by the boards job; the page only
          words them, so every claim can be checked against the list below. */}
      <div className="flex flex-col gap-3">
        <PickBlock verdict={verdict} scope="shelf" subject={label} apps={items} freshDays={verdict?.freshDays} />
        {computedAt && <RankingRule scope="shelf" computedAt={computedAt} freshDays={verdict?.freshDays} count={items.length} />}
      </div>

      <section aria-label={t('bestTitle', { label })} className="grid grid-cols-1 gap-x-8 gap-y-10 sm:grid-cols-2 lg:grid-cols-3">
        {items.map((app, i) => (
          <AppTile key={app.externalId} app={app} now={now} rank={i + 1} pick={verdict?.pick.externalId === app.externalId} />
        ))}
      </section>

      <footer className="flex flex-wrap gap-x-6 gap-y-2 border-t-2 border-ink pt-4 text-[13px]">
        <Link href={`/apps?shelf=${shelf}`} className="font-semibold text-ink underline underline-offset-4">
          {t('bestSeeShelf', { label })} →
        </Link>
        <Link href="/alternatives" className="font-semibold text-ink underline underline-offset-4">
          {a('altsLink')}
        </Link>
      </footer>
    </PageShell>
  );
}
