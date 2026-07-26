import type { Metadata } from 'next';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import { getAltTargets } from '@/lib/data';
import { PageTabs } from '@/components/rank/PageTabs';

export const dynamic = 'force-dynamic';

export async function generateMetadata({
  params: { locale },
}: {
  params: { locale: string };
}): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'rank' });
  return { title: t('altIndexTitle'), description: t('altIndexSubtitle') };
}

export default async function AlternativesIndexPage({ params: { locale } }: { params: { locale: string } }) {
  setRequestLocale(locale);
  const t = await getTranslations('rank');
  const targets = await getAltTargets();

  return (
    <div className="px-[26px] py-[22px]">
      <PageTabs items={[{ href: '/apps', label: t('tabDirectory') }, { href: '/alternatives', label: t('navAlternatives') }]} />
      <div className="mb-1 flex items-baseline gap-2.5">
        <h1 className="font-display text-lg font-extrabold">{t('altIndexTitle')}</h1>
        <span className="text-xs text-muted">{t('altIndexSubtitle')}</span>
      </div>

      {targets.length === 0 ? (
        <div className="py-10 text-center text-[13px] text-muted">{t('altEmpty')}</div>
      ) : (
        <div className="mt-4 grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4">
          {targets.map((tg) => (
            <Link
              key={tg.slug}
              href={`/alternatives/${tg.slug}`}
              className="flex items-center justify-between gap-2 rounded-lg border border-border bg-surface px-3 py-2.5 transition-colors hover:border-accent"
            >
              <span className="min-w-0">
                <span className="block truncate text-[13px] font-bold">{tg.name}</span>
                {tg.kind && <span className="block truncate font-mono text-[10px] text-muted">{tg.kind}</span>}
              </span>
              <span className="shrink-0 rounded-full bg-surface2 px-2 py-0.5 font-mono text-[11px] font-bold tabular-nums text-accent">
                {tg.count}
              </span>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
