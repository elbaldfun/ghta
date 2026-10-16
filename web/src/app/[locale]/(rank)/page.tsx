import type { Metadata } from 'next';
import { permanentRedirect } from 'next/navigation';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import {
  categoryLabel,
  getApps,
  getHeatmap,
  getHot,
  getModels,
  getTotalCount,
  searchRepos,
  type HeatCell,
} from '@/lib/data';
import { listPosts } from '@/lib/blog';
import { formatCompact } from '@/lib/rank-data';
import { boardFilters, boardHref, type BoardQuery } from '@/lib/board-url';
import { alternatesFor } from '@/lib/seo';
import { PageShell } from '@/components/rank/PageShell';

export async function generateMetadata({ params: { locale } }: { params: { locale: string } }): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'today' });
  return { title: t('todayTitle'), description: t('todaySubtitle'), alternates: alternatesFor(locale, '') };
}

const PER_COLUMN = 6;
// Lists, tutorials and interview prep collect stars too, but nobody asks
// "what is rising" to find them; they stay on the full board.
const RESOURCE_TYPES = new Set(['awesome', 'tutorial', 'interview']);

interface Row {
  key: string;
  href: string;
  name: string;
  owner?: string;
  gain: number;
}

/** One source's column: the few entries rising fastest right now. */
function Column({
  title,
  sub,
  href,
  all,
  rows,
  empty,
  dot,
}: {
  title: string;
  sub: string;
  href: string;
  all: string;
  rows: Row[];
  empty: string;
  dot: string;
}) {
  return (
    <section className="flex min-w-0 flex-col rounded-card border border-border bg-surface">
      <header className="flex items-baseline justify-between gap-2 border-b border-border px-4 py-3">
        <h2 className="flex min-w-0 items-center gap-2 text-[13.5px] font-extrabold">
          <span className="h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: dot }} aria-hidden="true" />
          <span className="truncate">{title}</span>
        </h2>
        <Link href={href} className="shrink-0 text-[11.5px] font-bold text-muted hover:text-fg">
          {all} →
        </Link>
      </header>
      <p className="px-4 pt-2 text-[10.5px] font-semibold uppercase tracking-wider text-muted">{sub}</p>
      {rows.length === 0 ? (
        <p className="px-4 py-6 text-[12.5px] text-muted">{empty}</p>
      ) : (
        <ol className="flex flex-col px-2 pb-2 pt-1">
          {rows.map((r, i) => (
            <li key={r.key}>
              <Link
                href={r.href}
                className="grid grid-cols-[18px_minmax(0,1fr)_auto] items-baseline gap-2 rounded-lg px-2 py-[7px] hover:bg-surface2"
              >
                <span className="font-mono text-[11px] font-bold tabular-nums text-muted">{i + 1}</span>
                <span className="truncate text-[13px] font-semibold">
                  {r.owner && <span className="font-medium text-muted">{r.owner}/</span>}
                  {r.name}
                </span>
                <span className="font-mono text-[12px] font-bold tabular-nums text-up">
                  {r.gain > 0 ? `+${formatCompact(r.gain)}` : ''}
                </span>
              </Link>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}

/** Heat ramp for a domain's new stars per repository per day. */
function heatVar(intensity: number): string {
  if (intensity >= 30) return '--heat4';
  if (intensity >= 15) return '--heat3';
  if (intensity >= 8) return '--heat2';
  if (intensity >= 3) return '--heat1';
  return '--heat0';
}

/** Store shelves are three levels deep and live under /apps; domains under /github. */
function heatHref(cell: HeatCell): string {
  const parts = cell.path.split('/');
  if (parts[0] === 'apps' && parts.length === 3) return `/apps/best/${parts[1]}/${parts[2]}`;
  return boardHref({ category: cell.path });
}

export default async function Today({
  params: { locale },
  searchParams,
}: {
  params: { locale: string };
  searchParams: BoardQuery;
}) {
  setRequestLocale(locale);
  // Filtered views used to live here as query parameters; they have their own
  // pages under /github now.
  if (Object.values(searchParams).some(Boolean)) {
    permanentRedirect(`/${locale}${boardHref(boardFilters({}, searchParams))}`);
  }
  const t = await getTranslations('today');
  const tr = await getTranslations('rank');

  const [repos, trending, apps, models, heat, total, posts] = await Promise.all([
    searchRepos({ sort: 'daily', page: 1, perPage: 30 }),
    getHot('daily', PER_COLUMN),
    getApps({ sort: 'hot', limit: PER_COLUMN }),
    getModels({ sort: 'hot', limit: PER_COLUMN }),
    getHeatmap(),
    getTotalCount(),
    listPosts(locale),
  ]);

  const repoRows: Row[] = (repos.data?.items ?? [])
    .filter((r) => !r.type || !RESOURCE_TYPES.has(r.type))
    .slice(0, PER_COLUMN)
    .map((r) => ({
      key: r.fullName,
      href: `/repo/${r.owner}/${r.name}`,
      owner: r.owner,
      name: r.name,
      gain: r.dailyIncrease ?? 0,
    }));
  const trendingRows: Row[] = trending.map(({ repo: r, starsGained }) => ({
    key: r.fullName,
    href: `/repo/${r.owner}/${r.name}`,
    owner: r.owner,
    name: r.name,
    gain: starsGained,
  }));
  const appRows: Row[] = apps.items.slice(0, PER_COLUMN).map((a) => {
    const [owner = '', name = ''] = a.externalId.split('/');
    return { key: a.externalId, href: `/repo/${owner}/${name}`, name: a.name, gain: a.growth };
  });
  const modelRows: Row[] = models.items.map((m) => ({
    key: m.externalId,
    href: m.url,
    owner: m.author,
    name: m.name,
    gain: m.growth,
  }));

  const hottest = heat
    .flatMap((g) => g.children ?? [])
    .filter((c) => c.repos >= 20)
    .sort((a, b) => b.intensity - a.intensity)
    .slice(0, 12);

  return (
    <PageShell className="flex flex-col gap-9 py-8">
      <section className="grid items-end gap-6 lg:grid-cols-[minmax(0,1fr)_260px]">
        <div>
          <h1 className="text-balance font-display text-[30px] font-extrabold leading-tight sm:text-[38px]">
            {t('todayTitle')}
          </h1>
          <p className="mt-3 max-w-[62ch] text-[14px] leading-relaxed text-muted">{t('todaySubtitle')}</p>
        </div>
        <dl className="grid gap-1.5 border-border text-[12.5px] lg:border-l lg:pl-5">
          <div className="flex justify-between gap-4">
            <dt className="text-muted">{t('statTotal')}</dt>
            <dd className="font-mono font-bold tabular-nums">{total ? total.toLocaleString(locale) : '—'}</dd>
          </div>
          <div className="flex justify-between gap-4">
            <dt className="text-muted">{t('statUpdated')}</dt>
            <dd className="font-bold">{t('statUpdatedValue')}</dd>
          </div>
          <div className="mt-1">
            <Link href="/github" className="text-[12.5px] font-bold text-accent hover:underline">
              {t('allTime')} →
            </Link>
          </div>
        </dl>
      </section>

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Column
          title={t('colRepos')}
          sub={t('colReposSub')}
          href={boardHref({ sort: 'daily' })}
          all={t('seeAll')}
          rows={repoRows}
          empty={t('empty')}
          dot="rgb(var(--source-github))"
        />
        <Column
          title={t('colTrending')}
          sub={t('colTrendingSub')}
          href="/breakout"
          all={t('seeAll')}
          rows={trendingRows}
          empty={t('empty')}
          dot="rgb(var(--heat3))"
        />
        <Column
          title={t('colApps')}
          sub={t('colAppsSub')}
          href="/apps"
          all={t('seeAll')}
          rows={appRows}
          empty={t('empty')}
          dot="rgb(var(--accent2))"
        />
        <Column
          title={t('colModels')}
          sub={t('colModelsSub')}
          href="/models"
          all={t('seeAll')}
          rows={modelRows}
          empty={t('empty')}
          dot="rgb(var(--heat2))"
        />
      </div>

      {hottest.length > 0 && (
        <section>
          <div className="mb-3 flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
            <h2 className="font-display text-lg font-extrabold">
              {t('heatTitle')} <span className="text-xs font-semibold text-muted">· {t('heatSub')}</span>
            </h2>
            <Link href="/map" className="text-[12px] font-bold text-muted hover:text-fg">
              {tr('navMap')} →
            </Link>
          </div>
          <ul className="grid grid-cols-2 gap-1.5 sm:grid-cols-4 lg:grid-cols-6">
            {hottest.map((c) => (
              <li key={c.path}>
                <Link
                  href={heatHref(c)}
                  className="flex h-[58px] flex-col justify-end rounded-lg px-2.5 py-2 text-white transition-opacity hover:opacity-90"
                  style={{ backgroundColor: `rgb(var(${heatVar(c.intensity)}))` }}
                >
                  <span className="truncate text-[12px] font-extrabold leading-tight">{categoryLabel(c, locale)}</span>
                  <span className="font-mono text-[10.5px] font-semibold opacity-90">+{c.intensity.toFixed(1)} ★</span>
                </Link>
              </li>
            ))}
          </ul>
        </section>
      )}

      {posts.length > 0 && (
        <section>
          <div className="mb-2 flex items-baseline justify-between gap-4">
            <h2 className="font-display text-lg font-extrabold">{t('insights')}</h2>
            <Link href="/blog" className="text-[12px] font-bold text-muted hover:text-fg">
              {t('seeAll')} →
            </Link>
          </div>
          <ul className="divide-y divide-border border-y border-border">
            {posts.slice(0, 3).map((p) => (
              <li key={p.slug}>
                <Link href={`/blog/${p.slug}`} className="flex items-baseline justify-between gap-4 py-3 hover:text-accent">
                  <span className="min-w-0">
                    <span className="block text-[14px] font-bold">{p.title}</span>
                    <span className="mt-0.5 block truncate text-[12.5px] text-muted">{p.description}</span>
                  </span>
                  <span className="shrink-0 font-mono text-[11.5px] tabular-nums text-muted">{p.date.slice(5, 10)}</span>
                </Link>
              </li>
            ))}
          </ul>
        </section>
      )}
    </PageShell>
  );
}
