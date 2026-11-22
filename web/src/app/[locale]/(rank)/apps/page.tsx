import { DEV_SHELVES, shelfLabelKey } from '@/lib/shelves';
import type { Metadata } from 'next';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import { getApps, type AppSort, type OS } from '@/lib/data';
import { Pagination } from '@/components/rank/Pagination';
import { PageShell } from '@/components/rank/PageShell';
import { Dropdown, Segmented, fmtInt } from '@/components/ui';
import { AppTile, OS_NAME } from '@/components/apps/AppTile';
import { alternatesFor } from '@/lib/seo';

// Fetched per request (backend caches ~1h); avoids baking an empty page on a
// build where the backend is unreachable.
export const dynamic = 'force-dynamic';

const PER_PAGE = 30;
const BOARD_CAP = 300;
const OSES: OS[] = ['macos', 'windows', 'linux', 'android', 'ios', 'web'];
const SORTS: AppSort[] = ['hot', 'popular', 'new'];
// Consumer shelves = the apps/ branch of taxonomy v3 (slug = path minus "apps/").
// "dev" is a view, not a shelf: the developer-app leaves that left the
// consumer shelves; its subs are full domain-leaf paths.
const SHELF_MAJORS = [
  'productivity', 'creative', 'ai', 'selfhosted', 'network',
  'media', 'social', 'system', 'security', 'games', 'dev',
] as const;
const SHELF_SUBS: Record<string, string[]> = {
  productivity: ['notes', 'todo', 'docs', 'knowledge'],
  creative: ['image', 'video', 'audio', 'whiteboard', 'cad', 'writing'],
  ai: ['assistant', 'local-llm', 'generation', 'builder'],
  selfhosted: ['cloud', 'media-server', 'monitoring', 'tunnel', 'automation'],
  network: ['browser', 'download', 'proxy', 'remote', 'browser-ext'],
  media: ['player', 'music', 'reader'],
  social: ['messaging', 'tools'],
  system: ['screenshot', 'launcher', 'files', 'transfer', 'phone', 'backup'],
  security: ['passwords', 'privacy'],
  games: ['games', 'emulator'],
  dev: DEV_SHELVES,
};
const DEV = 'dev';
/** Full shelf value for a (major, sub) pair: dev subs are already full paths. */
function shelfValue(major: string, sub: string): string {
  return major === DEV ? sub : `${major}/${sub}`;
}
/** Which major a shelf value belongs to ("" for none). */
function majorOf(shelf: string): string {
  if (!shelf) return '';
  if (shelf === DEV || SHELF_SUBS[DEV].includes(shelf)) return DEV;
  return shelf.split('/')[0] ?? '';
}
function validShelf(s: string): boolean {
  if (!s) return true;
  if (s === DEV || SHELF_SUBS[DEV].includes(s)) return true;
  const [major, sub, rest] = s.split('/');
  if (rest !== undefined || major === DEV) return false;
  const subs = SHELF_SUBS[major];
  if (!subs) return false;
  return sub === undefined || subs.includes(sub);
}

export async function generateMetadata({
  params: { locale },
}: {
  params: { locale: string };
}): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'appsPage' });
  return { alternates: alternatesFor(locale, '/apps'), title: t('title'), description: t('lead') };
}

