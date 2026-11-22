import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import { permanentRedirect } from 'next/navigation';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import {
  categoryLabel,
  getApps,
  getHeatmap,
  getModels,
  getStaleness,
  getTrends,
  searchRepos,
  type HeatCell,
  type RepoSummary,
} from '@/lib/data';
import { listPosts } from '@/lib/blog';
import { boardFilters, boardHref, type BoardQuery } from '@/lib/board-url';
import { alternatesFor } from '@/lib/seo';
import { dailySeries, fmtParams, isNewEntry, pickCool, pickFresh, pickLead } from '@/lib/front-page';
import { PageShell } from '@/components/rank/PageShell';
import { DailyBars, Delta, Kicker, SectionHeader, StatBlock, Tag, fmtInt, fmtTotal, HOT_DAILY } from '@/components/ui';

export async function generateMetadata({ params: { locale } }: { params: { locale: string } }): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'today' });
  return { title: t('todayTitle'), description: t('todaySubtitle'), alternates: alternatesFor(locale, '') };
}

// Lists, tutorials and interview prep collect stars too, but nobody asks
// "what is rising" to find them; they stay on the full board.
const RESOURCE_TYPES = new Set(['awesome', 'tutorial', 'interview']);
// The AI band's per-kind columns: a domain leaf and the label key for it.
const AI_KINDS = [
  { path: 'agents/skills', key: 'aiSkill' },
  { path: 'agents/mcp', key: 'aiMcp' },
  { path: 'agents/coding', key: 'aiCoding' },
] as const;

const repoHref = (r: RepoSummary) => `/repo/${r.owner}/${r.name}`;

/** Store shelves are three levels deep and live under /apps; domains under /github. */
function domainHref(cell: HeatCell): string {
  const parts = cell.path.split('/');
  if (parts[0] === 'apps' && parts.length === 3) return `/apps/best/${parts[1]}/${parts[2]}`;
  return boardHref({ category: cell.path });
}

/** "owner/" in ink-3, then the name: the house style for a repo's full name. */
function RepoName({ repo, className = '' }: { repo: RepoSummary; className?: string }) {
  return (
    <span className={`truncate ${className}`}>
      <span className="font-normal text-ink-3">{repo.owner}/</span>
      {repo.name}
    </span>
  );
}

