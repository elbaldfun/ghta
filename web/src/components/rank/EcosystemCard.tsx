import { useTranslations } from 'next-intl';
import { Link } from '@/i18n/navigation';
import type { EcoItem } from '@/lib/data';
import { formatCompact, langColor } from '@/lib/rank-data';
import { StarIcon } from './icons';

/** One repo in the AI-stack board, tagged with its pillar(s). */
export function EcosystemCard({ item, showGrowth = false }: { item: EcoItem; showGrowth?: boolean }) {
  const t = useTranslations('rank');
  const dot = langColor(item.language);
  const [owner = '', name = ''] = item.externalId.split('/');

  const badges: string[] = [];
  if (item.isSkill) badges.push(t('ecoSkill'));
  if (item.isMcp) badges.push('MCP');
  if (item.isAgent) badges.push(t('ecoAgent'));

  return (
    <Link
      href={`/repo/${owner}/${name}`}
      className="flex flex-col gap-2 rounded-card border border-border bg-surface p-4 transition-[box-shadow,border-color] hover:border-accent hover:shadow-card-hover"
    >
      <div className="flex items-start justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2">
          <span
            className="inline-block h-[9px] w-[9px] shrink-0 rounded-full"
            style={{ backgroundColor: dot }}
          />
          <span className="truncate text-sm font-bold">{item.externalId}</span>
        </div>
        {showGrowth && item.growth > 0 && (
          <span className="shrink-0 text-[11px] font-bold text-accent2">
            +{formatCompact(item.growth)} {t('devPerDay')}
          </span>
        )}
      </div>

      <p className="line-clamp-2 min-h-[34px] text-[12.5px] leading-normal text-muted">
        {item.description}
      </p>

      <div className="flex flex-wrap items-center gap-1.5">
        {badges.map((b) => (
          <span
            key={b}
            className="rounded-full border border-accent bg-surface2 px-[9px] py-0.5 text-[10px] font-bold text-accent"
          >
            {b}
          </span>
        ))}
        {item.language && (
          <span
            className="rounded-full px-[9px] py-0.5 text-[10px] font-semibold text-white opacity-90"
            style={{ backgroundColor: dot }}
          >
            {item.language}
          </span>
        )}
      </div>

      <div className="mt-0.5 flex items-center gap-3.5 border-t border-border pt-2.5 text-xs">
        <span className="flex items-center gap-1 font-bold">
          <StarIcon size={13} className="text-accent2" />
          {formatCompact(item.stars)}
        </span>
      </div>
    </Link>
  );
}
