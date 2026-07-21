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
 * The privacy policy stays in English outside zh — machine-translated legal
 * text is a liability, so the other locales' message files carry the English
 * wording verbatim. Blog posts are translated per file instead (see lib/blog).
 */
export const LEGAL_LOCALES: readonly Locale[] = ['en', 'zh'];
