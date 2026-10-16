import type { MetadataRoute } from 'next';
import { routing } from '@/i18n/routing';
import { getCategoryTree, getFacets, searchRepos } from '@/lib/data';
import { listPosts } from '@/lib/blog';
import { FILTER_LANGS } from '@/lib/rank-data';
import { boardPath } from '@/lib/board-url';
import { SITE_URL } from '@/lib/seo';

type Entry = MetadataRoute.Sitemap[number];

/** One entry per locale for a page every locale has, with hreflang alternates. */
function everyLocale(path: string, extra: Omit<Entry, 'url' | 'alternates'> = {}): Entry[] {
  const languages = Object.fromEntries(routing.locales.map((l) => [l, `${SITE_URL}/${l}${path}`]));
  return routing.locales.map((locale) => ({ url: `${SITE_URL}/${locale}${path}`, alternates: { languages }, ...extra }));
}

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const entries: MetadataRoute.Sitemap = [];

  entries.push(...everyLocale('', { changeFrequency: 'daily', priority: 1 }));
  for (const path of ['/github', '/apps', '/models', '/developers', '/breakout', '/ai', '/alternatives']) {
    entries.push(...everyLocale(path, { changeFrequency: 'daily', priority: 0.9 }));
  }

  // The board's landing pages: every domain and leaf, language and form.
  const [tree, facets] = await Promise.all([getCategoryTree(), getFacets()]);
  for (const group of tree) {
    for (const node of [group, ...(group.children ?? [])]) {
      if (node.count > 0) entries.push(...everyLocale(boardPath({ category: node.path }), { changeFrequency: 'daily' }));
    }
  }
  for (const lang of FILTER_LANGS) entries.push(...everyLocale(boardPath({ lang }), { changeFrequency: 'daily' }));
  for (const f of facets) {
    if (f.count > 0) entries.push(...everyLocale(boardPath({ type: f.key }), { changeFrequency: 'daily' }));
  }

  // Blog index and posts. Original content, so worth crawling often.
  for (const locale of routing.locales) {
    entries.push({ url: `${SITE_URL}/${locale}/blog`, changeFrequency: 'weekly' });
    for (const post of await listPosts(locale)) {
      entries.push({
        url: `${SITE_URL}/${locale}/blog/${post.slug}`,
        lastModified: post.date || undefined,
        changeFrequency: 'weekly',
        priority: 0.8,
      });
    }
  }

  // Top repo detail pages (best-effort; skipped if the backend is unreachable).
  const res = await searchRepos({ sort: 'stars', page: 1, perPage: 50 });
  if (res.error === null) {
    for (const repo of res.data.items) {
      entries.push(...everyLocale(`/repo/${repo.owner}/${repo.name}`, { changeFrequency: 'weekly' }));
    }
  }

  return entries;
}
