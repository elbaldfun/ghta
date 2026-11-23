import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import {
  categoryLabel,
  getEcosystem,
  getHeatmap,
  getModels,
  searchRepos,
  type EcoItem,
  type EcoPillar,
  type HeatCell,
  type ModelItem,
  type RepoSummary,
} from '@/lib/data';
import { boardHref } from '@/lib/board-url';
import { fmtParams } from '@/lib/front-page';
import { alternatesFor } from '@/lib/seo';
import { PageShell } from '@/components/rank/PageShell';
import { CopyCommand } from '@/components/rank/CopyCommand';
import { Delta, Kicker, Segmented, fmtInt, fmtTotal } from '@/components/ui';

// Rendered per request (the backend caches upstream): a build with the
// backend unreachable would otherwise bake an empty page.
export const dynamic = 'force-dynamic';

/** The AI scope: the whole agents group plus two leaves of the ML group. */
const AI_LEAVES_FROM_ML = ['ai/llm-serving', 'ai/llm-training'];
const PILLARS = ['skill', 'mcp', 'agent'] as const;
type Pillar = (typeof PILLARS)[number];

export async function generateMetadata({ params: { locale } }: { params: { locale: string } }): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'aiPage' });
  return { alternates: alternatesFor(locale, '/ai'), title: t('title'), description: t('lead') };
}

const repoHref = (fullName: string) => `/repo/${fullName}`;

/** A section opened by a 2px ink rule: title, and an aside on the right. */
function Section({ id, title, aside, children }: { id: string; title: string; aside?: ReactNode; children: ReactNode }) {
  return (
    <section aria-labelledby={id} className="flex min-w-0 flex-col">
      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 border-t-2 border-ink py-3">
        <h2 id={id} className="text-[22px] font-semibold leading-tight tracking-[-0.01em]">
          {title}
        </h2>
        {aside && <div className="text-sr-small text-ink-3">{aside}</div>}
      </div>
      {children}
    </section>
  );
}

function MoreLink({ href, children }: { href: string; children: ReactNode }) {
  return (
    <Link href={href} className="inline-flex min-h-11 items-center text-sr-small font-semibold text-ink underline decoration-1 underline-offset-4 hover:decoration-2 md:min-h-0">
      {children}
    </Link>
  );
}

function Name({ fullName, className = '' }: { fullName: string; className?: string }) {
  const i = fullName.indexOf('/');
  return (
    <span className={`truncate ${className}`}>
      <span className="font-normal text-ink-3">{fullName.slice(0, i + 1)}</span>
      {fullName.slice(i + 1)}
    </span>
  );
}

