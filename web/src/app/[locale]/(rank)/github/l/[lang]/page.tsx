import type { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { setRequestLocale } from 'next-intl/server';
import { boardFilters, langFromSlug, langSlug, type BoardQuery } from '@/lib/board-url';
import { GithubBoard, boardMetadata, redirectToCanonical } from '@/components/rank/GithubBoard';

type Params = { locale: string; lang: string };

export async function generateMetadata({ params: { locale, lang } }: { params: Params }): Promise<Metadata> {
  const name = langFromSlug(lang);
  return name ? boardMetadata(locale, { lang: name }) : {};
}

export default async function LanguagePage({ params: { locale, lang }, searchParams }: { params: Params; searchParams: BoardQuery }) {
  setRequestLocale(locale);
  const name = langFromSlug(lang);
  if (!name) notFound();
  const filters = boardFilters({ lang: name }, searchParams);
  redirectToCanonical(locale, `/github/l/${langSlug(name)}`, filters);
  return <GithubBoard locale={locale} filters={filters} />;
}
