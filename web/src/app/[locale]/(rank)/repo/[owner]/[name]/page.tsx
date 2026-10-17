import type { Metadata } from 'next';
import { notFound, permanentRedirect } from 'next/navigation';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import {
  categoryLabel,
  getCategoryTree,
  getDomainPeers,
  getOwnerRepos,
  getReadme,
  getRelatedRepos,
  getRepo,
  getStarHistory,
  type RepoSummary,
} from '@/lib/data';
import { artifactOf, formatCompact, homepageHost, installCmd, langColor } from '@/lib/rank-data';
import { GrowthChart } from '@/components/rank/GrowthChart';
import { ReadmeBlock } from '@/components/rank/ReadmeBlock';
import { BoxIcon, GlobeIcon } from '@/components/rank/icons';
import { BackLink } from '@/components/rank/BackLink';
import { PlatformBadges } from '@/components/rank/PlatformBadges';
import { Downloads } from '@/components/rank/Downloads';
import { PageShell } from '@/components/rank/PageShell';
import { CopyCommand } from '@/components/rank/CopyCommand';
import { BadgeCopy } from '@/components/rank/BadgeCopy';
import { boardHref } from '@/lib/board-url';
import { PUBLIC_API_URL, SITE_URL, alternatesFor } from '@/lib/seo';

