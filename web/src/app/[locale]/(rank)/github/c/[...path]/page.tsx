import type { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { setRequestLocale } from 'next-intl/server';
import { findCategory, getCategoryTree } from '@/lib/data';
import { boardFilters, type BoardQuery } from '@/lib/board-url';
import { GithubBoard, boardMetadata, redirectToCanonical } from '@/components/rank/GithubBoard';

type Params = { locale: string; path: string[] };

export async function generateMetadata({ params: { locale, path } }: { params: Params }): Promise<Metadata> {
  return boardMetadata(locale, { category: path.join('/') });
}

export default async function CategoryPage({ params: { locale, path }, searchParams }: { params: Params; searchParams: BoardQuery }) {
  setRequestLocale(locale);
  const category = path.join('/');
  // An unknown path is a 404, unless the tree could not be loaded at all.
  const tree = await getCategoryTree();
  if (tree.length > 0 && !findCategory(tree, category)) notFound();
  const filters = boardFilters({ category }, searchParams);
  redirectToCanonical(locale, `/github/c/${category}`, filters);
  return <GithubBoard locale={locale} filters={filters} />;
}
