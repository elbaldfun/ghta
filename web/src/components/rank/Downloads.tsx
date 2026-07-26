import { getTranslations } from 'next-intl/server';
import type { OS, ReleaseAsset } from '@/lib/data';

const OS_LABEL: Record<OS, string> = {
  macos: 'macOS',
  windows: 'Windows',
  linux: 'Linux',
  android: 'Android',
  ios: 'iOS',
  web: 'Web',
};
const OS_ORDER: OS[] = ['macos', 'windows', 'linux', 'android', 'ios', 'web'];

function fmtSize(bytes?: number): string {
  if (!bytes || bytes <= 0) return '';
  const mb = bytes / (1024 * 1024);
  if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB`;
  if (mb >= 1) return `${Math.round(mb)} MB`;
  return `${Math.max(1, Math.round(bytes / 1024))} KB`;
}

/**
 * The detail-page download section: the app's latest-release binaries grouped by
 * platform, each a direct link to the GitHub asset. Renders nothing when the repo
 * ships no parseable binaries (server-side apps, store-only distribution) — the
 * page falls back to its normal "open on GitHub" affordance instead of showing an
 * empty shell.
 */
export async function Downloads({
  assets,
  fullName,
  locale,
}: {
  assets: ReleaseAsset[];
  fullName: string;
  locale: string;
}) {
  const t = await getTranslations({ locale, namespace: 'rank' });
  if (!assets || assets.length === 0) return null;

  const byOS = new Map<OS, ReleaseAsset[]>();
  for (const a of assets) {
    const list = byOS.get(a.platform) ?? [];
    list.push(a);
    byOS.set(a.platform, list);
  }
  const groups = OS_ORDER.filter((os) => byOS.has(os));

  return (
    <section className="rounded-card border border-border bg-surface p-4">
      <div className="mb-3 flex items-baseline justify-between gap-2">
        <h2 className="text-[13px] font-extrabold">↓ {t('appsDownload')}</h2>
        <a
          href={`https://github.com/${fullName}/releases`}
          target="_blank"
          rel="noopener noreferrer"
          className="font-mono text-[11px] font-semibold text-accent hover:underline"
        >
          {t('detailAllReleases')} →
        </a>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {groups.map((os) => (
          <div key={os} className="rounded-lg border border-border bg-surface2 p-2.5">
            <div className="mb-1.5 font-mono text-[10px] font-bold uppercase tracking-[0.06em] text-accent">
              {OS_LABEL[os]}
            </div>
            <ul className="space-y-1">
              {byOS.get(os)!.slice(0, 8).map((a) => (
                <li key={a.name}>
                  <a
                    href={a.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex items-baseline justify-between gap-2 text-[11.5px] hover:text-accent"
                  >
                    <span className="truncate font-mono text-fg">{a.name}</span>
                    {fmtSize(a.size) && (
                      <span className="shrink-0 font-mono text-[10px] tabular-nums text-muted">{fmtSize(a.size)}</span>
                    )}
                  </a>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
    </section>
  );
}
