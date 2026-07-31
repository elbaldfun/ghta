import type { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import { getBestOf } from '@/lib/data';
import { AppCard } from '@/components/rank/AppCard';
import { PageShell } from '@/components/rank/PageShell';

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

  const items = await getBestOf(shelf);
  if (items.length === 0) notFound();

  const key = shelf.replace(/[/-]/g, '_');
  const label = t(`shelf_${key}` as never) as string;

  return (
    <PageShell className="py-[22px]">
      <div className="mb-1 flex items-baseline gap-2.5">
        <h1 className="font-display text-xl font-extrabold">{t('bestTitle', { label })}</h1>
      </div>
      {/* Hand-written intro: the first-party framing Google's thin-content
          policies want to see, not templated filler. */}
      <p className="mb-2 max-w-[75ch] text-[13px] leading-relaxed text-muted">
        {t(`bestIntro_${key}` as never) as string}
      </p>
      <p className="mb-5 text-[11.5px] text-muted">
        {t('bestMethodology', { count: items.length })}
      </p>

      <div className="overflow-hidden rounded-card border border-border bg-surface">
        {items.map((a, i) => (
          <div key={a.externalId} className="relative">
            <span
              className="absolute left-1 top-3.5 hidden w-6 text-right font-mono text-[15px] font-bold tabular-nums text-muted sm:block"
              style={{ color: i < 3 ? 'rgb(var(--accent))' : undefined }}
            >
              {i + 1}
            </span>
            <div className="sm:pl-7">
              <AppCard app={a} />
            </div>
          </div>
        ))}
      </div>

      <div className="mt-5 flex flex-wrap gap-x-5 gap-y-2 text-[12px]">
        <Link href={`/apps?shelf=${shelf}`} className="font-bold text-accent hover:underline">
          {t('bestSeeShelf', { label })} →
        </Link>
        <Link href="/alternatives" className="font-bold text-accent hover:underline">
          {t('navAlternatives')} →
        </Link>
      </div>
    </PageShell>
  );
}
