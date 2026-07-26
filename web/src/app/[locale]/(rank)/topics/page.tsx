import type { Metadata } from 'next';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import { getTopics, type TopicSort } from '@/lib/data';
import { formatCompact } from '@/lib/rank-data';

export async function generateMetadata({
  params: { locale },
}: {
  params: { locale: string };
}): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'rank' });
  return { title: t('topicsTitle'), description: t('topicsSubtitle') };
}

export default async function TopicsPage({
  params: { locale },
  searchParams,
}: {
  params: { locale: string };
  searchParams: { sort?: string };
}) {
  setRequestLocale(locale);
  const t = await getTranslations('rank');

  const sort: TopicSort = searchParams.sort === 'popular' ? 'popular' : 'hot';
  const topics = await getTopics('ai', sort, 40);

  const tab = (key: TopicSort, label: string) => (
    <Link
      href={`/topics?sort=${key}`}
      className={`rounded-lg border px-[13px] py-[7px] text-[12.5px] font-bold ${
        sort === key
          ? 'border-accent bg-accent/10 text-accent'
          : 'border-transparent text-muted hover:text-fg'
      }`}
    >
      {label}
    </Link>
  );

  return (
    <div className="px-[26px] py-[22px]">
      <div className="mb-1 flex items-baseline gap-2.5">
        <h1 className="font-display text-lg font-extrabold">{t('topicsTitle')}</h1>
        <span className="text-xs text-muted">{t('topicsSubtitle')}</span>
      </div>
      <div className="mb-[18px] mt-3 flex items-center gap-1">
        {tab('hot', t('topicsSortHot'))}
        {tab('popular', t('topicsSortPopular'))}
      </div>

      {topics.length === 0 ? (
        <div className="py-10 text-center text-[13px] text-muted">{t('loadError')}</div>
      ) : (
        <div className="grid gap-2.5 sm:grid-cols-2 lg:grid-cols-3">
          {topics.map((tp, i) => {
            const pct = tp.totalStars > 0 ? (tp.growth / tp.totalStars) * 100 : 0;
            const heat =
              pct >= 4
                ? 'rgb(var(--heat4))'
                : pct >= 2
                  ? 'rgb(var(--heat3))'
                  : pct >= 0.8
                    ? 'rgb(var(--heat2))'
                    : pct >= 0.2
                      ? 'rgb(var(--heat1))'
                      : 'rgb(var(--heat0))';
            return (
              <Link
                key={tp.topic}
                href={`/?q=${encodeURIComponent(tp.topic)}`}
                className="flex items-center justify-between gap-3 rounded-card border border-border bg-surface px-4 py-3 transition-colors hover:border-accent"
              >
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <span
                      className="w-5 shrink-0 text-right font-mono text-[11px] font-bold tabular-nums"
                      style={{ color: i < 3 ? heat : 'rgb(var(--muted))' }}
                    >
                      {i + 1}
                    </span>
                    <span className="truncate text-sm font-bold">{tp.topic}</span>
                  </div>
                  <div className="mt-0.5 pl-7 font-mono text-[11px] tabular-nums text-muted">
                    {tp.repoCount} {t('devRepos')} · {formatCompact(tp.totalStars)}★
                  </div>
                </div>
                {tp.growth > 0 && (
                  <span
                    className="shrink-0 font-mono text-[12px] font-bold tabular-nums"
                    style={{ color: sort === 'hot' ? heat : 'rgb(var(--muted))' }}
                  >
                    +{formatCompact(tp.growth)} {t('devPerDay')}
                  </span>
                )}
              </Link>
            );
          })}
        </div>
      )}
    </div>
  );
}
