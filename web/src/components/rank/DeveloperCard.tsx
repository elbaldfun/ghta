import { useTranslations } from 'next-intl';
import { Link } from '@/i18n/navigation';
import type { RankedDeveloper } from '@/lib/data';
import { formatCompact } from '@/lib/rank-data';
import { StarIcon } from './icons';

// Growth intensity relative to the developer's total stars, on the heat ramp —
// same visual language as the repo boards.
function heatColor(dev: RankedDeveloper): string {
  const pct = dev.totalStars > 0 ? (dev.growth / dev.totalStars) * 100 : 0;
  if (pct >= 4) return 'rgb(var(--heat4))';
  if (pct >= 2) return 'rgb(var(--heat3))';
  if (pct >= 0.8) return 'rgb(var(--heat2))';
  if (pct >= 0.2) return 'rgb(var(--heat1))';
  return 'rgb(var(--heat0))';
}

/**
 * One dense leaderboard row for a developer: prominent rank, avatar, identity and
 * flagship repo, with recent growth as a heat-coloured figure. The X handle rides
 * along as context, never the ranking basis.
 */
export function DeveloperCard({
  dev,
  rank,
  highlightGrowth = false,
}: {
  dev: RankedDeveloper;
  rank: number;
  highlightGrowth?: boolean;
}) {
  const t = useTranslations('rank');
  const color = heatColor(dev);

  return (
    <div className="grid grid-cols-[40px_auto_1fr_auto] items-center gap-x-3.5 gap-y-1 border-b border-border px-3 py-3 transition-colors last:border-b-0 hover:bg-surface2 sm:px-4">
      <span
        className="row-span-2 self-start pt-0.5 font-mono text-[16px] font-bold tabular-nums tracking-tight"
        style={{ color: rank <= 3 ? color : undefined }}
      >
        {rank}
      </span>

      {dev.avatarUrl ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img src={dev.avatarUrl} alt="" loading="lazy" className="row-span-2 h-10 w-10 shrink-0 self-start rounded-full" />
      ) : (
        <span className="row-span-2 h-10 w-10 shrink-0 self-start rounded-full bg-surface2" />
      )}

      <div className="min-w-0">
        <div className="flex items-baseline gap-2">
          <a
            href={`https://github.com/${dev.login}`}
            target="_blank"
            rel="noopener noreferrer"
            className="truncate text-[13.5px] font-bold hover:text-accent"
          >
            {dev.name || dev.login}
          </a>
          <span className="truncate text-[11px] text-muted">@{dev.login}</span>
          {dev.company && <span className="hidden truncate text-[11px] text-muted sm:inline">· {dev.company}</span>}
        </div>
        {dev.topRepo && (
          <Link
            href={`/repo/${dev.topRepo}`}
            className="mt-0.5 flex items-center gap-1 truncate text-[12px] font-semibold text-accent hover:underline"
          >
            <StarIcon size={11} className="shrink-0 text-accent2" />
            <span className="truncate">
              {dev.topRepo} · {formatCompact(dev.topRepoStars)}
            </span>
          </Link>
        )}
      </div>

      <div className="text-right">
        {dev.growth > 0 && (
          <span
            className="block font-mono text-[15px] font-bold leading-none tabular-nums tracking-tight"
            style={{ color: highlightGrowth ? color : undefined }}
          >
            +{formatCompact(dev.growth)}
            <span className="text-[9.5px] font-medium text-muted"> ★/{t('devPerDay')}</span>
          </span>
        )}
        <span className="mt-1 block font-mono text-[11px] tabular-nums text-muted">
          {dev.repoCount} {t('devRepos')} · {formatCompact(dev.totalStars)} ★
        </span>
      </div>

      {/* second row under identity: X badge */}
      <span className="col-start-3 -mt-0.5">
        {dev.twitterUsername && (
          <a
            href={`https://x.com/${dev.twitterUsername}`}
            target="_blank"
            rel="noopener noreferrer"
            className="font-mono text-[11px] font-semibold text-accent hover:underline"
          >
            𝕏 @{dev.twitterUsername}
            {dev.followers ? ` · ${formatCompact(dev.followers)}` : ''}
          </a>
        )}
      </span>
    </div>
  );
}
