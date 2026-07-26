import { useTranslations } from 'next-intl';
import { Link } from '@/i18n/navigation';
import type { AppItem, OS } from '@/lib/data';
import { formatCompact } from '@/lib/rank-data';
import { AppIcon } from './AppIcon';
import { StarIcon } from './icons';

const OS_SHORT: Record<OS, string> = {
  macos: 'macOS',
  windows: 'Windows',
  linux: 'Linux',
  android: 'Android',
  ios: 'iOS',
  web: 'Web',
};

// Strip GitHub-repo boilerplate so the card leads with the app's name, not its
// repository slug: "obsidian-releases" → "Obsidian", "clash-verge-rev" → "Clash
// Verge Rev". Only the first letter of each token is upper-cased so brand casing
// like "AppFlowy" survives.
const NAME_SUFFIX = /[-_.](releases?|desktop|app|electron|ui|client|gui|monorepo)$/i;
function appName(repoName: string): string {
  const stripped = repoName.replace(NAME_SUFFIX, '') || repoName;
  return stripped
    .split(/[-_]+/)
    .filter(Boolean)
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(' ');
}

/**
 * An app-store-style card: icon, app name, what it does, and download buttons for
 * the visitor's OS up front — with stars/license/repo demoted to a trust line.
 * The framing is "get this app", not "browse this repo".
 */
export function AppCard({ app }: { app: AppItem }) {
  const t = useTranslations('rank');
  const [owner = '', name = ''] = app.externalId.split('/');
  const title = appName(name);
  const downloads = app.downloads ?? [];
  const hasWeb = app.platforms.includes('web');

  return (
    <div className="flex items-start gap-3.5 border-b border-border px-3 py-3.5 transition-colors last:border-b-0 hover:bg-surface2 sm:gap-4 sm:px-4">
      {/* icon */}
      <Link href={`/repo/${owner}/${name}`} className="shrink-0">
        <AppIcon iconUrl={app.iconUrl} owner={owner} title={title} />
      </Link>

      {/* identity + trust */}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <Link href={`/repo/${owner}/${name}`} className="truncate text-[15px] font-extrabold hover:text-accent">
            {title}
          </Link>
          {app.kind === 'cli' && (
            <span className="shrink-0 rounded-[3px] border border-border bg-surface2 px-1 py-px font-mono text-[9px] font-semibold text-muted">
              CLI
            </span>
          )}
        </div>
        {app.description && (
          <p className="mt-0.5 line-clamp-2 max-w-[62ch] text-[12.5px] text-muted">{app.description}</p>
        )}
        {app.alternativeTo && app.alternativeTo.length > 0 && (
          <p className="mt-1 text-[11.5px] text-muted">
            <span aria-hidden="true">↔ </span>
            {t('appsAltTo')}{' '}
            {app.alternativeTo.map((a, i) => (
              <span key={a.slug}>
                {i > 0 && '、'}
                <Link href={`/alternatives/${a.slug}`} className="font-semibold text-accent hover:underline">
                  {a.name}
                </Link>
              </span>
            ))}
          </p>
        )}
        <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 font-mono text-[11px] text-muted">
          <span className="flex items-center gap-1 font-semibold text-fg">
            <StarIcon size={12} className="text-accent2" />
            {formatCompact(app.stars)}
          </span>
          {app.growth > 0 && (
            <span className="text-accent">
              +{formatCompact(app.growth)}/{t('devPerDay')}
            </span>
          )}
          {app.license && <span className="truncate">{app.license}</span>}
          <span className="truncate text-muted/80">
            {owner}/{name}
          </span>
        </div>

        {/* download-first actions: per-OS direct links, shown on every width */}
        <div className="mt-2.5 flex flex-wrap items-center gap-1.5">
          {downloads.map((d) => (
            <a
              key={d.os}
              href={d.url}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-1 rounded-md border border-accent/40 bg-accent/10 px-2.5 py-1 text-[11.5px] font-bold text-accent hover:bg-accent/20"
            >
              {OS_SHORT[d.os]}
              <span aria-hidden="true">↓</span>
            </a>
          ))}
          {hasWeb && app.homepage && (
            <a
              href={app.homepage.startsWith('http') ? app.homepage : `https://${app.homepage}`}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-1 rounded-md border border-accent/40 bg-accent/10 px-2.5 py-1 text-[11.5px] font-bold text-accent hover:bg-accent/20"
            >
              Web
              <span aria-hidden="true">↗</span>
            </a>
          )}
          {downloads.length === 0 && !hasWeb && (
            <Link
              href={`/repo/${owner}/${name}`}
              className="rounded-md border border-border px-2.5 py-1 text-[11.5px] font-bold text-fg hover:border-accent hover:text-accent"
            >
              {t('detailDetailPage')}
            </Link>
          )}
        </div>
      </div>
    </div>
  );
}
