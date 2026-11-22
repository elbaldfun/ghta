import type { Metadata } from 'next';
import { getTranslations } from 'next-intl/server';
import { permanentRedirect } from 'next/navigation';
import {
  categoryLabel,
  findCategory,
  getCategoryTree,
  getFacets,
  type CategoryNode,
  type TypeFacet,
} from '@/lib/data';
import { boardHref, boardPath, type BoardFilters } from '@/lib/board-url';
import { alternatesFor } from '@/lib/seo';

/**
 * Sends a request to the one URL that shows these filters, so every board has
 * a single indexable address. `current` is the path being served.
 */
export function redirectToCanonical(locale: string, current: string, f: BoardFilters) {
  if (boardPath(f) !== current) permanentRedirect(`/${locale}${boardHref(f)}`);
}

function facetLabel(t: Awaited<ReturnType<typeof getTranslations>>, facets: TypeFacet[], key: string) {
  if (t.has(`typeFacet.${key}`)) return t(`typeFacet.${key}`);
  return facets.find((f) => f.key === key)?.name ?? key;
}

/** What the board is about, for its heading and title; null for all repos. */
function subject(
  t: Awaited<ReturnType<typeof getTranslations>>,
  locale: string,
  tree: CategoryNode[],
  facets: TypeFacet[],
  f: BoardFilters,
): string | null {
  if (f.category) {
    const node = findCategory(tree, f.category);
    return node ? categoryLabel(node, locale) : null;
  }
  if (f.lang) return f.lang;
  if (f.type) return facetLabel(t, facets, f.type);
  return null;
}

export async function boardMetadata(locale: string, f: BoardFilters): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'rank' });
  const [tree, facets] = await Promise.all([
    f.category ? getCategoryTree() : Promise.resolve([]),
    f.type ? getFacets() : Promise.resolve([]),
  ]);
  const name = subject(t, locale, tree, facets, f);
  return {
    title: name ? t('boardTitle', { name }) : t('homeTitle'),
    description: name ? t('boardDescription', { name }) : t('homeSubtitle'),
    alternates: alternatesFor(locale, boardPath(f)),
  };
}
