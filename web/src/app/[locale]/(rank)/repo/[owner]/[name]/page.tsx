import type { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import { categoryLabel, getCategoryTree, getReadme, getRelatedRepos, getRepo, getStarHistory } from '@/lib/data';
import { artifactOf, formatCompact, homepageHost, installCmd, langColor } from '@/lib/rank-data';
import { Carousel } from '@/components/rank/Carousel';
import { GrowthChart } from '@/components/rank/GrowthChart';
import { ReadmeBlock } from '@/components/rank/ReadmeBlock';
import { RepoCard } from '@/components/rank/RepoCard';
import { BoxIcon, GlobeIcon } from '@/components/rank/icons';
import { BackLink } from '@/components/rank/BackLink';
import { PlatformBadges } from '@/components/rank/PlatformBadges';
import { Downloads } from '@/components/rank/Downloads';

interface Params {
  locale: string;
  owner: string;
  name: string;
}

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  return { title: `${params.owner}/${params.name}` };
}

export default async function RepoDetailPage({ params }: { params: Params }) {
  setRequestLocale(params.locale);
  const t = await getTranslations('rank');

  const repoRes = await getRepo(params.owner, params.name);
  if (repoRes.error !== null) notFound();
  const repo = repoRes.data;

  const [history, readme, related, tree] = await Promise.all([
    getStarHistory(repo.owner, repo.name),
    getReadme(repo.owner, repo.name),
    getRelatedRepos(repo),
    getCategoryTree(),
  ]);

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
            ? `/?category=${encodeURIComponent(r.key!)}`
            : r.scope === 'language'
              ? `/?lang=${encodeURIComponent(r.key!)}`
              : '/',
      }))
    : repo.categoryPath.slice(0, 2).map((p) => ({
        key: `path:${p}`,
        label: labelFor(p),
        href: `/?category=${encodeURIComponent(p)}`,
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
    // px-7 matches RankHeader's inner container, so content lines up with the brand/nav above.
    <div className="px-7 py-[22px]">
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
          <code className="truncate font-mono text-xs text-muted">
            {installCmd(repo.owner, repo.name, repo.language)}
          </code>
        </div>
      )}

      {readme && <ReadmeBlock html={readme.html} toc={readme.toc} />}

      {related.length > 0 && (
        <div className="mt-[26px]">
          <div className="mb-3 text-xs font-bold uppercase tracking-wider text-muted">{t('relatedRepos')}</div>
          <Carousel ariaLabel={t('relatedRepos')}>
            {related.map((r) => (
              <RepoCard key={r.fullName} repo={r} fixedWidth />
            ))}
          </Carousel>
        </div>
      )}
    </div>
  );
}
