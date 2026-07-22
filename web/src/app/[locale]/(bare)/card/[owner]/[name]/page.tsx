import { notFound } from 'next/navigation';
import { setRequestLocale } from 'next-intl/server';
import { getRepo, getStarHistory } from '@/lib/data';
import { artifactOf, formatCompact, installCmd, langColor } from '@/lib/rank-data';
import { GrowthChart } from '@/components/rank/GrowthChart';

interface Params {
  locale: string;
  owner: string;
  name: string;
}

/**
 * A fixed 1080×1440 (3:4) poster for one repo, meant to be screenshotted for
 * Xiaohongshu / WeChat. No site chrome — the page IS the image.
 *
 * ?growth=N stamps a "this week +N★" badge (the number comes from whoever
 * generates the digest, since the weekly field only fills in after 7 days).
 */
export default async function RepoCardPage({
  params,
  searchParams,
}: {
  params: Params;
  searchParams: { growth?: string; tag?: string };
}) {
  setRequestLocale(params.locale);

  const res = await getRepo(params.owner, params.name);
  if (res.error !== null) notFound();
  const repo = res.data;

  const history = await getStarHistory(repo.owner, repo.name);
  const dot = langColor(repo.language);
  const artifact = artifactOf(repo.language);
  const growth = Number(searchParams.growth);
  const hasGrowth = Number.isFinite(growth) && growth > 0;

  const stats: [string, string][] = [
    ['Stars', formatCompact(repo.stars)],
    ['Forks', formatCompact(repo.forks)],
    ['Issues', formatCompact(repo.openIssues)],
  ];

  return (
    // Force light theme and exact pixels so screenshots are deterministic.
    <div
      data-theme="light"
      className="flex items-center justify-center"
      style={{ width: 1080, height: 1440, background: 'rgb(var(--bg))' }}
    >
      <div className="flex h-full w-full flex-col justify-between p-[72px]">
        <div>
          <div className="flex items-center justify-between">
            <span className="font-display text-[34px] font-extrabold text-accent">StarRank</span>
            {searchParams.tag && (
              <span className="rounded-full bg-accent px-6 py-2 text-[24px] font-bold text-accent-fg">
                {searchParams.tag}
              </span>
            )}
          </div>

          <div className="mt-[64px] flex items-center gap-4">
            <span className="inline-block h-6 w-6 rounded-full" style={{ backgroundColor: dot }} />
            {repo.language && <span className="text-[28px] font-semibold text-muted">{repo.language}</span>}
            {hasGrowth && (
              <span className="ml-auto text-[30px] font-extrabold text-accent2">▲ +{formatCompact(growth)}★</span>
            )}
          </div>

          <h1 className="mt-4 font-display text-[68px] font-extrabold leading-[1.1] text-fg">
            {repo.owner}/<wbr />
            {repo.name}
          </h1>

          {repo.description && (
            <p className="mt-8 line-clamp-4 text-[32px] leading-[1.5] text-muted">{repo.description}</p>
          )}

          {repo.topics.length > 0 && (
            <div className="mt-8 flex flex-wrap gap-3">
              {repo.topics.slice(0, 6).map((topic) => (
                <span
                  key={topic}
                  className="rounded-full bg-surface2 px-5 py-2 text-[24px] font-semibold text-muted"
                >
                  {topic}
                </span>
              ))}
            </div>
          )}

          {history.length >= 2 && (
            <div className="mt-10">
              <GrowthChart points={history} className="h-[300px]" />
            </div>
          )}
        </div>

        <div>
          <div className="flex gap-4">
            {stats.map(([label, value]) => (
              <div key={label} className="flex-1 rounded-card border border-border bg-surface p-6">
                <div className="text-[24px] font-semibold text-muted">{label}</div>
                <div className="text-[44px] font-extrabold text-fg">{value}</div>
              </div>
            ))}
          </div>

          {artifact.has && (
            <div className="mt-5 rounded-card border border-accent bg-surface px-6 py-5">
              <code className="font-mono text-[28px] text-fg">
                {installCmd(repo.owner, repo.name, repo.language)}
              </code>
            </div>
          )}

          <div className="mt-8 flex items-center justify-between text-[26px] text-muted">
            <span>github.com/{repo.owner}/{repo.name}</span>
            <span className="font-semibold text-accent">starrank.dev</span>
          </div>
        </div>
      </div>
    </div>
  );
}
