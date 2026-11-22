import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import { getModels, type ModelItem, type ModelSort, type ModelTask } from '@/lib/data';
import { fmtParams } from '@/lib/front-page';
import { daysSince } from '@/lib/repo-facts';
import { alternatesFor } from '@/lib/seo';
import { PageShell } from '@/components/rank/PageShell';
import { Pagination } from '@/components/rank/Pagination';
import { Delta, Kicker, Segmented, StatBlock, fmtInt, fmtTotal } from '@/components/ui';

// Fetched per request (backend caches ~1h); avoids baking an empty page on a
// build where the backend is unreachable.
export const dynamic = 'force-dynamic';

const PER_PAGE = 30;
const BOARD_CAP = 300;
const TASKS: ModelTask[] = ['text-gen', 'multimodal', 'image-gen', 'video', 'audio', 'embedding', 'vision', 'nlp', 'rl', 'other'];
// "By task" shows the tasks people browse; rl and other stay in the filter.
const BY_TASK: ModelTask[] = ['text-gen', 'multimodal', 'image-gen', 'video', 'audio', 'embedding', 'vision', 'nlp'];
const SORTS: ModelSort[] = ['hot', 'downloads', 'likes', 'new'];

export async function generateMetadata({ params: { locale } }: { params: { locale: string } }): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'modelsPage' });
  return { alternates: alternatesFor(locale, '/models'), title: t('title'), description: t('lead') };
}

/** File formats worth naming: the ones that run locally without conversion. */
const FORMAT_LABEL: Record<string, string> = { gguf: 'GGUF', onnx: 'ONNX', mlx: 'MLX' };

function Section({ id, title, aside, children }: { id: string; title: string; aside?: ReactNode; children: ReactNode }) {
  return (
    <section aria-labelledby={id} className="flex min-w-0 flex-col">
      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 border-t-2 border-ink py-3">
        <h2 id={id} className="text-[22px] font-semibold leading-tight tracking-[-0.01em]">
          {title}
        </h2>
        {aside && <span className="text-sr-small text-ink-3">{aside}</span>}
      </div>
      {children}
    </section>
  );
}

