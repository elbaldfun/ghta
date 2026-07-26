import type { Metadata } from 'next';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import { getApps, type AppSort, type OS } from '@/lib/data';
import { formatCompact, langColor } from '@/lib/rank-data';
import { PlatformBadges } from '@/components/rank/PlatformBadges';
import { StarIcon } from '@/components/rank/icons';
import { Pagination } from '@/components/rank/Pagination';

// Fetched per request (backend caches ~1h); avoids baking an empty page on a
// build where the backend is unreachable.
export const dynamic = 'force-dynamic';

const PER_PAGE = 30;
const BOARD_CAP = 300;
const OSES: OS[] = ['macos', 'windows', 'linux', 'android', 'ios', 'web'];
const OS_LABEL: Record<OS, string> = {
  macos: 'macOS',
  windows: 'Windows',
  linux: 'Linux',
  android: 'Android',
  ios: 'iOS',
  web: 'Web',
};

export async function generateMetadata({
  params: { locale },
}: {
  params: { locale: string };
}): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'rank' });
  return { title: t('appsTitle'), description: t('appsSubtitle') };
}

function heatColor(pct: number): string {
  if (pct >= 4) return 'rgb(var(--heat4))';
  if (pct >= 2) return 'rgb(var(--heat3))';
  if (pct >= 0.8) return 'rgb(var(--heat2))';
  if (pct >= 0.2) return 'rgb(var(--heat1))';
  return 'rgb(var(--heat0))';
}

function releaseLabel(iso: string | null | undefined, locale: string): string | null {
  if (!iso) return null;
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return null;
  const days = Math.max(0, Math.round((Date.now() - t) / 86400000));
  if (days === 0) return locale === 'zh' ? '今天' : 'today';
  if (days >= 30) {
    const m = Math.round(days / 30);
    return locale === 'zh' ? `${m}个月前` : `${m}mo`;
  }
  return locale === 'zh' ? `${days}天前` : `${days}d`;
}

