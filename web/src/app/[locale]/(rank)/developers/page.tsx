import type { Metadata } from 'next';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import { getDevelopers, type DeveloperBoard } from '@/lib/data';
import { DeveloperCard } from '@/components/rank/DeveloperCard';

export async function generateMetadata({
  params: { locale },
}: {
  params: { locale: string };
}): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'rank' });
  return { title: t('devTitle'), description: t('devSubtitle') };
}

export default async function DevelopersPage({
  params: { locale },
  searchParams,
}: {
  params: { locale: string };
  searchParams: { board?: string };
}) {
  setRequestLocale(locale);
  const t = await getTranslations('rank');

  const board: DeveloperBoard = searchParams.board === 'merit' ? 'merit' : 'rising';
  const domain = 'ai'; // launch focus
  const { items } = await getDevelopers(board, domain, 30);

  const tab = (key: DeveloperBoard, label: string) => (
    <Link
      href={`/developers?board=${key}`}
      className={`rounded-lg px-[13px] py-[7px] text-[12.5px] font-bold ${
        board === key ? 'bg-accent text-accent-fg' : 'text-muted hover:text-fg'
      }`}
    >
      {label}
    </Link>
  );

  return (
    <div className="px-[26px] py-[22px]">
      <div className="mb-1 flex items-baseline gap-2.5">
        <h1 className="font-display text-lg font-extrabold">{t('devTitle')}</h1>
        <span className="text-xs text-muted">{t('devSubtitle')}</span>
      </div>
      <div className="mb-[18px] mt-3 flex items-center gap-1">
        {tab('rising', t('boardRising'))}
        {tab('merit', t('boardMerit'))}
      </div>

      {items.length === 0 ? (
        <div className="py-10 text-center text-[13px] text-muted">{t('loadError')}</div>
      ) : (
        <div className="grid gap-3.5 md:grid-cols-2">
          {items.map((dev, i) => (
            <DeveloperCard key={dev.login} dev={dev} rank={i + 1} showGrowth={board === 'rising'} />
          ))}
        </div>
      )}
    </div>
  );
}
