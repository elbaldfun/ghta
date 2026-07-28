import type { Metadata } from 'next';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import { getDevelopers, type DeveloperBoard } from '@/lib/data';
import { DeveloperCard } from '@/components/rank/DeveloperCard';
import { ErrorState } from '@/components/rank/ErrorState';
import { Pagination } from '@/components/rank/Pagination';
import { PageShell } from '@/components/rank/PageShell';

export async function generateMetadata({
  params: { locale },
}: {
  params: { locale: string };
}): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'rank' });
  return { title: t('devTitle'), description: t('devSubtitle') };
}

// The backend computes a top-500 board per board+domain; page within that.
const PER_PAGE = 30;
const BOARD_CAP = 500;

export default async function DevelopersPage({
  params: { locale },
  searchParams,
}: {
  params: { locale: string };
  searchParams: { board?: string; page?: string };
}) {
  setRequestLocale(locale);
  const t = await getTranslations('rank');

  const board: DeveloperBoard = searchParams.board === 'merit' ? 'merit' : 'rising';
  const domain = 'ai'; // launch focus
  const page = Math.max(1, Number(searchParams.page) || 1);
  const { items, total } = await getDevelopers(board, domain, PER_PAGE, page);

  const tab = (key: DeveloperBoard, label: string) => (
    <Link
      href={`/developers?board=${key}`}
      className={`rounded-lg border px-[13px] py-[7px] text-[12.5px] font-bold ${
        board === key
          ? 'border-accent bg-accent/10 text-accent'
          : 'border-transparent text-muted hover:text-fg'
      }`}
    >
      {label}
    </Link>
  );

  return (
    <PageShell className="py-[22px]">
      <div className="mb-1 flex items-baseline gap-2.5">
        <h1 className="font-display text-lg font-extrabold">{t('devTitle')}</h1>
        <span className="text-xs text-muted">{t('devSubtitle')}</span>
      </div>
      <div className="mb-[18px] mt-3 flex items-center gap-1">
        {tab('rising', t('boardRising'))}
        {tab('merit', t('boardMerit'))}
      </div>

      {items.length === 0 ? (
        <ErrorState />
      ) : (
        <div className="overflow-hidden rounded-card border border-border bg-surface">
          {items.map((dev, i) => (
            <DeveloperCard
              key={dev.login}
              dev={dev}
              rank={(page - 1) * PER_PAGE + i + 1}
              highlightGrowth={board === 'rising'}
            />
          ))}
        </div>
      )}
      {items.length > 0 && (
        <Pagination
          page={page}
          perPage={PER_PAGE}
          totalCount={total}
          basePath="/developers"
          cap={BOARD_CAP}
          params={{ board }}
        />
      )}
    </PageShell>
  );
}
