import { defineRouting } from 'next-intl/routing';

/**
 * Supported locales, ordered as they appear in the language menu.
 * All are left-to-right; adding an RTL locale (ar, he) would need layout work
 * well beyond translation.
 */
export const routing = defineRouting({
  locales: ['en', 'zh', 'ja', 'ko', 'es', 'de', 'fr', 'pt-BR'],
  defaultLocale: 'en',
});

export type Locale = (typeof routing.locales)[number];

/** Endonyms: a language menu should read in the language it offers. */
export const LOCALE_NAMES: Record<Locale, string> = {
  en: 'English',
  zh: '简体中文',
  ja: '日本語',
  ko: '한국어',
  es: 'Español',
  de: 'Deutsch',
  fr: 'Français',
  'pt-BR': 'Português (BR)',
};

/** Compact label for the collapsed menu button. */
export const LOCALE_SHORT: Record<Locale, string> = {
  en: 'EN',
  zh: '中',
  ja: '日',
  ko: '한',
  es: 'ES',
  de: 'DE',
  fr: 'FR',
  'pt-BR': 'PT',
};

/**
 * Locales with hand-written long-form content (privacy policy, blog posts).
 * Everything else falls back to English for those pages: machine-translated
 * legal text is a liability, and a blog index with no posts is worse than one
 * showing the English originals.
 */
export const CONTENT_LOCALES: readonly Locale[] = ['en', 'zh'];

export function contentLocale(locale: string): Locale {
  return (CONTENT_LOCALES as readonly string[]).includes(locale) ? (locale as Locale) : 'en';
}
