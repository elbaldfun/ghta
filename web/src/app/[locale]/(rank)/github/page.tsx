import type { Metadata } from 'next';
import { setRequestLocale } from 'next-intl/server';
import { boardFilters, type BoardQuery } from '@/lib/board-url';
import { GithubBoard, boardMetadata, redirectToCanonical } from '@/components/rank/GithubBoard';

export async function generateMetadata({ params: { locale } }: { params: { locale: string } }): Promise<Metadata> {
  return boardMetadata(locale, {});
}

export default async function GithubPage({
  params: { locale },
  searchParams,
}: {
  params: { locale: string };
  searchParams: BoardQuery;
}) {
  setRequestLocale(locale);
  const filters = boardFilters({}, searchParams);
  redirectToCanonical(locale, '/github', filters);
  return <GithubBoard locale={locale} filters={filters} />;
}
