import { useTranslations } from 'next-intl';
import { Link } from '@/i18n/navigation';
import type { RankedDeveloper } from '@/lib/data';
import { formatCompact } from '@/lib/rank-data';
import { StarIcon } from './icons';

/**
 * One row of a "developers to follow" board: the person, their flagship repo and
 * merit stats, with the self-declared X handle as a trailing badge (context, not
 * the ranking basis).
 */
export function DeveloperCard({
  dev,
  rank,
  showGrowth = false,
}: {
  dev: RankedDeveloper;
  rank: number;
  showGrowth?: boolean;
}) {
  const t = useTranslations('rank');
  return (
    <div className="flex gap-3.5 rounded-card border border-border bg-surface p-4">
      <div className="w-6 shrink-0 pt-0.5 text-center font-display text-sm font-extrabold text-muted">
        {rank}
      </div>
      {dev.avatarUrl && (
        // eslint-disable-next-line @next/next/no-img-element
        <img src={dev.avatarUrl} alt="" loading="lazy" className="h-11 w-11 shrink-0 rounded-full" />
      )}
      <div className="min-w-0 flex-1">
        <div className="flex items-baseline gap-2">
          <a
            href={`https://github.com/${dev.login}`}
            target="_blank"
            rel="noopener noreferrer"
            className="truncate text-sm font-bold hover:text-accent"
          >
            {dev.name || dev.login}
          </a>
          <span className="truncate text-[11px] text-muted">@{dev.login}</span>
        </div>
        {dev.company && <div className="mt-0.5 truncate text-[11px] text-muted">{dev.company}</div>}

        {dev.topRepo && (
          <Link
            href={`/repo/${dev.topRepo}`}
            className="mt-1.5 flex items-center gap-1 truncate text-[12px] font-semibold text-accent hover:underline"
          >
            <StarIcon size={11} className="shrink-0 text-accent2" />
            <span className="truncate">
              {dev.topRepo} · {formatCompact(dev.topRepoStars)}
            </span>
          </Link>
        )}

        <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-muted">
          <span>
            {dev.repoCount} {t('devRepos')}
          </span>
          <span className="flex items-center gap-1">
            <StarIcon size={11} className="text-accent2" />
            {formatCompact(dev.totalStars)}
          </span>
          {showGrowth && dev.growth > 0 && (
            <span className="font-bold text-accent2">
              +{formatCompact(dev.growth)} {t('devPerDay')}
            </span>
          )}
          {dev.twitterUsername && (
            <a
              href={`https://x.com/${dev.twitterUsername}`}
              target="_blank"
              rel="noopener noreferrer"
              className="font-semibold text-accent hover:underline"
            >
              𝕏 @{dev.twitterUsername}
              {dev.followers ? ` · ${formatCompact(dev.followers)}` : ''}
            </a>
          )}
        </div>
      </div>
    </div>
  );
}
