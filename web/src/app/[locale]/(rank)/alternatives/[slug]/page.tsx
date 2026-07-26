import type { Metadata } from 'next';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import { getAppsByAlternative } from '@/lib/data';
import { AppCard } from '@/components/rank/AppCard';

// Fetched per request; the backend caches ~1h.
export const dynamic = 'force-dynamic';

export async function generateMetadata({
  params: { locale, slug },
}: {
  params: { locale: string; slug: string };
}): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'rank' });
  const { name } = await getAppsByAlternative(slug);
  return {
    title: t('altTitle', { name }),
    description: t('altSubtitle', { name }),
  };
}

export default async function AlternativePage({
  params: { locale, slug },
}: {
  params: { locale: string; slug: string };
}) {
  setRequestLocale(locale);
  const t = await getTranslations('rank');
  const { items, name } = await getAppsByAlternative(slug);

  return (
    <div className="px-[26px] py-[22px]">
      <Link href="/alternatives" className="mb-4 flex w-fit items-center gap-1.5 text-xs font-semibold text-muted hover:text-fg">
        ← {t('altIndexTitle')}
      </Link>

      <h1 className="font-display text-xl font-extrabold">{t('altTitle', { name })}</h1>
      <p className="mt-1 text-[13px] text-muted">{t('altSubtitle', { name })}</p>

      {items.length === 0 ? (
        <div className="py-10 text-center text-[13px] text-muted">{t('altEmpty')}</div>
      ) : (
        <div className="mt-[18px] overflow-hidden rounded-card border border-border bg-surface">
          {items.map((a) => (
            <AppCard key={a.externalId} app={a} />
          ))}
        </div>
      )}
    </div>
  );
}
