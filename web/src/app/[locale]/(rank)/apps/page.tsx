import type { Metadata } from 'next';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import { getApps, getCategoryTree, type AppSort, type OS } from '@/lib/data';
import { AppCard } from '@/components/rank/AppCard';
import { FilterDropdown } from '@/components/rank/FilterDropdown';
import { Pagination } from '@/components/rank/Pagination';
import { PageTabs } from '@/components/rank/PageTabs';
import { PageShell } from '@/components/rank/PageShell';

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


export default async function AppsPage({
  params: { locale },
  searchParams,
}: {
  params: { locale: string };
  searchParams: { os?: string; kind?: string; category?: string; sort?: string; page?: string };
}) {
  setRequestLocale(locale);
  const t = await getTranslations('rank');

  const os = (OSES as string[]).includes(searchParams.os ?? '') ? (searchParams.os as OS) : '';
  const kind = searchParams.kind === 'app' || searchParams.kind === 'cli' ? searchParams.kind : '';
  const sort: AppSort =
    searchParams.sort === 'popular' || searchParams.sort === 'new' ? searchParams.sort : 'hot';
  const page = Math.max(1, Number(searchParams.page) || 1);

  const tree = await getCategoryTree();
  // Validate category against the real top-level domains so it can't be used to
  // spray arbitrary cache keys, and so the label lookup is safe.
  const domains = tree.map((n) => ({ path: n.path, label: locale === 'zh' ? n.name : n.nameEn || n.name }));
  const category = domains.some((d) => d.path === searchParams.category) ? (searchParams.category as string) : '';
  const activeDomain = domains.find((d) => d.path === category);

  const { items, total } = await getApps({ os, kind, category, sort, limit: PER_PAGE, page });

  const qp = (over: Record<string, string>) => {
    const base: Record<string, string> = {};
    if (os) base.os = os;
    if (kind) base.kind = kind;
    if (category) base.category = category;
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
    <PageShell className="py-[22px]">
      <PageTabs items={[{ href: '/apps', label: t('tabDirectory') }, { href: '/alternatives', label: t('navAlternatives') }]} />
      <div className="mb-1 flex items-baseline gap-2.5">
        <h1 className="font-display text-lg font-extrabold">{t('appsTitle')}</h1>
        <span className="text-xs text-muted">{t('appsSubtitle')}</span>
      </div>

      {/* OS filter — the primary dimension */}
      <div className="scrollbar-hide mb-2 mt-3 -ml-[12px] flex items-center gap-1 overflow-x-auto">
        {osTab('', t('appsAll'))}
        {OSES.map((o) => osTab(o, OS_LABEL[o]))}
      </div>

      {/* kind + domain + sort */}
      <div className="mb-[18px] flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <div className="flex items-center gap-1">
            {smallTab(kind === '', `/apps?${qp({ kind: '', page: '1' })}`, t('appsAll'))}
            {smallTab(kind === 'app', `/apps?${qp({ kind: 'app', page: '1' })}`, t('appsKindApp'))}
            {smallTab(kind === 'cli', `/apps?${qp({ kind: 'cli', page: '1' })}`, t('appsKindCli'))}
          </div>
          {domains.length > 0 && (
            <FilterDropdown
              label={activeDomain?.label ?? t('appsAllDomains')}
              active={!!activeDomain}
              items={[
                { key: 'all', label: t('appsAllDomains'), href: `/apps?${qp({ category: '', page: '1' })}`, active: category === '' },
                ...domains.map((d) => ({
                  key: d.path,
                  label: d.label,
                  href: `/apps?${qp({ category: d.path, page: '1' })}`,
                  active: category === d.path,
                })),
              ]}
            />
          )}
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
          {items.map((a) => (
            <AppCard key={a.externalId} app={a} />
          ))}
        </div>
      )}

      {items.length > 0 && (
        <Pagination page={page} perPage={PER_PAGE} totalCount={total} basePath="/apps" cap={BOARD_CAP} params={{ os, kind, category, sort }} />
      )}
    </PageShell>
  );
}
