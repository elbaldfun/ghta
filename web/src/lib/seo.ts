import type { Metadata } from 'next';
import { routing } from '@/i18n/routing';

export const SITE_URL = process.env.NEXT_PUBLIC_SITE_URL || 'http://localhost:3001';

/**
 * The canonical URL and hreflang alternates of one page. `path` is the page's
 * path without the locale prefix ("" for the home page, "/github/c/ai" …).
 * Every page sets this itself: one set by the layout would declare every page
 * a copy of the home page.
 */
export function alternatesFor(locale: string, path: string): Metadata['alternates'] {
  const languages = Object.fromEntries(routing.locales.map((l) => [l, `${SITE_URL}/${l}${path}`]));
  return {
    canonical: `/${locale}${path}`,
    languages: { ...languages, 'x-default': `${SITE_URL}/${routing.defaultLocale}${path}` },
  };
}
