import type { MetadataRoute } from 'next';

const SITE_URL = process.env.NEXT_PUBLIC_SITE_URL || 'http://localhost:3001';

export default function robots(): MetadataRoute.Robots {
  return {
    // Query URLs (filters, sorts, pagination) are uncacheable SSR and were
    // never indexed anyway — no crawlable <a href> emits them except
    // Pagination, and they're absent from the sitemap. Blocking them cuts the
    // bot traffic that exhausted the Vercel quota, at ~zero SEO cost.
    rules: { userAgent: '*', allow: '/', disallow: '/*?' },
    sitemap: `${SITE_URL}/sitemap.xml`,
  };
}
