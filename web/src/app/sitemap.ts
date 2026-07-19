import type { MetadataRoute } from 'next';
import { routing } from '@/i18n/routing';
import { searchRepos } from '@/lib/github';

const SITE_URL = process.env.NEXT_PUBLIC_SITE_URL || 'http://localhost:3001';

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const entries: MetadataRoute.Sitemap = [];

  // Home page per locale, with hreflang alternates.
  for (const locale of routing.locales) {
    entries.push({
      url: `${SITE_URL}/${locale}`,
      changeFrequency: 'daily',
      alternates: {
        languages: Object.fromEntries(routing.locales.map((l) => [l, `${SITE_URL}/${l}`])),
      },
    });
  }

  // Top repo detail pages (best-effort; skipped if GitHub is unreachable at build).
  const res = await searchRepos({ sort: 'stars', page: 1, perPage: 50 });
  if (res.error === null) {
    for (const repo of res.data.items) {
      for (const locale of routing.locales) {
        entries.push({
          url: `${SITE_URL}/${locale}/repo/${repo.owner}/${repo.name}`,
          changeFrequency: 'weekly',
        });
      }
    }
  }

  return entries;
}
