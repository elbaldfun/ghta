import type { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { setRequestLocale } from 'next-intl/server';
import { getFacets } from '@/lib/data';
import { boardFilters, type BoardQuery } from '@/lib/board-url';
import { GithubBoard, boardMetadata, redirectToCanonical } from '@/components/rank/GithubBoard';

type Params = { locale: string; type: string };

export async function generateMetadata({ params: { locale, type } }: { params: Params }): Promise<Metadata> {
  return boardMetadata(locale, { type });
}

export default async function FormPage({ params: { locale, type }, searchParams }: { params: Params; searchParams: BoardQuery }) {
  setRequestLocale(locale);
  // An unknown form is a 404, unless the facets could not be loaded at all.
  const facets = await getFacets();
  if (facets.length > 0 && !facets.some((f) => f.key === type)) notFound();
  const filters = boardFilters({ type }, searchParams);
  redirectToCanonical(locale, `/github/t/${encodeURIComponent(type)}`, filters);
  return <GithubBoard locale={locale} filters={filters} />;
}
