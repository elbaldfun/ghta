import type { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import { getReadmeHtml, getRelatedRepos, getRepo, getStarHistory } from '@/lib/data';
import { artifactOf, formatCompact, homepageHost, installCmd, langColor } from '@/lib/rank-data';
import { Carousel } from '@/components/rank/Carousel';
import { GrowthChart } from '@/components/rank/GrowthChart';
import { ReadmeBlock } from '@/components/rank/ReadmeBlock';
import { RepoCard } from '@/components/rank/RepoCard';
import { BackIcon, BoxIcon, GlobeIcon } from '@/components/rank/icons';

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

  const [history, readmeHtml, related] = await Promise.all([
    getStarHistory(repo.owner, repo.name),
    getReadmeHtml(repo.owner, repo.name),
    getRelatedRepos(repo),
  ]);

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
    <div className="mx-auto max-w-[1000px] px-[26px] py-[22px]">
      <Link
        href="/"
        className="mb-4 flex w-fit items-center gap-1.5 text-xs font-semibold text-muted hover:text-fg"
      >
        <BackIcon size={14} />
        {t('back')}
      </Link>

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

      {readmeHtml && <ReadmeBlock html={readmeHtml} />}

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
