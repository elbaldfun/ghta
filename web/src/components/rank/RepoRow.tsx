import { useTranslations, useLocale } from 'next-intl';
import { Link } from '@/i18n/navigation';
import type { RepoSummary } from '@/lib/data';
import { artifactOf, formatCompact, langColor } from '@/lib/rank-data';
import { ForkIcon, StarIcon } from './icons';

function heatColor(pct: number): string {
  if (pct >= 4) return 'rgb(var(--heat4))';
  if (pct >= 2) return 'rgb(var(--heat3))';
  if (pct >= 0.8) return 'rgb(var(--heat2))';
  if (pct >= 0.2) return 'rgb(var(--heat1))';
  return 'rgb(var(--heat0))';
}

function updatedLabel(pushedAt: string, locale: string): string {
  const days = Math.max(1, Math.round((Date.now() - Date.parse(pushedAt)) / 86400000));
  if (days >= 30) {
    const m = Math.round(days / 30);
    return locale === 'zh' ? `${m}个月前` : `${m}mo`;
  }
  return locale === 'zh' ? `${days}天前` : `${days}d`;
}

/**
 * One dense row for the corpus leaderboard: rank, identity + description, a few
 * classification chips, and stars/forks — with the daily star gain shown as a
 * heat-coloured figure when we have it, so the home page speaks the same
 * velocity language as the boards while keeping the filters and browse flow.
 */
export function RepoRow({
  repo,
  rank,
  showUpdated = false,
}: {
  repo: RepoSummary;
  rank: number;
  showUpdated?: boolean;
}) {
  const t = useTranslations('rank');
  const locale = useLocale();
  const dot = langColor(repo.language);
  const artifact = artifactOf(repo.language);
  const growth = repo.dailyIncrease ?? 0;
  const pct = growth > 0 && repo.stars > 0 ? (growth / repo.stars) * 100 : 0;
  const color = heatColor(pct);

  return (
    <Link
      href={`/repo/${repo.owner}/${repo.name}`}
      className="grid grid-cols-[36px_1fr_auto] items-center gap-2.5 border-b border-border px-3 py-2.5 transition-colors last:border-b-0 hover:bg-surface2 sm:gap-3.5 sm:px-4"
    >
      <span
        className="font-mono text-[15px] font-bold tabular-nums tracking-tight"
        style={{ color: rank <= 3 ? 'rgb(var(--accent))' : undefined }}
      >
        {rank}
      </span>

      <span className="min-w-0">
        <span className="flex items-center gap-2">
          <span className="inline-block h-[8px] w-[8px] shrink-0 rounded-full" style={{ backgroundColor: dot }} />
          <span className="truncate text-[13.5px] font-semibold">
            <span className="text-muted">{repo.owner}/</span>
            {repo.name}
          </span>
        </span>
        {repo.description && (
          <span className="mt-0.5 block truncate pl-[16px] text-[11.5px] text-muted">{repo.description}</span>
        )}
        <span className="mt-1 flex flex-wrap gap-1 pl-[16px]">
          {repo.language && (
            <span
              className="rounded-[3px] px-1.5 py-px font-mono text-[9.5px] font-semibold text-white opacity-90"
              style={{ backgroundColor: dot }}
            >
              {repo.language}
            </span>
          )}
          {repo.type && repo.type !== 'software' && (
            <span className="rounded-[3px] border border-border bg-surface2 px-1.5 py-px font-mono text-[9.5px] text-muted">
              {repo.type}
            </span>
          )}
          {artifact.has && (
            <span className="rounded-[3px] border border-accent bg-surface2 px-1.5 py-px font-mono text-[9.5px] font-bold text-accent">
              {artifact.registry}
            </span>
          )}
          {repo.topics.slice(0, 2).map((topic) => (
            <span key={topic} className="rounded-[3px] bg-surface2 px-1.5 py-px font-mono text-[9.5px] text-muted">
              {topic}
            </span>
          ))}
        </span>
      </span>

      <span className="flex items-center gap-4 pl-2 text-right">
        {growth > 0 && (
          <span className="hidden font-mono text-[12px] font-bold tabular-nums sm:block" style={{ color }}>
            +{formatCompact(growth)}
            <span className="text-[9px] font-medium text-muted"> ★/{t('devPerDay')}</span>
          </span>
        )}
        {showUpdated ? (
          <span className="w-[64px] font-mono text-[11px] tabular-nums text-muted">
            {updatedLabel(repo.pushedAt, locale)}
          </span>
        ) : (
          <span className="flex w-[64px] items-center justify-end gap-1 font-mono text-[13px] font-bold tabular-nums">
            <StarIcon size={12} className="text-accent2" />
            {formatCompact(repo.stars)}
          </span>
        )}
        <span className="hidden w-[58px] items-center justify-end gap-1 font-mono text-[12px] tabular-nums text-muted sm:flex">
          <ForkIcon size={12} />
          {formatCompact(repo.forks)}
        </span>
      </span>
    </Link>
  );
}