/** Text link with an ink underline, the page's "go further" affordance. */
function MoreLink({ href, children }: { href: string; children: ReactNode }) {
  return (
    <Link
      href={href}
      className="inline-flex min-h-11 items-center whitespace-nowrap text-sr-small font-semibold text-ink underline decoration-1 underline-offset-4 hover:decoration-2 md:min-h-0"
    >
      {children} →
    </Link>
  );
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

  const [daily, top, heat, apps, models, stale, posts, ...aiTops] = await Promise.all([
    searchRepos({ sort: 'daily', page: 1, perPage: 30 }),
    searchRepos({ sort: 'stars', page: 1, perPage: 10 }),
    getHeatmap(),
    getApps({ sort: 'hot', limit: 5 }),
    getModels({ sort: 'hot', limit: 5 }),
    getStaleness(0),
    listPosts(locale),
    ...AI_KINDS.map((k) => searchRepos({ category: k.path, sort: 'daily', page: 1, perPage: 1 })),
  ]);

  const movers = (daily.data?.items ?? []).filter((r) => !r.type || !RESOURCE_TYPES.has(r.type));
  const topByStars = top.data?.items ?? [];
  const ids = Array.from(new Set([...movers.slice(0, 16), ...topByStars].map((r) => r.fullName)));
  const trends = await getTrends(ids, 31);

  const lead = pickLead(movers, trends);
  const fresh = pickFresh(movers, trends);
  const cool = pickCool(topByStars, trends, new Set([lead?.repo.fullName, fresh?.repo.fullName].filter(Boolean) as string[]));
  const top10 = movers.slice(0, 10);

  const leaves = heat.flatMap((g) => g.children ?? []);
  const leafName = new Map(leaves.map((c) => [c.path, categoryLabel(c, locale)]));
  const aiFields = heat.filter((g) => g.path === 'agents' || g.path === 'ai').flatMap((g) => g.children ?? []);
  const hotField = [...aiFields].sort((a, b) => b.growth - a.growth)[0];
  const domains = [...leaves].sort((a, b) => b.growth - a.growth).slice(0, 10);
  const maxGrowth = Math.max(1, domains[0]?.growth ?? 1);
  const modelItems = models.items;
  const total = stale?.total ?? null;
  const bucket = (k: string) => stale?.buckets.find((b) => b.bucket === k)?.repos ?? null;

  const taskLabel = (k?: string) => (k ? (tr(`modelTask_${k.replace('-', '_')}` as never) as string) : '');
  const dateline = new Intl.DateTimeFormat(locale, { dateStyle: 'full', timeZone: 'UTC' }).format(new Date());

  // AI band cells: the hottest AI subfield, the top riser in three agent
  // kinds, and the model gaining most likes.
  type Cell = { key: string; kind: string; name: ReactNode; sub: string; gain: number; href: string; external?: boolean };
  const aiCells: Cell[] = [];
  if (hotField) {
    aiCells.push({
      key: `field:${hotField.path}`,
      kind: t('aiHotField'),
      name: categoryLabel(hotField, locale),
      sub: t('projects', { n: fmtInt(hotField.repos) }),
      gain: hotField.growth,
      href: domainHref(hotField),
    });
  }
  AI_KINDS.forEach((k, i) => {
    const r = aiTops[i]?.data?.items[0];
    if (r) {
      aiCells.push({
        key: `kind:${k.path}`,
        kind: t(k.key),
        name: <RepoName repo={r} />,
        sub: `${fmtTotal(r.stars)} star`,
        gain: r.dailyIncrease ?? 0,
        href: repoHref(r),
      });
    }
  });
  const m0 = modelItems[0];
  if (m0) {
    aiCells.push({
      key: m0.externalId,
      kind: t('aiModel'),
      name: m0.externalId,
      sub: [taskLabel(m0.task), fmtParams(m0.params)].filter(Boolean).join(' · '),
      gain: m0.growth,
      href: m0.url,
      external: true,
    });
  }

  return (
    <div className="bg-paper font-text text-ink">
      <PageShell className="flex flex-col gap-8 pb-4 pt-0 md:gap-12">
        {/* Masthead strip: the date and what the paper covers. */}
        <div className="flex flex-wrap justify-between gap-x-3 gap-y-1 border-b border-rule py-2.5 text-sr-caption text-ink-3">
          <span>{dateline}</span>
          {total != null && <span>{t('masthead', { n: fmtInt(total) })}</span>}
        </div>

        {aiCells.length > 0 && (
          <section aria-labelledby="ai-today" className="-mt-2 flex flex-col md:-mt-6">
            <div className="flex items-baseline justify-between gap-3 pb-1">
              <h2 id="ai-today" className="text-[13px] font-semibold tracking-[0.1em]">
                {t('aiTitle')} <span className="font-normal tracking-normal text-ink-3">{t('aiSub')}</span>
              </h2>
              <MoreLink href="/ai">{t('aiAll')}</MoreLink>
            </div>
            <ul className="grid grid-cols-2 border-y border-ink md:grid-cols-5">
              {aiCells.map((c, i) => {
                const inner = (
                  <>
                    <span className="text-sr-caption text-ink-3">{c.kind}</span>
                    <span className="truncate text-[15px] font-semibold">{c.name}</span>
                    <span className="flex items-baseline justify-between gap-2 text-[13px]">
                      <span className="truncate text-ink-3">{c.sub}</span>
                      <Delta value={c.gain} className="font-semibold" />
                    </span>
                  </>
                );
                const cls = 'flex min-w-0 flex-col gap-1 px-3 py-3.5 hover:bg-sunk md:px-4';
                return (
                  <li
                    key={c.key}
                    className={`min-w-0 ${i === 0 ? 'col-span-2 md:col-span-1' : 'border-t border-rule md:border-t-0'} ${
                      i > 0 && i % 2 === 0 ? 'border-l border-rule' : ''
                    } ${i > 0 ? 'md:border-l md:border-rule' : ''}`}
                  >
                    {c.external ? (
                      <a href={c.href} target="_blank" rel="noopener noreferrer" className={cls}>
                        {inner}
                      </a>
                    ) : (
                      <Link href={c.href} className={cls}>
                        {inner}
                      </Link>
                    )}
                  </li>
                );
              })}
            </ul>
          </section>
        )}

        {(lead || fresh || cool) && (
          <section className="grid gap-8 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)] lg:gap-14">
            {lead ? (
              <article className="flex min-w-0 flex-col gap-4">
                <Kicker>{t('leadKicker')}</Kicker>
                <h1 className="text-balance font-news text-[30px] font-medium leading-[1.1] tracking-[-0.015em] md:text-[48px]">
                  {lead.breakout && lead.prior != null
                    ? t('leadBreakout', {
                        name: lead.repo.name,
                        d7: fmtInt(lead.repo.weeklyIncrease ?? 0),
                        prior: fmtInt(lead.prior),
                      })
                    : t('leadSteady', {
                        name: lead.repo.name,
                        d1: fmtInt(lead.repo.dailyIncrease ?? 0),
                        d7: fmtInt(lead.repo.weeklyIncrease ?? 0),
                      })}
                </h1>
                {lead.repo.description && (
                  <p className="max-w-[720px] text-sr-body text-ink-2 md:text-sr-lead">
                    <span className="font-semibold text-ink">{lead.repo.fullName}</span> — {lead.repo.description}
                  </p>
                )}
                <div className="flex flex-wrap items-end gap-x-10 gap-y-4">
                  <StatBlock size="lg" label={t('statToday')} value={<Delta value={lead.repo.dailyIncrease} />} />
                  <StatBlock label={t('stat7d')} value={`+${fmtInt(lead.repo.weeklyIncrease ?? 0)}`} />
                  <StatBlock label={t('statStars')} value={fmtInt(lead.repo.stars)} />
                  {lead.repo.rank ? <StatBlock label={t('statRank')} value={`#${fmtInt(lead.repo.rank)}`} /> : null}
                </div>
                <figure className="flex flex-col gap-1.5">
                  <div className="h-24 border-b border-ink md:h-40">
                    <DailyBars
                      values={lead.series}
                      height="100%"
                      label={t('barsAria', { name: lead.repo.name, n: lead.series.length })}
                    />
                  </div>
                  <figcaption className="flex justify-between text-sr-caption text-ink-3">
                    <span>{t('barsStart')}</span>
                    <span>{t('barsCaption')}</span>
                    <span>{t('barsEnd')}</span>
                  </figcaption>
                </figure>
                <div className="flex flex-wrap items-center gap-2">
                  {lead.repo.language && <Tag>{lead.repo.language}</Tag>}
                  {lead.repo.categoryPath[0] && leafName.get(lead.repo.categoryPath[0]) && (
                    <Tag>{leafName.get(lead.repo.categoryPath[0])}</Tag>
                  )}
                  <span className="ml-auto">
                    <MoreLink href={repoHref(lead.repo)}>{t('profile')}</MoreLink>
                  </span>
                </div>
              </article>
            ) : (
              <div />
            )}

            <aside className="flex min-w-0 flex-col border-t-2 border-ink pt-6 lg:border-t-0 lg:pt-0">
              {fresh && (
                <article className="flex flex-col gap-2.5 border-b border-rule pb-6">
                  <Kicker>{t('freshKicker')}</Kicker>
                  <h2 className="font-news text-[24px] font-medium leading-[1.2] md:text-[26px]">
                    <Link href={repoHref(fresh.repo)} className="hover:underline">
                      {t('freshHead', { name: fresh.repo.name, days: fresh.days, stars: fmtInt(fresh.repo.stars) })}
                    </Link>
                  </h2>
                  {fresh.repo.description && (
                    <p className="line-clamp-3 text-[15px] leading-relaxed text-ink-2">{fresh.repo.description}</p>
                  )}
                  <div className="flex items-end gap-4">
                    <Delta value={fresh.repo.dailyIncrease} className="text-[28px] font-semibold leading-none" />
                    <div className="h-9 flex-1 border-b border-ink">
                      <DailyBars
                        values={fresh.series}
                        height="100%"
                        label={t('barsAria', { name: fresh.repo.name, n: fresh.series.length })}
                      />
                    </div>
                  </div>
                </article>
              )}
              {cool && (
                <article className="flex flex-col gap-2.5 border-b border-rule py-6">
                  <Kicker tone="muted">{t('coolKicker')}</Kicker>
                  <h2 className="font-news text-[24px] font-medium leading-[1.2] md:text-[26px]">
                    <Link href={repoHref(cool.repo)} className="hover:underline">
                      {t('coolHead', {
                        name: cool.repo.name,
                        rank: cool.repo.rank ?? '—',
                        from: fmtInt(cool.from),
                        to: fmtInt(cool.to),
                      })}
                    </Link>
                  </h2>
                  <p className="text-[15px] leading-relaxed text-ink-2">
                    {t('coolBody', {
                      stars: fmtInt(cool.repo.stars),
                      d30: fmtInt(cool.series.reduce((a, b) => a + b, 0)),
                    })}
                  </p>
                  <div className="flex items-end gap-4">
                    <Delta value={cool.to} hot={false} className="text-[28px] font-semibold leading-none" />
                    <div className="h-9 flex-1 border-b border-ink">
                      <DailyBars
                        values={cool.series}
                        height="100%"
                        highlightLast={false}
                        label={t('barsAria', { name: cool.repo.name, n: cool.series.length })}
                      />
                    </div>
                  </div>
                </article>
              )}
              {stale && (
                <div className="grid grid-cols-2 gap-4 pt-5">
                  <StatBlock size="sm" label={t('statActive')} value={fmtInt(bucket('active') ?? 0)} />
                  <StatBlock size="sm" label={t('statStale')} value={fmtInt(bucket('stale') ?? 0)} />
                </div>
              )}
            </aside>
          </section>
        )}

        {top10.length > 0 && (
          <section aria-labelledby="movers">
            <SectionHeader id="movers" title={t('moversTitle')} aside={<MoreLink href={boardHref({ sort: 'daily' })}>{t('seeAll')}</MoreLink>} />
            <div
              aria-hidden="true"
              className="hidden grid-cols-[32px_minmax(0,1fr)_200px_84px_84px_72px] gap-x-6 border-b border-rule py-2 text-sr-caption text-ink-3 md:grid"
            >
              <span>#</span>
              <span>{t('colProject')}</span>
              <span>{t('colBars')}</span>
              <span className="text-right">{t('statToday')}</span>
              <span className="text-right">{t('col7d')}</span>
              <span className="text-right">{t('colStars')}</span>
            </div>
            <ol>
              {top10.map((r, i) => {
                const series = dailySeries(trends[r.fullName]);
                const isNew = isNewEntry(trends[r.fullName]);
                return (
                  <li key={r.fullName} className="border-b border-rule">
                    <Link
                      href={repoHref(r)}
                      className="grid grid-cols-[22px_minmax(0,1fr)_auto] items-center gap-x-3 gap-y-1.5 py-3 hover:bg-sunk md:grid-cols-[32px_minmax(0,1fr)_200px_84px_84px_72px] md:gap-x-6"
                    >
                      <span className="text-sr-small tabular-nums text-ink-3">{i + 1}</span>
                      <span className="flex min-w-0 flex-col gap-0.5">
                        <span className="flex min-w-0 items-center gap-2 text-[15px] font-semibold">
                          <RepoName repo={r} />
                          {isNew && (
                            <Tag tone="signal" className="shrink-0 !px-1.5 !py-0 !text-[11px]">
                              {t('newTag')}
                            </Tag>
                          )}
                        </span>
                        {r.description && (
                          <span className="hidden truncate text-[13px] text-ink-2 md:block">{r.description}</span>
                        )}
                      </span>
                      <span className="order-none text-right text-[15px] font-semibold md:order-3">
                        <Delta value={r.dailyIncrease} />
                      </span>
                      <span className="col-start-2 h-6 md:order-2 md:col-start-auto md:h-7">
                        <DailyBars values={series} height="100%" label={t('barsAria', { name: r.name, n: series.length })} />
                      </span>
                      <span className="hidden text-right text-sr-small tabular-nums text-ink-2 md:order-4 md:block">
                        {isNew ? t('newTag') : `+${fmtInt(r.weeklyIncrease ?? 0)}`}
                      </span>
                      <span className="text-right text-sr-small tabular-nums text-ink-2 md:order-5">{fmtTotal(r.stars)}</span>
                    </Link>
                  </li>
                );
              })}
            </ol>
          </section>
        )}

        <section className="grid gap-10 lg:grid-cols-2 lg:gap-12">
          {domains.length > 0 && (
            <div>
              <SectionHeader title={t('domainsTitle')} aside={t('domainsSub')} />
              <ol>
                {domains.map((g, i) => (
                  <li key={g.path} className="border-b border-rule">
                    <Link
                      href={domainHref(g)}
                      className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-4 gap-y-1.5 py-2.5 hover:bg-sunk md:grid-cols-[minmax(0,1fr)_88px_160px]"
                    >
                      <span className="flex min-w-0 items-baseline gap-2">
                        <span className="truncate text-[15px] font-semibold">{categoryLabel(g, locale)}</span>
                        <span className="shrink-0 text-sr-caption text-ink-3">{t('projects', { n: fmtInt(g.repos) })}</span>
                      </span>
                      <span className="text-right text-sr-small font-semibold tabular-nums">
                        <Delta value={g.growth} hot={i === 0} />
                      </span>
                      <span className="col-span-2 h-1.5 bg-sunk md:col-span-1" aria-hidden="true">
                        <span
                          className={`block h-full ${i === 0 ? 'bg-signal-bar' : 'bg-ink'}`}
                          style={{ width: `${Math.max(2, Math.round((Math.max(g.growth, 0) / maxGrowth) * 100))}%` }}
                        />
                      </span>
                    </Link>
                  </li>
                ))}
              </ol>
            </div>
          )}

          <div className="flex flex-col gap-10">
            {apps.items.length > 0 && (
              <div>
                <SectionHeader title={t('appsTitle')} aside={<MoreLink href="/apps">{t('appsAll')}</MoreLink>} />
                <ol>
                  {apps.items.slice(0, 5).map((a) => {
                    const [owner = '', name = ''] = a.externalId.split('/');
                    return (
                      <li key={a.externalId} className="border-b border-rule">
                        <Link
                          href={`/repo/${owner}/${name}`}
                          className="grid grid-cols-[32px_minmax(0,1fr)_auto] items-center gap-3 py-2.5 hover:bg-sunk"
                        >
                          <span
                            aria-hidden="true"
                            className="flex h-8 w-8 items-center justify-center rounded border border-rule bg-sunk text-sr-small font-semibold text-ink-2"
                          >
                            {a.name.slice(0, 1).toUpperCase()}
                          </span>
                          <span className="flex min-w-0 flex-col">
                            <span className="truncate text-[15px] font-semibold">{a.name}</span>
                            {a.platforms.length > 0 && (
                              <span className="truncate text-sr-caption text-ink-3">{a.platforms.join(' · ')}</span>
                            )}
                          </span>
                          <span className="text-sr-small font-semibold">
                            <Delta value={a.growth} />
                          </span>
                        </Link>
                      </li>
                    );
                  })}
                </ol>
              </div>
            )}
            {modelItems.length > 0 && (
              <div>
                <SectionHeader title={t('modelsTitle')} aside={t('modelsSub')} />
                <ol>
                  {modelItems.map((m) => (
                    <li key={m.externalId} className="border-b border-rule">
                      <a
                        href={m.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 py-2.5 hover:bg-sunk"
                      >
                        <span className="flex min-w-0 flex-col">
                          <span className="truncate text-[15px] font-semibold">
                            {m.author && <span className="font-normal text-ink-3">{m.author}/</span>}
                            {m.name}
                          </span>
                          <span className="truncate text-sr-caption text-ink-3">
                            {[taskLabel(m.task), fmtParams(m.params)].filter(Boolean).join(' · ')}
                          </span>
                        </span>
                        <span className="text-sr-small font-semibold">
                          <Delta value={m.growth} hot={m.growth >= HOT_DAILY} />
                        </span>
                      </a>
                    </li>
                  ))}
                </ol>
              </div>
            )}
          </div>
        </section>

        {posts.length > 0 && (
          <section>
            <SectionHeader title={t('insights')} aside={<MoreLink href="/blog">{t('seeAll')}</MoreLink>} />
            <ul>
              {posts.slice(0, 3).map((p) => (
                <li key={p.slug} className="border-b border-rule">
                  <Link href={`/blog/${p.slug}`} className="flex items-baseline justify-between gap-4 py-3 hover:bg-sunk">
                    <span className="min-w-0">
                      <span className="block font-news text-[20px] font-medium leading-snug">{p.title}</span>
                      <span className="mt-0.5 block truncate text-sr-small text-ink-2">{p.description}</span>
                    </span>
                    <span className="shrink-0 text-sr-caption tabular-nums text-ink-3">{p.date.slice(5, 10)}</span>
                  </Link>
                </li>
              ))}
            </ul>
          </section>
        )}

        <p className="flex flex-col gap-1 border-t-2 border-ink pt-3 text-sr-caption leading-relaxed text-ink-3 md:flex-row md:justify-between md:gap-8">
          <span className="max-w-[820px]">
            <span className="font-semibold text-ink-2">{t('rulesLabel')}</span> {t('rules')}
          </span>
          <span className="shrink-0">{t('dataNote')}</span>
        </p>
      </PageShell>
    </div>
  );
}
