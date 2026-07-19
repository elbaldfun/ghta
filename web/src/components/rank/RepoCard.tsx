import { useTranslations, useLocale } from 'next-intl';
import { Link } from '@/i18n/navigation';
import type { RepoSummary } from '@/lib/data';
import { artifactOf, formatCompact, homepageHost, langColor } from '@/lib/rank-data';
import { BoxIcon, ClockIcon, ForkIcon, GlobeIcon, ShieldIcon, StarIcon } from './icons';

function updatedLabel(pushedAt: string, locale: string): string {
  const days = Math.max(1, Math.round((Date.now() - Date.parse(pushedAt)) / 86400000));
  if (days >= 30) {
    const m = Math.round(days / 30);
    return locale === 'zh' ? `${m}个月前` : `${m}mo ago`;
  }
  return locale === 'zh' ? `${days}天前` : `${days}d ago`;
}

/**
 * The single repo card used by the home grid, related-repos carousels and any
 * future grid (per the handoff: build ONE component).
 */
export function RepoCard({
  repo,
  showUpdated = false,
  fixedWidth = false,
}: {
  repo: RepoSummary;
  /** true when the active sort is "recently updated" (design swaps footer right side). */
  showUpdated?: boolean;
  /** carousel items are fixed 290px and don't shrink. */
  fixedWidth?: boolean;
}) {
  const t = useTranslations('rank');
  const locale = useLocale();
  const dot = langColor(repo.language);
  const artifact = artifactOf(repo.language);
  const host = homepageHost(repo.homepage);

  return (
    <Link
      href={`/repo/${repo.owner}/${repo.name}`}
      className={`group flex cursor-pointer flex-col gap-2.5 rounded-card border border-border bg-surface p-4 transition-[box-shadow,border-color] hover:border-accent hover:shadow-card-hover ${
        fixedWidth ? 'w-[290px] shrink-0' : ''
      }`}
    >
      <div className="flex items-start justify-between gap-2.5">
        <div className="flex min-w-0 items-center gap-2">
          <span className="inline-block h-[9px] w-[9px] shrink-0 rounded-full" style={{ backgroundColor: dot }} />
          <span className="truncate text-sm font-bold">
            {repo.owner}/{repo.name}
          </span>
        </div>
      </div>

      <p className="line-clamp-2 min-h-[37px] text-[12.5px] leading-normal text-muted">{repo.description}</p>

      <div className="flex flex-wrap gap-1.5">
        {repo.language && (
          <span
            className="rounded-full px-[9px] py-0.5 text-[10px] font-semibold text-white opacity-90"
            style={{ backgroundColor: dot }}
          >
            {repo.language}
          </span>
        )}
        {artifact.has && (
          <span
            title={`${t('artifacts')} · ${artifact.registry} — ${t('artifactTip')}`}
            className="flex items-center gap-1 rounded-full border border-accent bg-surface2 px-[9px] py-0.5 text-[10px] font-bold text-accent"
          >
            <BoxIcon size={10} />
            {artifact.registry}
          </span>
        )}
        {repo.license && (
          <span className="flex items-center gap-1 rounded-full border border-border bg-surface2 px-[9px] py-0.5 text-[10px] font-semibold text-muted">
            <ShieldIcon size={10} />
            {repo.license}
          </span>
        )}
        {repo.topics.slice(0, 2).map((topic) => (
          <span key={topic} className="rounded-full bg-surface2 px-[9px] py-0.5 text-[10px] font-semibold text-muted">
            {topic}
          </span>
        ))}
      </div>

      {host && (
        <span className="flex items-center gap-[5px] truncate text-[11px] font-semibold text-accent">
          <GlobeIcon size={12} className="shrink-0" />
          {host}
        </span>
      )}

      <div className="mt-0.5 flex items-center justify-between border-t border-border pt-2.5">
        <div className="flex gap-3.5">
          <span className="flex items-center gap-1 text-xs font-bold">
            <StarIcon size={13} className="text-accent2" />
            {formatCompact(repo.stars)}
          </span>
          <span className="flex items-center gap-1 text-xs text-muted">
            <ForkIcon size={13} />
            {formatCompact(repo.forks)}
          </span>
        </div>
        {showUpdated && (
          <span className="flex items-center gap-1 text-[11px] font-semibold text-accent">
            <ClockIcon size={12} />
            {updatedLabel(repo.pushedAt, locale)}
          </span>
        )}
      </div>
    </Link>
  );
}
