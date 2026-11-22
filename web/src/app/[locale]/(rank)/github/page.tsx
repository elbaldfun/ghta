import type { Metadata } from 'next';
import { setRequestLocale } from 'next-intl/server';
import { boardFilters, type BoardQuery } from '@/lib/board-url';
import { boardMetadata, redirectToCanonical } from '@/components/rank/GithubBoard';
import { GithubList } from '@/components/rank/GithubList';

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
  return <GithubList locale={locale} filters={filters} />;
}