export default async function AppsPage({
  params: { locale },
  searchParams,
}: {
  params: { locale: string };
  searchParams: { os?: string; kind?: string; sort?: string; page?: string };
}) {
  setRequestLocale(locale);
  const t = await getTranslations('rank');

  const os = (OSES as string[]).includes(searchParams.os ?? '') ? (searchParams.os as OS) : '';
  const kind = searchParams.kind === 'app' || searchParams.kind === 'cli' ? searchParams.kind : '';
  const sort: AppSort =
    searchParams.sort === 'popular' || searchParams.sort === 'new' ? searchParams.sort : 'hot';
  const page = Math.max(1, Number(searchParams.page) || 1);

  const { items, total } = await getApps({ os, kind, sort, limit: PER_PAGE, page });

  const qp = (over: Record<string, string>) => {
    const base: Record<string, string> = {};
    if (os) base.os = os;
    if (kind) base.kind = kind;
    base.sort = sort;
    return new URLSearchParams({ ...base, ...over }).toString();
  };

  const osTab = (key: OS | '', label: string) => (
    <Link
      key={key || 'all'}
      href={`/apps?${qp({ os: key, page: '1' })}`}
      className={`rounded-lg border px-[11px] py-[6px] text-[12px] font-bold ${
        os === key ? 'border-accent bg-accent/10 text-accent' : 'border-transparent text-muted hover:text-fg'
      }`}
    >
      {label}
    </Link>
  );

  const smallTab = (active: boolean, href: string, label: string) => (
    <Link
      href={href}
      className={`rounded-md px-2.5 py-1 text-[11.5px] font-semibold ${
        active ? 'text-accent' : 'text-muted hover:text-fg'
      }`}
    >
      {label}
    </Link>
  );

  return (
    <div className="px-[26px] py-[22px]">
      <div className="mb-1 flex items-baseline gap-2.5">
        <h1 className="font-display text-lg font-extrabold">{t('appsTitle')}</h1>
        <span className="text-xs text-muted">{t('appsSubtitle')}</span>
      </div>

      {/* OS filter — the primary dimension */}
      <div className="scrollbar-hide mb-2 mt-3 flex items-center gap-1 overflow-x-auto">
        {osTab('', t('appsAll'))}
        {OSES.map((o) => osTab(o, OS_LABEL[o]))}
      </div>

      {/* kind + sort */}
      <div className="mb-[18px] flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-1">
          {smallTab(kind === '', `/apps?${qp({ kind: '', page: '1' })}`, t('appsAll'))}
          {smallTab(kind === 'app', `/apps?${qp({ kind: 'app', page: '1' })}`, t('appsKindApp'))}
          {smallTab(kind === 'cli', `/apps?${qp({ kind: 'cli', page: '1' })}`, t('appsKindCli'))}
        </div>
        <div className="flex items-center gap-1">
          {smallTab(sort === 'hot', `/apps?${qp({ sort: 'hot', page: '1' })}`, t('ecoSortHot'))}
          {smallTab(sort === 'popular', `/apps?${qp({ sort: 'popular', page: '1' })}`, t('ecoSortPopular'))}
          {smallTab(sort === 'new', `/apps?${qp({ sort: 'new', page: '1' })}`, t('appsSortNew'))}
        </div>
      </div>

      {items.length === 0 ? (
        <div className="py-10 text-center text-[13px] text-muted">{t('appsEmpty')}</div>
      ) : (
        <div className="overflow-hidden rounded-card border border-border bg-surface">
          {items.map((a, i) => {
            const [owner = '', name = ''] = a.externalId.split('/');
            const rank = (page - 1) * PER_PAGE + i + 1;
            const pct = a.stars > 0 && a.growth > 0 ? (a.growth / a.stars) * 100 : 0;
            const color = heatColor(pct);
            const dot = langColor(a.language);
            const rel = releaseLabel(a.latestReleaseAt, locale);
            const inferred = a.platformSource !== 'asset';
            return (
              <div
                key={a.externalId}
                className="grid grid-cols-[36px_1fr_auto] items-center gap-2.5 border-b border-border px-3 py-2.5 last:border-b-0 hover:bg-surface2 sm:gap-3.5 sm:px-4"
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
                    <Link href={`/repo/${owner}/${name}`} className="truncate text-[13.5px] font-semibold hover:text-accent">
                      <span className="text-muted">{owner}/</span>
                      {name}
                    </Link>
                    {a.kind === 'cli' && (
                      <span className="shrink-0 rounded-[3px] border border-border bg-surface2 px-1 py-px font-mono text-[9px] font-semibold text-muted">
                        CLI
                      </span>
                    )}
                  </span>
                  {a.description && (
                    <span className="mt-0.5 block truncate pl-[16px] text-[11.5px] text-muted">{a.description}</span>
                  )}
                  <span className="mt-1 flex flex-wrap items-center gap-1 pl-[16px]">
                    <PlatformBadges platforms={a.platforms} inferred={inferred} inferredLabel={t('appsInferred')} />
                    {a.language && (
                      <span
                        className="rounded-[3px] px-1.5 py-px font-mono text-[9.5px] font-semibold text-white opacity-90"
                        style={{ backgroundColor: dot }}
                      >
                        {a.language}
                      </span>
                    )}
                  </span>
                </span>

                <span className="flex items-center gap-3.5 pl-2 text-right sm:gap-5">
                  {a.growth > 0 && (
                    <span className="hidden font-mono text-[12px] font-bold tabular-nums sm:block" style={{ color }}>
                      +{formatCompact(a.growth)}
                      <span className="text-[9px] font-medium text-muted"> ★/{t('devPerDay')}</span>
                    </span>
                  )}
                  <span className="flex w-[58px] items-center justify-end gap-1 font-mono text-[13px] font-bold tabular-nums">
                    <StarIcon size={12} className="text-accent2" />
                    {formatCompact(a.stars)}
                  </span>
                  {rel && (
                    <span className="hidden w-[52px] text-right font-mono text-[10.5px] tabular-nums text-muted md:block">
                      {t('appsLatest')} {rel}
                    </span>
                  )}
                  <a
                    href={`https://github.com/${a.externalId}/releases`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="hidden shrink-0 rounded-md border border-border px-2.5 py-1 text-[11px] font-bold text-fg hover:border-accent hover:text-accent sm:block"
                  >
                    {t('appsDownload')} ↓
                  </a>
                </span>
              </div>
            );
          })}
        </div>
      )}

      {items.length > 0 && (
        <Pagination page={page} perPage={PER_PAGE} totalCount={total} basePath="/apps" cap={BOARD_CAP} params={{ os, kind, sort }} />
      )}
    </div>
  );
}
