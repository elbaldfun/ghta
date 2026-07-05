import type { MetadataRoute } from 'next';
import { routing } from '@/i18n/routing';
import { fetchTrending } from '@/lib/api';

const SITE_URL = process.env.NEXT_PUBLIC_SITE_URL || 'http://localhost:3001';
const STATIC_PATHS = ['', '/trending', '/rising', '/categories'];

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const entries: MetadataRoute.Sitemap = [];

  // Static pages per locale, with hreflang alternates.
  for (const path of STATIC_PATHS) {
    for (const locale of routing.locales) {
      entries.push({
        url: `${SITE_URL}/${locale}${path}`,
        changeFrequency: 'daily',
        alternates: {
          languages: Object.fromEntries(
            routing.locales.map((l) => [l, `${SITE_URL}/${l}${path}`]),
          ),
        },
      });
    }
  }

  // Item detail pages (best-effort; skipped if the API is unreachable at build).
  const res = await fetchTrending({ sort: 'stars:desc', limit: 50 });
  if (res.data) {
    for (const item of res.data) {
      for (const locale of routing.locales) {
        entries.push({
          url: `${SITE_URL}/${locale}/item/${item.source}/${item.externalId}`,
          changeFrequency: 'weekly',
        });
      }
    }
  }

  return entries;
}
