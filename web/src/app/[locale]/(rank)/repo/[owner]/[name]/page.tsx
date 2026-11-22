import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import { notFound, permanentRedirect } from 'next/navigation';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import {
  categoryLabel,
  findCategory,
  getCategoryTree,
  getDomainPeers,
  getOwnerRepos,
  getReadme,
  getRelatedRepos,
  getRepo,
  getStarHistory,
  type OS,
  type RepoSummary,
} from '@/lib/data';
import { homepageHost } from '@/lib/rank-data';
import {
  community,
  daysSince,
  licenseKind,
  licenseShort,
  pace,
  peakDay,
  recentDaily,
  trackedPoints,
  upkeep,
} from '@/lib/repo-facts';
import { RankMove } from '@/components/rank/RankMove';
import { GrowthChart } from '@/components/rank/GrowthChart';
import { ReadmeBlock } from '@/components/rank/ReadmeBlock';
import { Downloads } from '@/components/rank/Downloads';
import { PageShell } from '@/components/rank/PageShell';
import { CopyCommand } from '@/components/rank/CopyCommand';
import { BadgeCopy } from '@/components/rank/BadgeCopy';
import { ChartToggle } from '@/components/rank/ChartToggle';
import { DailyBars, Delta, StatBlock, fmtInt, fmtTotal, srButton } from '@/components/ui';
import { boardHref } from '@/lib/board-url';
import { PUBLIC_API_URL, SITE_URL, alternatesFor } from '@/lib/seo';

const OS_LABEL: Record<OS, string> = {
  macos: 'macOS',
  windows: 'Windows',
  linux: 'Linux',
  android: 'Android',
  ios: 'iOS',
  web: 'Web',
};

/** A section opened by a 2px ink rule, the page's one heading style. */
function Block({ title, id, aside, children }: { title: string; id: string; aside?: ReactNode; children: ReactNode }) {
  return (
    <section aria-labelledby={id} className="flex min-w-0 flex-col">
      <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1 border-t-2 border-ink py-3">
        <h2 id={id} className="text-[18px] font-semibold">
          {title}
        </h2>
        {aside && <span className="text-sr-caption text-ink-3">{aside}</span>}
      </div>
      {children}
    </section>
  );
}

/** Peers, the owner's other repos: name, today's gain, stars. */
function RepoList({ repos, narrow = false }: { repos: RepoSummary[]; narrow?: boolean }) {
  return (
    <ul>
      {repos.map((r) => (
        <li key={r.fullName}>
          <Link
            href={`/repo/${r.owner}/${r.name}`}
            className={`grid gap-3 border-b border-rule py-3 text-sr-small hover:bg-sunk ${narrow ? 'grid-cols-[minmax(0,1fr)_56px_56px]' : 'grid-cols-[minmax(0,1fr)_72px_64px] md:grid-cols-[minmax(0,1fr)_80px_80px] md:gap-4'}`}
          >
            <span className="truncate font-semibold">
              <span className="font-normal text-ink-3">{r.owner}/</span>
              {r.name}
            </span>
            <Delta value={r.dailyIncrease} className="text-right" />
            <span className="text-right font-semibold tabular-nums">{fmtTotal(r.stars)}</span>
          </Link>
        </li>
      ))}
    </ul>
  );
}

interface Params {
  locale: string;
  owner: string;
  name: string;
}

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  return { alternates: alternatesFor(params.locale, `/repo/${params.owner}/${params.name}`), title: `${params.owner}/${params.name}` };
}