export default async function AiPage({
  params: { locale },
  searchParams,
}: {
  params: { locale: string };
  searchParams: { eco?: string };
}) {
  setRequestLocale(locale);
  const t = await getTranslations('aiPage');
  const pillar: Pillar = PILLARS.includes(searchParams.eco as Pillar) ? (searchParams.eco as Pillar) : 'skill';

  const [heat, models, ...ecos] = await Promise.all([
    getHeatmap(),
    getModels({ sort: 'hot', limit: 100 }),
    ...PILLARS.map((p) => getEcosystem(p as EcoPillar, 'hot', p === pillar ? 8 : 3)),
  ]);
  const ecoBy = Object.fromEntries(PILLARS.map((p, i) => [p, ecos[i].items])) as Record<Pillar, EcoItem[]>;

  const subs: HeatCell[] = [
    ...(heat.find((g) => g.path === 'agents')?.children ?? []),
    ...(heat.find((g) => g.path === 'ai')?.children ?? []).filter((c) => AI_LEAVES_FROM_ML.includes(c.path)),
  ].sort((a, b) => b.growth - a.growth);
  const label = (c: HeatCell) => categoryLabel(c, locale);
  const top = subs.slice(0, 6);
  // Each column's three movers; the lead's list is the first column's.
  const movers = await Promise.all(
    top.map((c) => searchRepos({ category: c.path, sort: 'daily', page: 1, perPage: 3 })),
  );
  const moversOf = (i: number): RepoSummary[] => movers[i]?.data?.items ?? [];

  const lead = subs[0];
  const second = subs[1];
  const ratio = lead && second && second.growth > 0 ? lead.growth / second.growth : null;
  const maxGrowth = Math.max(1, lead?.growth ?? 0);

  // Four models, one per task, in today's order.
  const modelCards: ModelItem[] = [];
  const seen = new Set<string>();
  for (const m of models.items) {
    const task = m.task ?? 'other';
    if (task === 'other' || seen.has(task)) continue;
    seen.add(task);
    modelCards.push(m);
    if (modelCards.length === 4) break;
  }
  const taskName = (task?: string) => (task && t.has(`task_${task.replace('-', '_')}`) ? t(`task_${task.replace('-', '_')}`) : '');
  const modelSub = (m: ModelItem) =>
    [fmtParams(m.params), m.baseRelation === 'adapter' ? t('adapter') : null, m.gated ? t('gated') : null]
      .filter(Boolean)
      .join(' · ');

  const pillarName = (p: Pillar) => t(`pillar_${p}`);
  const others = PILLARS.filter((p) => p !== pillar);
  const howOf = (e: EcoItem) =>
    [e.isMcp ? t('pillar_mcp') : null, e.isSkill ? t('pillar_skill') : null, e.isAgent ? t('pillar_agent') : null, e.language]
      .filter(Boolean)
      .join(' · ');

  return (
    <PageShell className="flex flex-col gap-8 pb-12 md:gap-12">
      <section className="flex flex-col gap-2 pt-6 md:pt-10">
        <p className="text-sr-caption text-ink-3">{t('kicker')}</p>
        <h1 className="font-news text-[36px] font-medium leading-[1.05] tracking-[-0.02em] md:text-[56px]">{t('title')}</h1>
        <p className="max-w-[760px] text-sr-body text-ink-2 md:text-sr-lead">{t('lead')}</p>
      </section>

      {lead && (
        <section aria-labelledby="ai-lead" className="grid gap-8 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)] lg:gap-14">
          <article className="flex min-w-0 flex-col gap-4 border-t-2 border-ink pt-4">
            <Kicker>{t('leadKicker')}</Kicker>
            <h2
              id="ai-lead"
              className="break-words font-news text-[28px] font-medium leading-[1.15] tracking-[-0.015em] [overflow-wrap:anywhere] md:text-[44px]"
            >
              {ratio != null && ratio >= 1.2
                ? t('leadHead', { name: label(lead), g: fmtInt(lead.growth), second: label(second!), x: ratio.toFixed(1) })
                : t('leadHeadClose', { name: label(lead), g: fmtInt(lead.growth) })}
            </h2>
            <p className="max-w-[720px] text-sr-body text-ink-2 md:text-sr-lead">
              {t('leadBody', { n: fmtInt(lead.repos), per: lead.intensity.toFixed(1) })}
            </p>
            <ol className="border-t border-rule">
              {moversOf(0).map((r, i) => (
                <li key={r.fullName} className="border-b border-rule">
                  <Link href={repoHref(r.fullName)} className="grid grid-cols-[24px_minmax(0,1fr)_auto] items-baseline gap-x-3 py-3 hover:bg-sunk">
                    <span className="text-sr-small tabular-nums text-ink-3">{i + 1}</span>
                    <span className="flex min-w-0 flex-col gap-0.5">
                      <Name fullName={r.fullName} className="text-[15px] font-semibold" />
                      {r.description && <span className="truncate text-[13px] text-ink-2">{r.description}</span>}
                    </span>
                    <Delta value={r.dailyIncrease} className="text-[15px] font-semibold" />
                  </Link>
                </li>
              ))}
            </ol>
            <MoreLink href={boardHref({ category: lead.path })}>{t('toDomain', { name: label(lead) })}</MoreLink>
          </article>

          <aside className="flex min-w-0 flex-col border-t-2 border-ink">
            <div className="flex flex-wrap items-baseline justify-between gap-x-3 py-3">
              <h2 className="text-[18px] font-semibold">{t('subsTitle')}</h2>
              <span className="text-sr-caption text-ink-3">{t('subsSub')}</span>
            </div>
            {subs.map((c, i) => (
              <Link
                key={c.path}
                href={boardHref({ category: c.path })}
                className="grid grid-cols-[minmax(0,1fr)_64px_44px] items-baseline gap-x-2 gap-y-1.5 border-b border-rule py-2.5 hover:bg-sunk"
              >
                <span className="truncate text-sr-small">{label(c)}</span>
                <span className="text-right text-sr-small font-semibold tabular-nums">+{fmtInt(c.growth)}</span>
                <span className="text-right text-sr-caption tabular-nums text-ink-3">{c.intensity.toFixed(1)}</span>
                <span className="col-span-3 h-1 bg-sunk" aria-hidden="true">
                  <span
                    className={`block h-full ${i === 0 ? 'bg-signal-bar' : 'bg-ink'}`}
                    style={{ width: `${Math.max(2, Math.round((c.growth / maxGrowth) * 100))}%` }}
                  />
                </span>
              </Link>
            ))}
          </aside>
        </section>
      )}

      {top.length > 0 && (
        <Section id="by-sub" title={t('bySubTitle')} aside={t('bySubSub')}>
          <div className="grid border-rule md:grid-cols-2 lg:grid-cols-3">
            {top.map((c, i) => (
              <div key={c.path} className="flex min-w-0 flex-col border-b border-rule py-4 md:pr-6 md:even:pr-0 lg:even:pr-6 lg:[&:nth-child(3n)]:pr-0">
                <div className="flex items-baseline justify-between gap-2 pb-2">
                  <span className="truncate text-[16px] font-semibold">{label(c)}</span>
                  <span className="shrink-0 text-sr-caption text-ink-3">{t('projects', { n: fmtInt(c.repos) })}</span>
                </div>
                {moversOf(i).map((r) => (
                  <Link
                    key={r.fullName}
                    href={repoHref(r.fullName)}
                    className="grid grid-cols-[minmax(0,1fr)_auto] items-baseline gap-3 border-t border-rule py-2 hover:bg-sunk"
                  >
                    <span className="flex min-w-0 items-baseline gap-2">
                      <span className="truncate text-sr-small font-semibold">{r.name}</span>
                      <span className="shrink-0 text-sr-caption tabular-nums text-ink-3">{t('stars', { n: fmtTotal(r.stars) })}</span>
                    </span>
                    <Delta value={r.dailyIncrease} className="text-sr-small font-semibold" />
                  </Link>
                ))}
                <Link href={boardHref({ category: c.path })} className="mt-2 text-sr-caption text-ink-2 underline underline-offset-4 hover:text-ink">
                  {t('toDomain', { name: label(c) })}
                </Link>
              </div>
            ))}
          </div>
        </Section>
      )}

      {PILLARS.some((p) => ecoBy[p].length > 0) && (
        <Section
          id="eco"
          title={t('ecoTitle')}
          aside={
            <Segmented
              label={t('ecoTitle')}
              items={PILLARS.map((p) => ({
                key: p,
                label: pillarName(p),
                href: p === 'skill' ? '/ai#eco' : `/ai?eco=${p}#eco`,
                active: p === pillar,
              }))}
            />
          }
        >
          <div
            aria-hidden="true"
            className="hidden grid-cols-[32px_minmax(0,1fr)_minmax(0,360px)_84px_84px] gap-x-6 border-b border-ink py-2 text-sr-caption text-ink-3 md:grid"
          >
            <span>#</span>
            <span>{pillarName(pillar)}</span>
            <span>{t('colGet')}</span>
            <span className="text-right">{t('colToday')}</span>
            <span className="text-right">{t('colStars')}</span>
          </div>
          {ecoBy[pillar].length === 0 && (
            <p className="border-b border-rule py-6 text-sr-small text-ink-2">{t('ecoEmpty', { pillar: pillarName(pillar) })}</p>
          )}
          <ol>
            {ecoBy[pillar].map((e, i) => (
              <li
                key={e.externalId}
                className="grid grid-cols-[24px_minmax(0,1fr)_auto] items-baseline gap-x-3 gap-y-1 border-b border-rule py-3 md:grid-cols-[32px_minmax(0,1fr)_minmax(0,360px)_84px_84px] md:items-center md:gap-x-6"
              >
                <span className="text-sr-small tabular-nums text-ink-3">{i + 1}</span>
                <Link href={repoHref(e.externalId)} className="flex min-w-0 flex-col gap-0.5 hover:underline">
                  <Name fullName={e.externalId} className="text-[15px] font-semibold" />
                  {e.description && <span className="hidden truncate text-[13px] text-ink-2 md:block">{e.description}</span>}
                </Link>
                <span className="col-span-2 col-start-2 row-start-2 flex min-w-0 items-center rounded border border-rule bg-sunk px-2.5 md:col-span-1 md:col-start-auto md:row-start-auto">
                  <CopyCommand command={`git clone https://github.com/${e.externalId}`} repo={e.externalId} />
                </span>
                <Delta value={e.growth} className="col-start-3 row-start-1 text-right text-[15px] font-semibold md:col-start-auto md:row-start-auto" />
                <span className="hidden text-right text-sr-small tabular-nums text-ink-2 md:block">{fmtTotal(e.stars)}</span>
              </li>
            ))}
          </ol>
          <div className="mt-6 grid gap-8 md:grid-cols-2 md:gap-12">
            {others.map((p) => (
              <div key={p} className="flex min-w-0 flex-col">
                <div className="flex items-baseline justify-between gap-3 border-b border-ink pb-2">
                  <span className="text-[16px] font-semibold">{pillarName(p)}</span>
                  <Link href={p === 'skill' ? '/ai#eco' : `/ai?eco=${p}#eco`} className="text-sr-caption text-ink-2 underline underline-offset-4 hover:text-ink">
                    {t('switchTo', { name: pillarName(p) })}
                  </Link>
                </div>
                {ecoBy[p].map((e) => (
                  <Link
                    key={e.externalId}
                    href={repoHref(e.externalId)}
                    className="grid grid-cols-[minmax(0,1fr)_auto] items-baseline gap-3 border-b border-rule py-2.5 hover:bg-sunk"
                  >
                    <span className="flex min-w-0 flex-col gap-0.5">
                      <Name fullName={e.externalId} className="text-sr-small font-semibold" />
                      <span className="truncate text-sr-caption text-ink-3">{howOf(e)}</span>
                    </span>
                    <Delta value={e.growth} className="text-sr-small font-semibold" />
                  </Link>
                ))}
              </div>
            ))}
          </div>
          <div className="mt-3">
            <MoreLink href={`/ecosystem?pillar=${pillar}`}>{t('ecoAll')}</MoreLink>
          </div>
        </Section>
      )}

      {modelCards.length > 0 && (
        <Section id="ai-models" title={t('modelsTitle')} aside={<MoreLink href="/models">{t('modelsAll')}</MoreLink>}>
          <div className="grid grid-cols-2 lg:grid-cols-4">
            {modelCards.map((m, i) => (
              <a
                key={m.externalId}
                href={m.url}
                target="_blank"
                rel="noopener noreferrer"
                className={`flex min-w-0 flex-col gap-1 py-3 pr-3 hover:bg-sunk ${i > 1 ? 'border-t border-rule lg:border-t-0' : ''}`}
              >
                <span className="text-sr-caption text-ink-3">{taskName(m.task)}</span>
                <span className="truncate text-[15px] font-semibold">{m.name}</span>
                <span className="flex items-baseline justify-between gap-2 text-sr-caption">
                  <span className="truncate text-ink-3">{modelSub(m) || m.author}</span>
                  <span className="shrink-0">
                    <Delta value={m.growth} /> <span className="text-ink-3">{t('likesUnit')}</span>
                  </span>
                </span>
              </a>
            ))}
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
