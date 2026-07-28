import type { Metadata } from 'next';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import { getModels, type ModelSort, type ModelTask } from '@/lib/data';
import { ModelRow } from '@/components/rank/ModelRow';
import { PageShell } from '@/components/rank/PageShell';
import { Pagination } from '@/components/rank/Pagination';

// Fetched per request (backend caches ~1h); avoids baking an empty page on a
// build where the backend is unreachable.
export const dynamic = 'force-dynamic';

const PER_PAGE = 30;
const BOARD_CAP = 300;
const TASKS: ModelTask[] = [
  'text-gen', 'multimodal', 'image-gen', 'video', 'audio',
  'embedding', 'vision', 'nlp', 'rl', 'other',
];

export async function generateMetadata({
  params: { locale },
}: {
  params: { locale: string };
}): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'rank' });
  return { title: t('modelsTitle'), description: t('modelsSubtitle') };
}

export default async function ModelsPage({
  params: { locale },
  searchParams,
}: {
  params: { locale: string };
  searchParams: { task?: string; sort?: string; page?: string };
}) {
  setRequestLocale(locale);
  const t = await getTranslations('rank');

  const task = (TASKS as string[]).includes(searchParams.task ?? '') ? (searchParams.task as ModelTask) : '';
  const sort: ModelSort = ['downloads', 'likes', 'new'].includes(searchParams.sort ?? '')
    ? (searchParams.sort as ModelSort)
    : 'hot';
  const page = Math.max(1, Number(searchParams.page) || 1);

  const { items, total } = await getModels({ task, sort, limit: PER_PAGE, page });

  const taskLabel = (k: string) => t(`modelTask_${k.replace('-', '_')}` as never) as string;

  const qp = (over: Record<string, string>) => {
    const base: Record<string, string> = { sort };
    if (task) base.task = task;
    return new URLSearchParams({ ...base, ...over }).toString();
  };

  const taskTab = (key: ModelTask | '', label: string) => (
    <Link
      key={key || 'all'}
      href={`/models?${qp({ task: key, page: '1' })}`}
      className={`whitespace-nowrap rounded-lg border px-[11px] py-[6px] text-[12px] font-bold ${
        task === key ? 'border-accent bg-accent/10 text-accent' : 'border-transparent text-muted hover:text-fg'
      }`}
    >
      {label}
    </Link>
  );

  const sortTab = (key: ModelSort, label: string) => (
    <Link
      key={key}
      href={`/models?${qp({ sort: key, page: '1' })}`}
      className={`rounded-md px-2.5 py-1 text-[11.5px] font-semibold ${
        sort === key ? 'text-accent' : 'text-muted hover:text-fg'
      }`}
    >
      {label}
    </Link>
  );

  return (
    <PageShell className="py-[22px]">
      <div className="mb-1 flex items-baseline gap-2.5">
        <h1 className="font-display text-lg font-extrabold">{t('modelsTitle')}</h1>
        <span className="text-xs text-muted">{t('modelsSubtitle')}</span>
      </div>

      <div className="scrollbar-hide mb-2 mt-3 -ml-[12px] flex items-center gap-1 overflow-x-auto">
        {taskTab('', t('appsAll'))}
        {TASKS.map((k) => taskTab(k, taskLabel(k)))}
      </div>

      <div className="mb-[18px] flex flex-wrap items-center justify-end gap-1">
        {sortTab('hot', t('ecoSortHot'))}
        {sortTab('downloads', t('modelSortDownloads'))}
        {sortTab('likes', t('modelSortLikes'))}
        {sortTab('new', t('appsSortNew'))}
      </div>

      {items.length === 0 ? (
        <div className="py-10 text-center text-[13px] text-muted">{t('modelsEmpty')}</div>
      ) : (
        <div className="overflow-hidden rounded-card border border-border bg-surface">
          <div className="grid grid-cols-[36px_1fr_auto] items-center gap-2.5 border-b border-border px-3 py-2 font-mono text-[10px] uppercase tracking-[0.08em] text-muted sm:grid-cols-[40px_1fr_110px_96px_80px] sm:gap-3.5 sm:px-4">
            <div>#</div>
            <div>{t('modelColModel')}</div>
            <div className="text-right">{t('mapColVelocity')}</div>
            <div className="hidden text-right sm:block">{t('modelDl30d')}</div>
            <div className="hidden text-right sm:block">{t('modelColLikes')}</div>
          </div>
          {items.map((m, i) => (
            <ModelRow
              key={m.externalId}
              model={m}
              rank={(page - 1) * PER_PAGE + i + 1}
              taskLabel={m.task ? taskLabel(m.task) : undefined}
            />
          ))}
        </div>
      )}

      {items.length > 0 && (
        <Pagination page={page} perPage={PER_PAGE} totalCount={total} basePath="/models" cap={BOARD_CAP} params={{ task, sort }} />
      )}
    </PageShell>
  );
}