export default async function RepoDetailPage({ params }: { params: Params }) {
  setRequestLocale(params.locale);
  const t = await getTranslations('rank');
  const d = await getTranslations('repoPage');
  const locale = params.locale;
  const now = Date.now();

  const repoRes = await getRepo(params.owner, params.name);
  if (repoRes.error !== null) notFound();
  const repo = repoRes.data;
  // A renamed repository: the API followed it to its new name, so does the page.
  if (repo.fullName !== `${params.owner}/${params.name}`) {
    permanentRedirect(`/${locale}/repo/${repo.fullName}`);
  }

  const [history, readme, ownerRepos, peers, tree] = await Promise.all([
    getStarHistory(repo.owner, repo.name),
    getReadme(repo.owner, repo.name),
    getOwnerRepos(repo),
    getDomainPeers(repo),
    getCategoryTree(),
  ]);
  // Without a domain, the repos sharing its leading topic stand in.
  const related = peers.length === 0 ? await getRelatedRepos(repo, 6) : [];

  const labelFor = (path: string) => {
    const node = findCategory(tree, path);
    return node ? categoryLabel(node, locale) : (path.split('/').pop() ?? path);
  };
  const leaf = repo.categoryPath[0];
  const group = leaf?.includes('/') ? leaf.split('/')[0] : undefined;
  const crumbs = [
    { label: 'GitHub', href: '/github' },
    ...(group ? [{ label: labelFor(group), href: boardHref({ category: group }) }] : []),
    ...(leaf ? [{ label: labelFor(leaf), href: boardHref({ category: leaf }) }] : []),
  ];

  // Rankings with their move since yesterday (kept from the rank-change work).
  const ranks = repo.ranks.map((r) => {
    const scope = r.scope === 'overall' ? t('rankOverall') : r.scope === 'language' ? r.key! : labelFor(r.key!);
    const href =
      r.scope === 'category' ? boardHref({ category: r.key! }) : r.scope === 'language' ? boardHref({ lang: r.key! }) : '/github';
    return { key: `${r.scope}:${r.key ?? ''}`, scope, rank: r.rank, change: r.change, isNew: r.isNew, href };
  });
  const overall = repo.ranks.find((r) => r.scope === 'overall');
  const domainRank = repo.ranks.find((r) => r.scope === 'category');

  // ---- Numbers ----
  const win = recentDaily(history, 31);
  const series = win.series;
  const p = pace(series);
  const n = series.length;
  const d1 = repo.dailyIncrease;
  const d7 = repo.weeklyIncrease;
  const vsStart = n >= 14 && d1 != null && series[0] > 0 ? Math.round(((d1 - series[0]) / series[0]) * 100) : null;
  const maxBar = Math.max(1, ...series);

  // ---- Dates ----
  const dateLabel = (ms: number) => {
    const iso = new Date(ms).toISOString();
    return new Date(ms).getUTCFullYear() === new Date(now).getUTCFullYear() ? iso.slice(5, 10) : iso.slice(0, 10);
  };
  const ago = (days: number) => (days === 0 ? d('today') : days === 1 ? d('yesterday') : d('daysAgo', { n: days }));
  const pushedDays = daysSince(repo.pushedAt, now);
  const release = repo.latestRelease?.publishedAt ? repo.latestRelease : null;
  const releaseDays = release ? daysSince(release.publishedAt, now) : null;

  // ---- The four questions ----
  const up = upkeep(pushedDays);
  const upWhy = [
    pushedDays != null ? d('whyPushed', { when: ago(pushedDays) }) : d('whyPushedUnknown'),
    release && releaseDays != null
      ? d('whyRelease', { tag: release.tag ?? '', when: ago(releaseDays) })
      : d('whyNoRelease'),
  ].join(d('sep'));

  const paceWhy =
    p.momentum === 'new'
      ? d('whyPaceNew', { n, d1: fmtInt(d1 ?? 0) })
      : d('whyPace', { recent: fmtInt(p.recent), n, before: fmtInt(p.before) }) +
        (domainRank && leaf ? d('sep') + d('whyDomainRank', { domain: labelFor(domainRank.key!), rank: fmtInt(domainRank.rank) }) : '');

  const lk = licenseKind(repo.license);
  const platforms = repo.platforms.map((o) => OS_LABEL[o]).filter(Boolean);
  const useAnswer = [licenseShort(repo.license) ?? d('noLicense'), platforms.join(' / ')].filter(Boolean).join(' · ');
  const useWhy =
    d(`license_${lk}`) +
    (platforms.length ? d('sep') + d(repo.platformSource === 'asset' ? 'platformAsset' : 'platformInferred') : '');

  const checks = [
    { q: d('qUpkeep'), a: d(`upkeep_${up}`), why: upWhy },
    { q: d('qPace'), a: d(`pace_${p.momentum}`), why: paceWhy },
    {
      q: d('qCommunity'),
      a: d(`community_${community(repo.forks)}`),
      why: d('whyCommunity', { forks: fmtInt(repo.forks), issues: fmtInt(repo.openIssues) }),
    },
    { q: d('qUse'), a: useAnswer, why: useWhy },
  ];

  // ---- Timeline, newest first ----
  const tracked = trackedPoints(history);
  const timeline: { at: number; what: string; src: string; tone: 'ink' | 'signal' | 'muted' }[] = [];
  if (pushedDays != null) timeline.push({ at: Date.parse(repo.pushedAt), what: d('tlPushed'), src: 'pushedAt', tone: 'ink' });
  if (tracked.length > 0 && d1 != null) {
    timeline.push({
      at: tracked[tracked.length - 1].t,
      what: overall ? d('tlSnapshotRank', { d1: fmtInt(d1), rank: fmtInt(overall.rank) }) : d('tlSnapshot', { d1: fmtInt(d1) }),
      src: d('srcSnapshot'),
      tone: 'ink',
    });
  }
  if (release) timeline.push({ at: Date.parse(release.publishedAt!), what: d('tlRelease', { tag: release.tag ?? '' }), src: 'GitHub Releases', tone: 'ink' });
  const peak = peakDay(series);
  if (peak != null) {
    timeline.push({ at: win.dayOf(peak), what: d('tlPeak', { n: fmtInt(series[peak]) }), src: d('srcSnapshot'), tone: 'signal' });
  }
  if (tracked.length > 1) {
    timeline.push({ at: tracked[0].t, what: d('tlFirst', { n: fmtInt(tracked[0].v) }), src: d('srcSnapshot'), tone: 'muted' });
  }
  timeline.sort((a, b) => b.at - a.at);

  // ---- Chart ----
  const caption =
    p.momentum === 'cooling'
      ? d('chartCooling', { from: fmtInt(p.before), to: fmtInt(p.recent) })
      : p.momentum === 'rising'
        ? d('chartRising', { from: fmtInt(p.before), to: fmtInt(p.recent) })
        : p.momentum === 'steady'
          ? d('chartSteady', { to: fmtInt(p.recent) })
          : d('chartNew', { n });

  const daily = (
    <div className="grid grid-cols-[40px_minmax(0,1fr)] gap-2 md:grid-cols-[48px_minmax(0,1fr)]">
      <div aria-hidden="true" className="flex h-[160px] flex-col justify-between text-right text-[11px] tabular-nums text-ink-3 md:h-[240px]">
        <span>{fmtTotal(maxBar)}</span>
        <span>{fmtTotal(Math.round(maxBar / 2))}</span>
        <span>0</span>
      </div>
      <div className="h-[160px] border-b border-ink bg-[linear-gradient(rgb(var(--sr-rule))_1px,transparent_1px)] bg-[length:100%_50%] md:h-[240px]">
        <DailyBars values={series} days={Math.max(n, 1)} height="100%" label={d('barsAria', { n, caption })} />
      </div>
      <span />
      <div aria-hidden="true" className="flex justify-between text-[11px] text-ink-3">
        <span>{d('nDaysAgo', { n })}</span>
        {n >= 10 && <span>{d('nDaysAgo', { n: Math.round(n / 2) })}</span>}
        <span>{d('lastDay')}</span>
      </div>
    </div>
  );

  const host = homepageHost(repo.homepage);
  const lead = locale.startsWith('zh') ? repo.taglineZh : repo.taglineEn;
  const meta = [repo.language, licenseShort(repo.license), repo.type && repo.type !== 'software' ? t.has(`typeFacet.${repo.type}`) ? t(`typeFacet.${repo.type}`) : repo.type : null, platforms.join(' / ')]
    .filter(Boolean)
    .join(' · ');

  return (
    <PageShell className="pb-12">
      <section className="flex flex-col gap-3 border-b border-rule pb-6 pt-6 md:pb-7 md:pt-10">
        <nav aria-label={d('crumbs')} className="flex flex-wrap gap-x-1.5 text-sr-caption text-ink-3">
          {crumbs.map((c, i) => (
            <span key={c.href} className="flex gap-1.5">
              {i > 0 && <span aria-hidden="true">·</span>}
              <Link href={c.href} className="hover:text-ink">
                {c.label}
              </Link>
            </span>
          ))}
        </nav>
        <h1 className="break-words font-news text-[40px] font-medium leading-[1.05] tracking-[-0.02em] [overflow-wrap:anywhere] md:text-[64px]">
          <span className="text-ink-3">{repo.owner}/</span>
          {repo.name}
        </h1>
        {(lead || repo.description) && (
          <p className="max-w-[760px] break-words text-sr-body text-ink-2 [overflow-wrap:anywhere] md:text-sr-lead">
            {lead && lead !== repo.description ? (
              <>
                {lead} {repo.description && <span className="text-ink-3">{repo.description}</span>}
              </>
            ) : (
              repo.description
            )}
          </p>
        )}
        <div className="flex flex-wrap items-center gap-2">
          <a href={repo.htmlUrl} target="_blank" rel="noopener noreferrer" className={srButton('solid')}>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true">
              <path d="M9 19c-4 1.5-4-2-6-2.5M15 22v-3.5a3 3 0 0 0-.9-2.4c3-.3 6.2-1.5 6.2-6.7a5.2 5.2 0 0 0-1.4-3.6 4.8 4.8 0 0 0-.1-3.6s-1.1-.3-3.7 1.4a12.7 12.7 0 0 0-6.6 0C6.1 1.9 5 2.2 5 2.2a4.8 4.8 0 0 0-.1 3.6A5.2 5.2 0 0 0 3.5 9.4c0 5.2 3.2 6.4 6.2 6.7a3 3 0 0 0-.9 2.4V22" />
            </svg>
            GitHub
          </a>
          {host && (
            <a
              href={repo.homepage!.startsWith('http') ? repo.homepage! : `https://${repo.homepage}`}
              target="_blank"
              rel="noopener noreferrer"
              className={srButton('quiet', 'max-w-full !text-ink')}
            >
              <span className="truncate">{host}</span> ↗
            </a>
          )}
          {meta && <span className="text-[13px] text-ink-3 md:ml-2">{meta}</span>}
        </div>
        {repo.alternativeTo.length > 0 && (
          <p className="text-sr-small text-ink-2">
            {t('appsAltTo')}{' '}
            {repo.alternativeTo.map((a, i) => (
              <span key={a.slug}>
                {i > 0 && '、'}
                <Link href={`/alternatives/${a.slug}`} className="font-semibold text-ink underline underline-offset-4">
                  {a.name}
                </Link>
              </span>
            ))}
          </p>
        )}
      </section>

      <div className="grid gap-8 py-8 lg:grid-cols-[minmax(0,1fr)_320px] lg:gap-14">
        <div className="flex min-w-0 flex-col gap-7">
          <div className="grid grid-cols-3 border-t-2 border-ink">
            <StatBlock
              className="border-r border-rule py-3.5 pr-3 md:pr-4"
              label={d('statToday')}
              value={<Delta value={d1} />}
              sub={vsStart == null ? undefined : d(vsStart < 0 ? 'vsStartDown' : 'vsStartUp', { n, p: Math.abs(vsStart) })}
            />
            <StatBlock
              className="border-r border-rule py-3.5 pl-3 pr-3 md:pl-4 md:pr-4"
              label={d('stat7d')}
              value={<Delta value={d7} hot={false} className="!text-ink" />}
              sub={d7 == null ? undefined : d('perDay', { n: fmtInt(d7 / 7) })}
            />
            <StatBlock
              className="py-3.5 pl-3 md:pl-4"
              label={d('statStars')}
              value={fmtInt(repo.stars)}
              sub={repo.monthlyIncrease == null ? undefined : d('stat30d', { n: fmtInt(repo.monthlyIncrease) })}
            />
          </div>

          {history.length >= 2 && (
            <div className="flex flex-col gap-2.5">
              <ChartToggle
                caption={caption}
                label={d('chartKind')}
                views={[
                  ...(n > 0 ? [{ key: 'daily', label: d('chartDaily'), body: daily }] : []),
                  { key: 'total', label: d('chartTotal'), body: <GrowthChart tone="sr" points={history} className="h-[180px] md:h-[264px]" /> },
                ]}
              />
              {p.momentum !== 'new' && (
                <div className="grid grid-cols-2 gap-4 text-sr-small text-ink-2">
                  <div className="border-t border-rule pt-2.5">
                    {d('avgBefore', { n })} <span className="font-semibold tabular-nums text-ink">{fmtInt(p.before)}</span>
                  </div>
                  <div className="border-t border-rule pt-2.5">
                    {d('avgRecent')} <span className="font-semibold tabular-nums text-ink">{fmtInt(p.recent)}</span>
                  </div>
                </div>
              )}
            </div>
          )}

          <Block id="judge" title={d('judgeTitle')} aside={d('judgeSub')}>
            {checks.map((c) => (
              <div
                key={c.q}
                className="grid grid-cols-[88px_minmax(0,1fr)] items-baseline gap-x-4 gap-y-1 border-b border-rule py-3 text-sr-small md:grid-cols-[112px_168px_minmax(0,1fr)]"
              >
                <span className="text-ink-3">{c.q}</span>
                <span className="break-words font-semibold">{c.a}</span>
                <span className="col-start-2 break-words text-ink-2 md:col-start-auto">{c.why}</span>
              </div>
            ))}
            {timeline.length > 0 && (
              <>
                <h3 className="mt-6 border-b border-ink pb-2 text-sr-small font-semibold">{d('timeline')}</h3>
                <ol>
                  {timeline.map((e) => (
                    <li
                      key={`${e.at}:${e.what}`}
                      className="grid grid-cols-[64px_12px_minmax(0,1fr)] items-baseline gap-x-3 border-b border-rule py-2.5 text-sr-small md:grid-cols-[88px_12px_minmax(0,1fr)]"
                    >
                      <span className="text-[13px] tabular-nums text-ink-3">{dateLabel(e.at)}</span>
                      <span
                        aria-hidden="true"
                        className={`h-[7px] w-[7px] self-center rounded-full ${
                          e.tone === 'signal' ? 'bg-signal-bar' : e.tone === 'muted' ? 'bg-ink-3' : 'bg-ink'
                        }`}
                      />
                      <span className="break-words">
                        <span className="font-medium">{e.what}</span>{' '}
                        <span className="text-[13px] text-ink-3">· {e.src}</span>
                      </span>
                    </li>
                  ))}
                </ol>
              </>
            )}
          </Block>

          {/* Only what is certain: the release binaries GitHub lists, and a
              clone. Package names are not guessed from the language. */}
          <Block id="get" title={d('getTitle')}>
            <div className="flex flex-col gap-3">
              {repo.releaseAssets.length > 0 && (
                <Downloads assets={repo.releaseAssets} fullName={repo.fullName} locale={locale} />
              )}
              <div className="flex min-w-0 items-center gap-3 rounded border border-rule bg-sunk px-3 py-3">
                <CopyCommand command={`git clone https://github.com/${repo.fullName}`} repo={repo.fullName} />
              </div>
            </div>
          </Block>

          {peers.length > 0 && leaf && (
            <Block id="peers" title={d('peersTitle', { name: labelFor(leaf) })} aside={<Link href={boardHref({ category: leaf, sort: 'weekly' })} className="underline underline-offset-4 hover:text-ink">{t('seeAll')}</Link>}>
              <RepoList repos={peers.slice(0, 5)} />
            </Block>
          )}
          {related.length > 0 && (
            <Block id="related" title={t('relatedRepos')}>
              <RepoList repos={related.slice(0, 5)} />
            </Block>
          )}

          {readme && <ReadmeBlock html={readme.html} toc={readme.toc} />}
        </div>

        <aside className="flex min-w-0 flex-col gap-7">
          {ranks.length > 0 && (
            <Block id="ranks" title={d('ranksTitle')}>
              {ranks.map((r) => (
                <Link
                  key={r.key}
                  href={r.href}
                  className="flex items-baseline justify-between gap-3 border-b border-rule py-3 hover:bg-sunk"
                >
                  <span className="min-w-0 truncate text-sr-small text-ink-2">{r.scope}</span>
                  <span className="flex shrink-0 items-baseline gap-2.5">
                    {r.isNew || r.change ? (
                      <RankMove change={r.change} isNew={r.isNew} scope={r.scope} className="!text-[12px]" />
                    ) : (
                      <span className="text-sr-caption text-ink-3">{d('rankSame')}</span>
                    )}
                    <span className="text-[28px] font-semibold tabular-nums tracking-[-0.02em]">#{fmtInt(r.rank)}</span>
                  </span>
                </Link>
              ))}
            </Block>
          )}
          <BadgeCopy
            src={`${PUBLIC_API_URL}/v1/badge/${repo.owner}/${repo.name}.svg${locale === 'zh' ? '?lang=zh' : ''}`}
            href={`${SITE_URL}/${locale}/repo/${repo.owner}/${repo.name}`}
            repo={repo.fullName}
          />
          {repo.topics.length > 0 && (
            <Block id="topics" title={d('topicsTitle')}>
              <div className="flex flex-wrap gap-1.5">
                {repo.topics.slice(0, 16).map((topic) => (
                  <span key={topic} className="rounded border border-rule px-2 py-0.5 text-[13px] text-ink-2">
                    {topic}
                  </span>
                ))}
              </div>
            </Block>
          )}
          {ownerRepos.length > 0 && (
            <Block id="owner" title={t('ownerRepos', { owner: repo.owner })}>
              <RepoList narrow repos={ownerRepos.slice(0, 5)} />
            </Block>
          )}
        </aside>
      </div>

      <footer className="border-t-2 border-ink pt-4 text-[13px] leading-relaxed text-ink-2">
        <span className="font-semibold text-ink">{d('dataLabel')}</span> {d('dataNote')}
      </footer>
    </PageShell>
  );
}
