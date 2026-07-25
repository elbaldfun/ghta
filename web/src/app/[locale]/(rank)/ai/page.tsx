import type { Metadata } from 'next';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import {
  categoryLabel,
  findCategory,
  getCategoryTree,
  getDevelopers,
  getEcosystem,
  getTopics,
  type CategoryNode,
} from '@/lib/data';
import { formatCompact } from '@/lib/rank-data';
import { Carousel } from '@/components/rank/Carousel';
import { DeveloperCard } from '@/components/rank/DeveloperCard';
import { EcosystemCard } from '@/components/rank/EcosystemCard';

export async function generateMetadata({
  params: { locale },
}: {
  params: { locale: string };
}): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'rank' });
  return { title: t('aiHubTitle'), description: t('aiHubSubtitle') };
}

/** Section header with a "see all" link to the dedicated page. */
function SectionHead({ title, sub, href, more }: { title: string; sub: string; href: string; more: string }) {
  return (
    <div className="mb-3 flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
      <div className="flex items-baseline gap-2.5">
        <h2 className="font-display text-lg font-extrabold">{title}</h2>
        <span className="text-xs text-muted">{sub}</span>
      </div>
      <Link href={href} className="text-[12px] font-bold text-accent hover:underline">
        {more} →
      </Link>
    </div>
  );
}

export default async function AiHub({ params: { locale } }: { params: { locale: string } }) {
  setRequestLocale(locale);
  const t = await getTranslations('rank');

  const [tree, eco, devs, topics] = await Promise.all([
    getCategoryTree(),
    getEcosystem('all', 'hot', 8),
    getDevelopers('rising', 'ai', 6),
    getTopics('ai', 'hot', 12),
  ]);

  // The AI subtree drives the "which AI area is hot" navigator.
  const aiNode = findCategory(tree, 'ai');
  const subdomains: CategoryNode[] = aiNode?.children ?? [];

  return (
    <div className="px-[26px] py-[22px]">
      <div className="mb-1 flex items-baseline gap-2.5">
        <h1 className="font-display text-xl font-extrabold">{t('aiHubTitle')}</h1>
      </div>
      <p className="mb-6 text-[13px] text-muted">{t('aiHubSubtitle')}</p>

      {/* 1. Which AI area is hot — the spine, straight off our taxonomy. */}
      {subdomains.length > 0 && (
        <section className="mb-8">
          <SectionHead
            title={t('aiAreasTitle')}
            sub={t('aiAreasSubtitle')}
            href="/?category=ai"
            more={t('seeAll')}
          />
          <div className="grid grid-cols-[repeat(auto-fill,minmax(190px,1fr))] gap-2.5">
            {subdomains.map((node) => (
              <Link
                key={node.path}
                href={`/?category=${encodeURIComponent(node.path)}`}
                className="rounded-card border border-border bg-surface px-4 py-3 transition-colors hover:border-accent"
              >
                <div className="truncate text-sm font-bold">{categoryLabel(node, locale)}</div>
                <div className="mt-0.5 text-[11px] text-muted">
                  {formatCompact(node.count)} {t('devRepos')}
                </div>
              </Link>
            ))}
          </div>
        </section>
      )}

      {/* 2. The AI stack: skills / MCP / agents, by momentum. */}
      {eco.length > 0 && (
        <section className="mb-8">
          <SectionHead
            title={t('ecoTitle')}
            sub={t('ecoSubtitle')}
            href="/ecosystem"
            more={t('seeAll')}
          />
          <div className="grid grid-cols-[repeat(auto-fill,minmax(288px,1fr))] gap-3.5">
            {eco.map((item) => (
              <EcosystemCard key={item.externalId} item={item} showGrowth />
            ))}
          </div>
        </section>
      )}

      {/* 3. Who to follow in AI. */}
      {devs.items.length > 0 && (
        <section className="mb-8">
          <SectionHead
            title={t('devTitle')}
            sub={t('devSubtitle')}
            href="/developers"
            more={t('seeAll')}
          />
          <div className="grid gap-3.5 md:grid-cols-2">
            {devs.items.map((dev, i) => (
              <DeveloperCard key={dev.login} dev={dev} rank={i + 1} showGrowth />
            ))}
          </div>
        </section>
      )}

      {/* 4. What's gaining momentum, as topic chips. */}
      {topics.length > 0 && (
        <section>
          <SectionHead
            title={t('topicsTitle')}
            sub={t('topicsSubtitle')}
            href="/topics"
            more={t('seeAll')}
          />
          <Carousel ariaLabel={t('topicsTitle')}>
            {topics.map((tp) => (
              <Link
                key={tp.topic}
                href={`/?q=${encodeURIComponent(tp.topic)}`}
                className="flex w-[190px] shrink-0 flex-col gap-1 rounded-card border border-border bg-surface px-4 py-3 transition-colors hover:border-accent"
              >
                <span className="truncate text-sm font-bold">{tp.topic}</span>
                <span className="text-[11px] font-bold text-accent2">
                  +{formatCompact(tp.growth)} {t('devPerDay')}
                </span>
              </Link>
            ))}
          </Carousel>
        </section>
      )}
    </div>
  );
}
