import type { Metadata } from 'next';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { getHot, type HotWindow } from '@/lib/data';
import { BreakoutBoard } from '@/components/rank/BreakoutBoard';

// Trending is fetched fresh per request (backend caches ~1h). Static export would
// bake empty data on preview builds where API_URL is unset.
export const dynamic = 'force-dynamic';

export async function generateMetadata({
  params: { locale },
}: {
  params: { locale: string };
}): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'rank' });
  return { title: t('breakoutTitle'), description: t('breakoutSubtitle') };
}

const WINDOWS: HotWindow[] = ['daily', 'weekly', 'monthly'];

export default async function BreakoutPage({
  params: { locale },
  searchParams,
}: {
  params: { locale: string };
  searchParams: { since?: string };
}) {
  setRequestLocale(locale);
  const t = await getTranslations('rank');

  const initial: HotWindow = WINDOWS.includes(searchParams.since as HotWindow)
    ? (searchParams.since as HotWindow)
    : 'weekly';

  const [daily, weekly, monthly] = await Promise.all([
    getHot('daily', 50),
    getHot('weekly', 50),
    getHot('monthly', 50),
  ]);

  return (
    <div className="px-[26px] py-[22px]">
      <div className="mb-4 flex items-baseline gap-2.5">
        <h1 className="font-display text-lg font-extrabold">🔥 {t('breakoutTitle')}</h1>
        <span className="text-xs text-muted">{t('breakoutSubtitle')}</span>
      </div>
      <BreakoutBoard daily={daily} weekly={weekly} monthly={monthly} initial={initial} />
    </div>
  );
}
