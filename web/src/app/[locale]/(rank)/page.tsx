import type { Metadata } from 'next';
import { permanentRedirect } from 'next/navigation';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { boardFilters, boardHref, type BoardQuery } from '@/lib/board-url';
import { alternatesFor } from '@/lib/seo';
import { GithubBoard } from '@/components/rank/GithubBoard';

export async function generateMetadata({ params: { locale } }: { params: { locale: string } }): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'rank' });
  return { title: t('homeTitle'), description: t('homeSubtitle'), alternates: alternatesFor(locale, '') };
}

export default async function Home({
  params: { locale },
  searchParams,
}: {
  params: { locale: string };
  searchParams: BoardQuery;
}) {
  setRequestLocale(locale);
  // Filtered views used to live here as query parameters; they have their own
  // pages under /github now.
  if (Object.values(searchParams).some(Boolean)) {
    permanentRedirect(`/${locale}${boardHref(boardFilters({}, searchParams))}`);
  }
  return <GithubBoard locale={locale} filters={{}} />;
}