export default async function AppsPage({
  params: { locale },
  searchParams,
}: {
  params: { locale: string };
  searchParams: { os?: string; shelf?: string; sort?: string; page?: string };
}) {
  setRequestLocale(locale);
  const t = await getTranslations('appsPage');
  const tr = await getTranslations('rank');

  const os = (OSES as string[]).includes(searchParams.os ?? '') ? (searchParams.os as OS) : '';
  const shelf = validShelf(searchParams.shelf ?? '') ? (searchParams.shelf ?? '') : '';
  const major = majorOf(shelf);
  const sort: AppSort = (SORTS as string[]).includes(searchParams.sort ?? '') ? (searchParams.sort as AppSort) : 'hot';
  const page = Math.max(1, Math.floor(Number(searchParams.page)) || 1);

  const { items, total } = await getApps({ os, shelf, sort, limit: PER_PAGE, page });
  const now = new Date();

  /** Link to this page with some filters changed; defaults are left out. */
  const href = (over: { os?: string; shelf?: string; sort?: AppSort; page?: number }) => {
    const v = { os, shelf, sort, page, ...over };
    const q = new URLSearchParams();
    if (v.os) q.set('os', v.os);
    if (v.shelf) q.set('shelf', v.shelf);
    if (v.sort !== 'hot') q.set('sort', v.sort);
    if (v.page > 1) q.set('page', String(v.page));
    const s = q.toString();
    return s ? `/apps?${s}` : '/apps';
  };

  const majorLabel = (m: string) => tr(`shelfM_${m}` as never) as string;
  const subLabel = (value: string) => tr(shelfLabelKey(value) as never) as string;
  const sortLabel: Record<AppSort, string> = { hot: t('sortHot'), popular: t('sortPopular'), new: t('sortNew') };
  const sortRule: Record<AppSort, string> = { hot: t('ruleHot'), popular: t('rulePopular'), new: t('ruleNew') };

  // What is being shown, for the count line above the grid.
  const scope = [os ? OS_NAME[os] : '', shelf && shelf !== major ? subLabel(shelf) : major ? majorLabel(major) : '']
    .filter(Boolean)
    .join(' · ');

  return (
    <PageShell className="flex flex-col gap-6 pb-12 md:gap-8">
      <section className="flex flex-col gap-2 pt-6 md:pt-10">
        <p className="text-sr-caption text-ink-3">{t('kicker')}</p>
        <div className="flex flex-wrap items-end justify-between gap-x-4 gap-y-2">
          <h1 className="font-news text-[36px] font-medium leading-[1.05] tracking-[-0.02em] md:text-[56px]">{t('title')}</h1>
          <span className="text-sr-small text-ink-3">{sortRule[sort]}</span>
        </div>
        <p className="max-w-[760px] text-sr-body text-ink-2 md:text-sr-lead">{t('lead')}</p>
      </section>

      <section aria-label={t('filters')} className="flex flex-col gap-3 border-y border-rule py-3 lg:flex-row lg:items-center lg:justify-between">
        <nav aria-label={t('osLabel')} className="flex flex-wrap gap-1.5">
          {[{ key: '', name: t('allOs') }, ...OSES.map((o) => ({ key: o, name: OS_NAME[o] }))].map((x) => (
            <Link
              key={x.key || 'all'}
              href={href({ os: x.key, page: 1 })}
              aria-current={os === x.key ? 'true' : undefined}
              className={`flex h-11 items-center rounded border px-3 text-[13px] md:h-8 ${
                os === x.key ? 'border-ink bg-ink text-paper' : 'border-rule text-ink-2 hover:border-ink-3 hover:text-ink'
              }`}
            >
              {x.name}
            </Link>
          ))}
        </nav>
        <div className="flex flex-wrap items-center gap-2">
          <Dropdown
            label={major ? t('shelfIs', { name: majorLabel(major) }) : t('shelfAll')}
            active={!!major}
            items={[
              { key: 'all', label: t('shelfAllItem'), href: href({ shelf: '', page: 1 }), active: !major },
              ...SHELF_MAJORS.map((m) => ({ key: m, label: majorLabel(m), href: href({ shelf: m, page: 1 }), active: major === m })),
            ]}
          />
          {major && (
            <Dropdown
              label={shelf !== major ? subLabel(shelf) : t('subAll')}
              active={shelf !== major}
              items={[
                { key: 'all', label: t('subAll'), href: href({ shelf: major, page: 1 }), active: shelf === major },
                ...SHELF_SUBS[major].map((s) => {
                  const v = shelfValue(major, s);
                  return { key: v, label: subLabel(v), href: href({ shelf: v, page: 1 }), active: shelf === v };
                }),
              ]}
            />
          )}
          <Segmented
            label={t('sortLabel')}
            items={SORTS.map((s) => ({ key: s, label: sortLabel[s], href: href({ sort: s, page: 1 }), active: sort === s }))}
          />
        </div>
      </section>

      {items.length === 0 ? (
        <div className="flex flex-col items-start gap-3 border-b border-rule py-12">
          <p className="font-news text-[24px] leading-tight">{t('emptyHead')}</p>
          <p className="max-w-[560px] text-sr-small text-ink-2">{t('emptyBody')}</p>
          {(os || shelf) && (
            <Link href={href({ os: '', shelf: '', page: 1 })} className="inline-flex min-h-11 items-center text-sr-small font-semibold underline underline-offset-4 md:min-h-0">
              {t('clear')}
            </Link>
          )}
        </div>
      ) : (
        <>
          <p className="-mb-2 text-sr-caption text-ink-3">
            {scope ? t('countIn', { scope, n: fmtInt(total) }) : t('count', { n: fmtInt(total) })}
          </p>
          <section className="grid grid-cols-1 gap-x-8 gap-y-10 sm:grid-cols-2 lg:grid-cols-3">
            {items.map((a) => (
              <AppTile key={a.externalId} app={a} now={now} />
            ))}
          </section>
          <Pagination tone="sr" page={page} perPage={PER_PAGE} totalCount={total} cap={BOARD_CAP} href={(p) => href({ page: p })} />
        </>
      )}

      <footer className="flex flex-col gap-1 border-t-2 border-ink pt-3 text-[13px] leading-relaxed text-ink-2">
        <span>
          <span className="font-semibold text-ink">{t('rulesLabel')}</span> {t('rules')}
        </span>
        <span>
          {t('altsNote')}{' '}
          <Link href="/alternatives" className="font-semibold text-ink underline underline-offset-4">
            {t('altsLink')}
          </Link>
        </span>
      </footer>
    </PageShell>
  );
}