/** A short list in the sidebar: name, stars gained this week or in all. */
function SideList({ title, href, all, repos }: { title: string; href?: string; all: string; repos: RepoSummary[] }) {
  return (
    <section className="rounded-card border border-border bg-surface p-4">
      <div className="mb-2 flex items-baseline justify-between gap-2">
        <h2 className="min-w-0 truncate text-[12.5px] font-bold text-muted">{title}</h2>
        {href && (
          <Link href={href} className="shrink-0 text-[11.5px] font-bold text-muted hover:text-fg">
            {all} →
          </Link>
        )}
      </div>
      <ul className="-mx-2 flex flex-col">
        {repos.map((r) => (
          <li key={r.fullName}>
            <Link
              href={`/repo/${r.owner}/${r.name}`}
              className="grid grid-cols-[minmax(0,1fr)_auto] items-baseline gap-2 rounded-lg px-2 py-[6px] hover:bg-surface2"
            >
              <span className="min-w-0">
                <span className="block truncate text-[13px] font-semibold">{r.name}</span>
                {r.description && <span className="block truncate text-[11.5px] text-muted">{r.description}</span>}
              </span>
              <span className="font-mono text-[11.5px] font-bold tabular-nums text-muted">★ {formatCompact(r.stars)}</span>
            </Link>
          </li>
        ))}
      </ul>
    </section>
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

  const repoRes = await getRepo(params.owner, params.name);
  if (repoRes.error !== null) notFound();
  const repo = repoRes.data;
  // A renamed repository: the API followed it to its new name, so does the page.
  if (repo.fullName !== `${params.owner}/${params.name}`) {
    permanentRedirect(`/${params.locale}/repo/${repo.fullName}`);
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
  const domain = repo.categoryPath[0];

  // path -> localized label, for the classification breadcrumb + rank chips.
  const pathLabel = new Map<string, string>();
  for (const g of tree) {
    pathLabel.set(g.path, categoryLabel(g, params.locale));
    for (const c of g.children ?? []) pathLabel.set(c.path, categoryLabel(c, params.locale));
  }
  const labelFor = (path: string) => pathLabel.get(path) ?? path.split('/').pop() ?? path;

  // The rank line: "Overall #16 · JavaScript #3 · 前端框架 #1". Category chips
  // link into the filtered leaderboard; fall back to plain category links when
  // the API predates ranks.
  const rankChips: { key: string; label: string; href?: string }[] = repo.ranks.length
    ? repo.ranks.map((r) => ({
        key: `${r.scope}:${r.key ?? ''}`,
        label: `${
          r.scope === 'overall' ? t('rankOverall') : r.scope === 'language' ? r.key! : labelFor(r.key!)
        } #${r.rank.toLocaleString()}`,
        href:
          r.scope === 'category'
            ? boardHref({ category: r.key! })
            : r.scope === 'language'
              ? boardHref({ lang: r.key! })
              : '/github',
      }))
    : repo.categoryPath.slice(0, 2).map((p) => ({
        key: `path:${p}`,
        label: labelFor(p),
        href: boardHref({ category: p }),
      }));

  const dot = langColor(repo.language);
  const artifact = artifactOf(repo.language);
  const host = homepageHost(repo.homepage);

  const stats: { label: string; value: string; accent?: boolean }[] = [
    { label: t('stars'), value: formatCompact(repo.stars), accent: true },
    { label: t('forks'), value: formatCompact(repo.forks) },
    {
      label: t('weeklyIncrease'),
      value: repo.weeklyIncrease === null ? '—' : `+${formatCompact(repo.weeklyIncrease)}`,
    },
    { label: t('issues'), value: formatCompact(repo.openIssues) },
  ];
  const statTiles = stats.map((s) => (
    <div key={s.label} className="flex flex-col justify-center rounded-card border border-border bg-surface p-3.5">
      <div className="text-[11px] font-semibold text-muted">{s.label}</div>
      <div className={`text-[19px] font-extrabold ${s.accent ? 'text-accent' : ''}`}>{s.value}</div>
    </div>
  ));

  return (
    <PageShell className="py-[22px]">
      <BackLink />

      <div className="mb-2 flex flex-wrap items-center gap-2.5">
        <span className="inline-block h-2.5 w-2.5 rounded-full" style={{ backgroundColor: dot }} />
        <h1 className="font-display text-[23px] font-extrabold">
          {repo.owner}/{repo.name}
        </h1>
        {repo.language && (
          <span
            className="rounded-full px-2.5 py-[3px] text-[11px] text-white"
            style={{ backgroundColor: dot }}
          >
            {repo.language}
          </span>
        )}
        {host && (
          <a
            href={repo.homepage!.startsWith('http') ? repo.homepage! : `https://${repo.homepage}`}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-[5px] text-xs font-semibold text-accent"
          >
            <GlobeIcon size={13} />
            {host}
          </a>
        )}
      </div>
      {rankChips.length > 0 && (
        <div className="mb-2.5 flex flex-wrap items-center gap-1.5">
          {rankChips.map((c) =>
            c.href ? (
              <Link
                key={c.key}
                href={c.href}
                className="rounded-full border border-accent/40 bg-accent/5 px-2.5 py-[3px] font-mono text-[11px] font-bold text-accent transition-colors hover:bg-accent/15"
              >
                {c.label}
              </Link>
            ) : (
              <span
                key={c.key}
                className="rounded-full border border-accent/40 bg-accent/5 px-2.5 py-[3px] font-mono text-[11px] font-bold text-accent"
              >
                {c.label}
              </span>
            ),
          )}
        </div>
      )}
      {repo.platforms.length > 0 && (
        <div className="mb-2.5">
          <PlatformBadges
            platforms={repo.platforms}
            inferred={repo.platformSource !== 'asset'}
            inferredLabel={t('appsInferred')}
          />
        </div>
      )}
      {repo.alternativeTo.length > 0 && (
        <p className="mb-2.5 text-[13px] text-muted">
          <span aria-hidden="true">↔ </span>
          {t('appsAltTo')}{' '}
          {repo.alternativeTo.map((a, i) => (
            <span key={a.slug}>
              {i > 0 && '、'}
              <Link href={`/alternatives/${a.slug}`} className="font-bold text-accent hover:underline">
                {a.name}
              </Link>
            </span>
          ))}
        </p>
      )}
      <div className="mb-[22px]">
        {repo.description && <p className="max-w-[640px] text-[13px] text-muted">{repo.description}</p>}
        {repo.topics.length > 0 && (
          <div className="mt-2.5 flex flex-wrap gap-2">
            {repo.topics.slice(0, 12).map((topic) => (
              <span key={topic} className="rounded-full bg-surface2 px-3 py-[5px] text-[11px] font-semibold">
                {topic}
              </span>
            ))}
          </div>
        )}
      </div>

      {history.length >= 2 ? (
        // Numbers on the left, growth trend filling the right.
        <div className="mb-6 grid gap-x-3 gap-y-2.5 md:grid-cols-[220px_1fr] md:grid-rows-[auto_1fr]">
          <div className="hidden text-xs font-bold uppercase tracking-wider text-muted md:block">
            {t('keyMetrics')}
          </div>
          <div className="order-2 text-xs font-bold uppercase tracking-wider text-muted md:order-none">
            {t('growth')}
          </div>
          <div className="order-1 grid grid-cols-2 gap-3 md:order-none md:h-full md:grid-cols-1 md:grid-rows-4">
            {statTiles}
          </div>
          <GrowthChart points={history} className="order-3 min-h-[170px] md:order-none md:h-full" />
        </div>
      ) : (
        <div className="mb-6 grid grid-cols-2 gap-3 sm:grid-cols-4">{statTiles}</div>
      )}

      {repo.releaseAssets.length > 0 && (
        <div className="mb-6">
          <Downloads assets={repo.releaseAssets} fullName={repo.fullName} locale={params.locale} />
        </div>
      )}

      {artifact.has && (
        <div className="mt-4 flex flex-wrap items-center gap-3.5 rounded-card border border-accent bg-surface px-4 py-[13px]">
          <span className="flex items-center gap-[7px] whitespace-nowrap text-[13px] font-bold text-accent">
            <BoxIcon size={16} />
            {t('artifacts')}
          </span>
          <span className="rounded-full border border-border bg-surface2 px-[11px] py-[3px] text-xs font-bold">
            {artifact.registry}
          </span>
          <CopyCommand
            command={installCmd(repo.owner, repo.name, repo.language)}
            repo={`${repo.owner}/${repo.name}`}
          />
        </div>
      )}

      <div className="grid gap-x-6 lg:grid-cols-[minmax(0,1fr)_280px]">
        <div className="min-w-0">{readme && <ReadmeBlock html={readme.html} toc={readme.toc} />}</div>
        <aside className="mt-6 flex min-w-0 flex-col gap-4">
          <BadgeCopy
            src={`${PUBLIC_API_URL}/v1/badge/${repo.owner}/${repo.name}.svg${params.locale === 'zh' ? '?lang=zh' : ''}`}
            href={`${SITE_URL}/${params.locale}/repo/${repo.owner}/${repo.name}`}
            repo={repo.fullName}
          />
          {ownerRepos.length > 0 && (
            <SideList title={t('ownerRepos', { owner: repo.owner })} all={t('seeAll')} repos={ownerRepos} />
          )}
          {peers.length > 0 && domain && (
            <SideList
              title={t('domainPeers', { name: labelFor(domain) })}
              href={boardHref({ category: domain, sort: 'weekly' })}
              all={t('seeAll')}
              repos={peers}
            />
          )}
          {related.length > 0 && <SideList title={t('relatedRepos')} all={t('seeAll')} repos={related} />}
        </aside>
      </div>
    </PageShell>
  );
}
