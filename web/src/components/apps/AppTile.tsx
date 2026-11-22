import { useLocale, useTranslations } from 'next-intl';
import { Link } from '@/i18n/navigation';
import type { AppItem, OS } from '@/lib/data';
import { licenseShort } from '@/lib/repo-facts';
import { fmtInt, fmtTotal } from '@/components/ui';
import { AppIcon } from '@/components/rank/AppIcon';
import { appName } from '@/components/rank/AppCard';
import { AppShot } from './AppShot';

export const OS_NAME: Record<OS, string> = {
  macos: 'macOS',
  windows: 'Windows',
  linux: 'Linux',
  android: 'Android',
  ios: 'iOS',
  web: 'Web',
};

/** Platform glyphs, drawn in the page's ink so they follow the theme. */
function OsGlyph({ os }: { os: OS }) {
  const p = { width: 18, height: 18, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', strokeWidth: 1.6, 'aria-hidden': true };
  switch (os) {
    case 'macos':
      return (
        <svg {...p}>
          <rect x="4" y="5" width="16" height="11" rx="1.5" />
          <path d="M2 19h20" />
        </svg>
      );
    case 'windows':
      return (
        <svg {...p}>
          <rect x="4" y="4" width="7" height="7" />
          <rect x="13" y="4" width="7" height="7" />
          <rect x="4" y="13" width="7" height="7" />
          <rect x="13" y="13" width="7" height="7" />
        </svg>
      );
    case 'linux':
      return (
        <svg {...p}>
          <rect x="3" y="4" width="18" height="16" rx="1.5" />
          <path d="m7 9 3 3-3 3M12 15h5" />
        </svg>
      );
    case 'android':
      return (
        <svg {...p}>
          <path d="M5 11a7 7 0 0 1 14 0v7H5z" />
          <path d="m8 5-1.5-2M16 5l1.5-2" />
        </svg>
      );
    case 'ios':
      return (
        <svg {...p}>
          <rect x="7" y="2.5" width="10" height="19" rx="2" />
          <path d="M11 18.5h2" />
        </svg>
      );
    case 'web':
      return (
        <svg {...p}>
          <circle cx="12" cy="12" r="9" />
          <path d="M3 12h18M12 3a14 14 0 0 1 0 18M12 3a14 14 0 0 0 0 18" />
        </svg>
      );
  }
}

const BTN =
  'flex h-11 shrink-0 items-center rounded border border-ink px-3 text-[13px] font-semibold text-ink hover:bg-ink hover:text-paper md:h-8';

/** "11-19" for this year, "2025-11-19" before it. */
function shortDate(iso: string | null | undefined, now: Date): string | null {
  if (!iso) return null;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return null;
  const md = `${String(d.getUTCMonth() + 1).padStart(2, '0')}-${String(d.getUTCDate()).padStart(2, '0')}`;
  return d.getUTCFullYear() === now.getUTCFullYear() ? md : `${d.getUTCFullYear()}-${md}`;
}

/**
 * One app in the gallery: screenshot, icon and name, what it does, the
 * platforms its release files cover, and a download link to the latest
 * release.
 */
export function AppTile({ app, now }: { app: AppItem; now: Date }) {
  const t = useTranslations('appsPage');
  const locale = useLocale();
  const [owner = '', repo = ''] = app.externalId.split('/');
  const title = appName(repo);
  const tagline = (locale === 'zh' ? app.taglineZh : app.taglineEn) || app.taglineEn || app.description || '';
  const platforms = app.platforms.filter((o) => OS_NAME[o]);
  const released = shortDate(app.latestReleaseAt, now);
  const meta = [licenseShort(app.license), released && t('released', { date: released })].filter(Boolean).join(' · ');
  const alts = app.alternativeTo ?? [];
  // Release files → the latest release; a web app with a homepage → open it;
  // otherwise the detail page, never a dead "Download".
  const action =
    app.hasDownloads || app.latestReleaseAt
      ? { href: `https://github.com/${app.externalId}/releases/latest`, label: t('download'), external: true }
      : app.platforms.includes('web') && /^https?:\/\//.test(app.homepage ?? '')
        ? { href: app.homepage!, label: t('open'), external: true }
        : { href: `/repo/${owner}/${repo}`, label: t('details'), external: false };

  return (
    <article className="flex min-w-0 flex-col gap-3.5">
      <Link href={`/repo/${owner}/${repo}`} tabIndex={-1} aria-hidden="true">
        <AppShot src={app.screenshotUrl} alt="" missing={t('noShot')} />
      </Link>
      <div className="grid grid-cols-[44px_minmax(0,1fr)_auto] items-center gap-3">
        <AppIcon iconUrl={app.iconUrl} owner={owner} title={title} />
        <span className="flex min-w-0 flex-col gap-px">
          <Link href={`/repo/${owner}/${repo}`} className="truncate text-[17px] font-semibold text-ink hover:underline">
            {title}
          </Link>
          <span className="truncate text-[12px] text-ink-3">
            {owner}
            {app.kind === 'cli' && <span className="ml-1.5 rounded border border-rule px-1 font-code text-[10px]">CLI</span>}
          </span>
        </span>
        <span className="flex flex-col items-end gap-px tabular-nums">
          {app.growth > 0 && <span className="text-[15px] font-semibold text-signal">▲{fmtInt(app.growth)}</span>}
          <span className="whitespace-nowrap text-[12px] text-ink-3">{t('stars', { n: fmtTotal(app.stars) })}</span>
        </span>
      </div>
      <p className="line-clamp-2 min-h-[2lh] text-[14px] leading-[1.55] text-ink-2">{tagline}</p>
      {alts.length > 0 && (
        <p className="-mt-1.5 truncate text-[12px] text-ink-3">
          {t('altTo')}{' '}
          {alts.slice(0, 3).map((a, i) => (
            <span key={a.slug}>
              {i > 0 && t('sep')}
              <Link href={`/alternatives/${a.slug}`} className="text-ink-2 underline decoration-rule underline-offset-2 hover:text-ink">
                {a.name}
              </Link>
            </span>
          ))}
        </p>
      )}
      <div className="mt-auto flex items-center justify-between gap-2 border-t border-rule pt-3">
        <span
          className="flex shrink-0 gap-2.5 text-ink-2"
          role="img"
          aria-label={t('runsOn', { list: platforms.map((o) => OS_NAME[o]).join(', ') })}
        >
          {platforms.map((o) => (
            <span key={o} title={OS_NAME[o]} className="flex">
              <OsGlyph os={o} />
            </span>
          ))}
        </span>
        <span className="min-w-0 truncate text-[12px] text-ink-3">{meta}</span>
        {action.external ? (
          <a href={action.href} target="_blank" rel="noopener noreferrer" className={BTN}>
            {action.label}
          </a>
        ) : (
          <Link href={action.href} className={BTN}>
            {action.label}
          </Link>
        )}
      </div>
    </article>
  );
}