export default async function ModelsPage({
  params: { locale },
  searchParams,
}: {
  params: { locale: string };
  searchParams: { task?: string; sort?: string; page?: string };
}) {
  setRequestLocale(locale);
  const t = await getTranslations('modelsPage');
  const now = Date.now();

  const task = (TASKS as string[]).includes(searchParams.task ?? '') ? (searchParams.task as ModelTask) : '';
  const sort: ModelSort = SORTS.includes(searchParams.sort as ModelSort) ? (searchParams.sort as ModelSort) : 'hot';
  const page = Math.max(1, Number(searchParams.page) || 1);
  // The front view (no filter, first page) carries the lead story, the
  // downloads list beside it and the per-task columns.
  const front = !task && sort === 'hot' && page === 1;

  const [board, downloads, ...byTask] = await Promise.all([
    getModels({ task, sort, limit: PER_PAGE, page }),
    front ? getModels({ sort: 'downloads', limit: 6 }) : Promise.resolve({ items: [], total: 0 }),
    ...(front ? BY_TASK.map((k) => getModels({ task: k, sort: 'hot', limit: 2 })) : []),
  ]);
  const { items, total } = board;

  const taskName = (k?: string) => (k ? t(`task_${k.replace('-', '_')}`) : '—');
  const href = (over: { task?: string; sort?: string; page?: number }) => {
    const q = new URLSearchParams();
    const tk = over.task ?? task;
    const so = over.sort ?? sort;
    if (tk) q.set('task', tk);
    if (so !== 'hot') q.set('sort', so);
    if (over.page && over.page > 1) q.set('page', String(over.page));
    const qs = q.toString();
    return qs ? `/models?${qs}` : '/models';
  };

  const ago = (iso?: string | null) => {
    const d = daysSince(iso ?? null, now);
    if (d == null) return null;
    if (d < 1) return t('ageToday');
    if (d < 60) return t('ageDays', { n: d });
    if (d < 730) return t('ageMonths', { n: Math.round(d / 30) });
    return t('ageYears', { n: Math.round(d / 365) });
  };
  const baseName = (m: ModelItem) => m.baseModel?.[0]?.split('/').pop();
  /** What the model is made from: "Quantized from X", "Adapter on X". */
  const origin = (m: ModelItem) => {
    const base = baseName(m);
    if (!base) return null;
    // A base with no (or an unknown) relation still names where it came from.
    return m.baseRelation && t.has(`rel_${m.baseRelation}`) ? t(`rel_${m.baseRelation}`, { base }) : t('rel_other', { base });
  };
  const tags = (m: ModelItem) => [
    ...(m.quantFormats ?? []).filter((f) => FORMAT_LABEL[f]).map((f) => FORMAT_LABEL[f]),
    // With a base model the origin text already says "Adapter on X".
    ...(m.baseRelation === 'adapter' && !baseName(m) ? [t('adapter')] : []),
  ];

  const lead = front ? items[0] : undefined;
  const leadAge = lead ? daysSince(lead.createdAt ?? null, now) : null;
  const sortLabel: Record<ModelSort, string> = {
    hot: t('sortHot'),
    downloads: t('sortDownloads'),
    likes: t('sortLikes'),
    new: t('sortNew'),
  };
  const figure = (m: ModelItem) =>
    sort === 'downloads' ? fmtTotal(m.downloads30d) : sort === 'likes' ? fmtInt(m.likes) : null;

  return (
    <PageShell className="flex flex-col gap-8 pb-12 md:gap-12">
      <section className="flex flex-col gap-2 pt-6 md:pt-10">
        <p className="text-sr-caption text-ink-3">{t('kicker')}</p>
        <h1 className="font-news text-[36px] font-medium leading-[1.05] tracking-[-0.02em] md:text-[56px]">{t('title')}</h1>
        <p className="max-w-[760px] text-sr-body text-ink-2 md:text-sr-lead">{t('lead')}</p>
      </section>

      <section aria-label={t('filters')} className="-mt-4 flex flex-col gap-3 border-y border-rule py-3 md:-mt-6 md:flex-row md:items-center md:justify-between">
        <nav aria-label={t('taskLabel')} className="flex flex-wrap gap-1.5">
          {[{ key: '', name: t('all') }, ...TASKS.map((k) => ({ key: k, name: taskName(k) }))].map((x) => (
            <Link
              key={x.key || 'all'}
              href={href({ task: x.key, page: 1 })}
              aria-current={task === x.key ? 'true' : undefined}
              className={`flex h-11 items-center rounded border px-3 text-[13px] md:h-8 ${
                task === x.key ? 'border-ink bg-ink text-paper' : 'border-rule text-ink-2 hover:border-ink-3 hover:text-ink'
              }`}
            >
              {x.name}
            </Link>
          ))}
        </nav>
        <div className="-mx-4 overflow-x-auto px-4 md:mx-0 md:overflow-visible md:px-0">
          <Segmented
            label={t('sortLabel')}
            items={SORTS.map((s) => ({ key: s, label: sortLabel[s], href: href({ sort: s, page: 1 }), active: sort === s }))}
          />
        </div>
      </section>

      {lead && (
        <section className="grid gap-8 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)] lg:gap-14">
          <article className="flex min-w-0 flex-col gap-4 border-t-2 border-ink pt-4">
            <Kicker>{t('leadKicker')}</Kicker>
            <h2 className="break-words font-news text-[28px] font-medium leading-[1.15] tracking-[-0.015em] [overflow-wrap:anywhere] md:text-[40px]">
              {leadAge != null && leadAge <= 30
                ? t('leadHeadNew', { name: lead.name, days: leadAge + 1, g: fmtInt(lead.growth) })
                : t('leadHead', { name: lead.name, g: fmtInt(lead.growth) })}
            </h2>
            <p className="max-w-[720px] text-sr-body text-ink-2 md:text-sr-lead">
              {t('leadBody', {
                author: lead.author ?? lead.externalId.split('/')[0],
                task: taskName(lead.task),
                likes: fmtInt(lead.likes),
                dl: fmtInt(lead.downloads30d),
              })}
            </p>
            <div className="grid grid-cols-2 gap-y-4 border-t border-rule pt-4 md:grid-cols-4">
              <StatBlock label={t('statToday')} value={<Delta value={lead.growth} />} size="md" />
              <StatBlock label={t('statLikes')} value={fmtInt(lead.likes)} size="md" />
              <StatBlock label={t('statDl')} value={fmtTotal(lead.downloads30d)} size="md" />
              <StatBlock label={t('statParams')} value={fmtParams(lead.params) ?? '—'} size="md" />
            </div>
            <dl className="grid grid-cols-2 gap-x-6 border-t border-rule md:grid-cols-3">
              {[
                [t('specTask'), lead.pipelineTag ?? taskName(lead.task)],
                [t('specLicense'), lead.license ?? '—'],
                [t('specFormat'), (lead.quantFormats ?? []).join(' / ') || '—'],
                [t('specCreated'), lead.createdAt ? `${lead.createdAt.slice(5, 10)} · ${ago(lead.createdAt)}` : '—'],
                [t('specAccess'), lead.gated ? t('gated') : t('open')],
                [t('specOrigin'), origin(lead) ?? (lead.derivatives ? t('derivatives', { n: fmtInt(lead.derivatives) }) : t('original'))],
              ].map(([k, v]) => (
                <div key={k} className="flex min-w-0 flex-col gap-0.5 border-b border-rule py-2.5">
                  <dt className="text-sr-caption text-ink-3">{k}</dt>
                  <dd className="break-words text-sr-small">{v}</dd>
                </div>
              ))}
            </dl>
            <a href={lead.url} target="_blank" rel="noopener noreferrer" className="self-start text-sr-small font-semibold underline underline-offset-4">
              {t('openHf')} ↗
            </a>
          </article>

          {downloads.items.length > 0 && (
            <aside className="flex min-w-0 flex-col border-t-2 border-ink">
              <h2 className="py-3 text-[18px] font-semibold">{t('dlTitle')}</h2>
              {downloads.items.map((m, i) => (
                <a
                  key={m.externalId}
                  href={m.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="grid grid-cols-[20px_minmax(0,1fr)_auto] items-baseline gap-x-3 border-b border-rule py-2.5 hover:bg-sunk"
                >
                  <span className="text-sr-small tabular-nums text-ink-3">{i + 1}</span>
                  <span className="flex min-w-0 flex-col gap-0.5">
                    <span className="truncate text-sr-small font-semibold">
                      <span className="font-normal text-ink-3">{m.author ? `${m.author}/` : ''}</span>
                      {m.name}
                    </span>
                    <span className="truncate text-sr-caption text-ink-3">
                      {[taskName(m.task), fmtParams(m.params)].filter(Boolean).join(' · ')}
                    </span>
                  </span>
                  <span className="text-sr-small font-semibold tabular-nums">{fmtTotal(m.downloads30d)}</span>
                </a>
              ))}
              <p className="pt-3 text-sr-caption leading-relaxed text-ink-3">{t('dlNote')}</p>
            </aside>
          )}
        </section>
      )}

      <Section
        id="board"
        title={front ? t('boardTitleFront', { n: items.length }) : `${task ? taskName(task) : t('all')} · ${sortLabel[sort]}`}
        aside={t('boardSub')}
      >
        {items.length === 0 ? (
          <p className="py-10 text-sr-body text-ink-2">{t('empty')}</p>
        ) : (
          <>
            <div
              aria-hidden="true"
              className="hidden grid-cols-[32px_minmax(0,1fr)_96px_64px_150px_96px_84px_84px] gap-x-5 border-b border-ink py-2 text-sr-caption text-ink-3 lg:grid"
            >
              <span>#</span>
              <span>{t('colModel')}</span>
              <span>{t('colTask')}</span>
              <span className="text-right">{t('colParams')}</span>
              <span>{t('colFormat')}</span>
              <span className="text-right">{t('colDl')}</span>
              <span className="text-right">{t('colLikes')}</span>
              <span className="text-right">{t('colToday')}</span>
            </div>
            <ol>
              {items.map((m, i) => {
                const tg = tags(m);
                const from = origin(m);
                const age = ago(m.createdAt);
                return (
                  <li key={m.externalId} className="border-b border-rule">
                    <a
                      href={m.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="grid grid-cols-[22px_minmax(0,1fr)_auto] items-baseline gap-x-3 gap-y-1 py-3 hover:bg-sunk lg:grid-cols-[32px_minmax(0,1fr)_96px_64px_150px_96px_84px_84px] lg:items-center lg:gap-x-5"
                    >
                      <span className="text-sr-small tabular-nums text-ink-3">{(page - 1) * PER_PAGE + i + 1}</span>
                      <span className="flex min-w-0 flex-col gap-0.5">
                        <span className="flex min-w-0 items-baseline gap-2">
                          <span className="truncate text-[15px] font-semibold">
                            <span className="font-normal text-ink-3">{m.author ? `${m.author}/` : ''}</span>
                            {m.name}
                          </span>
                          {m.gated && (
                            <span className="shrink-0 rounded border border-ink-3 px-1.5 text-[11px] text-ink-2">{t('gated')}</span>
                          )}
                        </span>
                        <span className="hidden truncate text-[13px] text-ink-3 lg:block">
                          {[m.pipelineTag, m.license, age && t('released', { age })].filter(Boolean).join(' · ')}
                        </span>
                        {/* Phones: one line of facts under the name. */}
                        <span className="truncate text-[12px] text-ink-3 lg:hidden">
                          {[taskName(m.task), fmtParams(m.params), ...tg, from, sort === 'downloads' ? null : t('dlShort', { n: fmtTotal(m.downloads30d) })]
                            .filter(Boolean)
                            .join(' · ')}
                        </span>
                      </span>
                      {/* Phones show the figure the board is sorted by. */}
                      <span className="text-right text-[15px] font-semibold lg:hidden">
                        {figure(m) ?? <Delta value={m.growth} />}
                      </span>
                      <span className="hidden truncate text-sr-small text-ink-2 lg:block">{taskName(m.task)}</span>
                      <span className="hidden text-right text-sr-small tabular-nums text-ink-2 lg:block">{fmtParams(m.params) ?? '—'}</span>
                      <span className="hidden min-w-0 flex-wrap items-center gap-1 lg:flex">
                        {tg.map((x) => (
                          <span key={x} className="rounded border border-ink-3 px-1.5 text-[11px] text-ink">
                            {x}
                          </span>
                        ))}
                        {from && <span className="truncate text-[12px] text-ink-3">{from}</span>}
                        {!tg.length && !from && <span className="text-[12px] text-ink-3">—</span>}
                      </span>
                      <span className="hidden text-right text-sr-small tabular-nums text-ink-2 lg:block">
                        {m.downloads30d ? fmtTotal(m.downloads30d) : '—'}
                      </span>
                      <span className="hidden text-right text-sr-small tabular-nums text-ink-2 lg:block">{fmtInt(m.likes)}</span>
                      <span className="hidden text-right text-[15px] font-semibold lg:block">
                        <Delta value={m.growth} />
                      </span>
                    </a>
                  </li>
                );
              })}
            </ol>
            <Pagination tone="sr" page={page} perPage={PER_PAGE} totalCount={total} cap={BOARD_CAP} href={(p) => href({ page: p })} />
          </>
        )}
      </Section>

      {front && byTask.some((r) => r.items.length > 0) && (
        <Section id="bytask" title={t('byTaskTitle')} aside={t('byTaskSub')}>
          <div className="grid md:grid-cols-2 lg:grid-cols-4">
            {BY_TASK.map((k, i) =>
              byTask[i]?.items.length ? (
                <div key={k} className="flex min-w-0 flex-col border-b border-rule py-3 md:pr-6">
                  <div className="flex items-baseline justify-between gap-2 pb-1.5">
                    <span className="text-[16px] font-semibold">{taskName(k)}</span>
                    <Link href={href({ task: k, page: 1 })} className="text-sr-caption text-ink-2 underline underline-offset-4 hover:text-ink">
                      {t('all')} →
                    </Link>
                  </div>
                  {byTask[i].items.map((m) => (
                    <a
                      key={m.externalId}
                      href={m.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="grid grid-cols-[minmax(0,1fr)_auto] items-baseline gap-3 border-t border-rule py-2 hover:bg-sunk"
                    >
                      <span className="flex min-w-0 flex-col gap-0.5">
                        <span className="truncate text-sr-small font-semibold">{m.name}</span>
                        <span className="truncate text-sr-caption text-ink-3">
                          {[origin(m) ?? m.author, fmtParams(m.params), m.gated ? t('gated') : null].filter(Boolean).join(' · ')}
                        </span>
                      </span>
                      <Delta value={m.growth} className="text-sr-small font-semibold" />
                    </a>
                  ))}
                </div>
              ) : null,
            )}
          </div>
        </Section>
      )}

      <footer className="flex flex-col gap-1 border-t-2 border-ink pt-3 text-[13px] leading-relaxed text-ink-2">
        <span>
          <span className="font-semibold text-ink">{t('rulesLabel')}</span> {t('rules')}
        </span>
        <span className="text-ink-3">{t('dataNote')}</span>
      </footer>
    </PageShell>
  );
}
