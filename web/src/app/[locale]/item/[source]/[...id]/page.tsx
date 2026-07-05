import type { Metadata } from 'next';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { notFound } from 'next/navigation';
import { fetchItemDetail } from '@/lib/api';
import { Card, SourceBadge, GrowthBadge, PageHeader } from '@/components/ui';
import { Sparkline, type Point } from '@/components/Sparkline';
import { JsonLd } from '@/components/JsonLd';
import { formatNumber, primaryValue } from '@/lib/format';

const SITE_URL = process.env.NEXT_PUBLIC_SITE_URL || 'http://localhost:3001';

export async function generateMetadata({
  params,
}: {
  params: { locale: string; source: string; id: string[] };
}): Promise<Metadata> {
  const externalId = params.id.join('/');
  const res = await fetchItemDetail(params.source, externalId);
  if (res.error || !res.data) return { title: externalId };
  const { item } = res.data;
  return {
    title: item.externalId,
    description: item.description || item.externalId,
    alternates: { canonical: `/${params.locale}/item/${params.source}/${externalId}` },
    openGraph: { title: item.externalId, description: item.description, type: 'article' },
  };
}

export default async function ItemPage({
  params,
}: {
  params: { locale: string; source: string; id: string[] };
}) {
  setRequestLocale(params.locale);
  const t = await getTranslations();
  const externalId = params.id.join('/');
  const res = await fetchItemDetail(params.source, externalId);
  if (res.error || !res.data) notFound();
  const { item, history } = res.data;

  const points: Point[] = (history ?? [])
    .map((s) => ({ t: Date.parse(s.capturedAt), v: s.metrics[item.primaryMetric] ?? 0 }))
    .filter((p) => !Number.isNaN(p.t))
    .sort((a, b) => a.t - b.t);

  const jsonLd = {
    '@context': 'https://schema.org',
    '@type': 'SoftwareApplication',
    name: item.name,
    identifier: item.externalId,
    applicationCategory: item.categoryPath || 'DeveloperApplication',
    description: item.description,
    programmingLanguage: item.language,
    aggregateRating: item.metrics.stars
      ? { '@type': 'AggregateRating', ratingValue: 5, reviewCount: Math.round(item.metrics.stars) }
      : undefined,
    url: `${SITE_URL}/${params.locale}/item/${params.source}/${externalId}`,
  };

  const metricEntries = Object.entries(item.metrics);

  return (
    <article>
      <JsonLd data={jsonLd} />
      <div className="mb-2 flex items-center gap-2">
        <SourceBadge source={item.source} />
        {item.language && <span className="text-sm text-muted">{item.language}</span>}
        {item.categoryPath && <span className="text-sm text-muted">· {item.categoryPath}</span>}
      </div>
      <PageHeader title={item.externalId} description={item.description} />

      <div className="mb-6 grid grid-cols-2 gap-3 sm:grid-cols-4">
        {metricEntries.map(([k, v]) => (
          <Card key={k} className="p-4">
            <div className="text-xs uppercase tracking-wide text-muted">{k}</div>
            <div className="mt-1 text-xl font-bold tabular-nums">{formatNumber(v)}</div>
          </Card>
        ))}
        <Card className="p-4">
          <div className="text-xs uppercase tracking-wide text-muted">{t('rising.weekly')} {t('rising.increase')}</div>
          <div className="mt-1 text-xl font-bold">
            <GrowthBadge value={item.weeklyIncrease} />
          </div>
        </Card>
      </div>

      <section aria-labelledby="history-heading" className="mb-6">
        <h2 id="history-heading" className="mb-2 text-lg font-semibold">
          {t('detail.history')}
        </h2>
        <Card className="p-5">
          {points.length >= 2 ? (
            <Sparkline points={points} />
          ) : (
            <p className="py-8 text-center text-muted">{t('detail.noHistory')}</p>
          )}
        </Card>
      </section>

      <div className="text-sm text-muted">
        {t('common.stars')}: {formatNumber(primaryValue(item))}
      </div>
    </article>
  );
}
