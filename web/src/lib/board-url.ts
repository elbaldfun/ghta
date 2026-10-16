// URLs of the GitHub board. Each browsing dimension that deserves a landing
// page lives in the path (a category, a language, a form) so search engines
// index it; everything else (license, sort, search, page) stays in the query.
//
//   /github                  every repository
//   /github/c/ai/agents      a domain or one of its leaves
//   /github/l/rust           a language
//   /github/t/cli            a form (type facet)
//
// Only one dimension sits in the path, in the order category › language ›
// form; the others, when combined, fall back to query parameters.

import { FILTER_LANGS } from './rank-data';

export interface BoardFilters {
  category?: string;
  lang?: string;
  type?: string;
  license?: string;
  sort?: string;
  q?: string;
  page?: number;
}

const LANG_SLUGS: Record<string, string> = { 'C++': 'cpp', 'C#': 'csharp', 'F#': 'fsharp' };

/** The URL segment of a language: "C++" → "cpp", "TypeScript" → "typescript". */
export function langSlug(lang: string): string {
  return LANG_SLUGS[lang] ?? lang.toLowerCase().replace(/\s+/g, '-');
}

/** The language a URL segment names, or undefined when it is not one we list. */
export function langFromSlug(slug: string): string | undefined {
  return FILTER_LANGS.find((l) => langSlug(l) === slug.toLowerCase());
}

/** The path of the board's landing page for these filters, without the query. */
export function boardPath(f: Pick<BoardFilters, 'category' | 'lang' | 'type'>): string {
  if (f.category) return `/github/c/${f.category}`;
  if (f.lang) return `/github/l/${langSlug(f.lang)}`;
  if (f.type) return `/github/t/${encodeURIComponent(f.type)}`;
  return '/github';
}

/** The full URL of the board for these filters. */
export function boardHref(f: BoardFilters): string {
  const path = boardPath(f);
  const q = new URLSearchParams();
  // The dimensions the path does not already carry.
  if (f.category) {
    if (f.lang) q.set('lang', f.lang);
    if (f.type) q.set('type', f.type);
  } else if (f.lang) {
    if (f.type) q.set('type', f.type);
  }
  if (f.license) q.set('license', f.license);
  if (f.sort) q.set('sort', f.sort);
  if (f.q) q.set('q', f.q);
  if (f.page && f.page > 1) q.set('page', String(f.page));
  const qs = q.toString();
  return qs ? `${path}?${qs}` : path;
}

/** The query parameters the board pages accept. */
export interface BoardQuery {
  category?: string;
  type?: string;
  lang?: string;
  license?: string;
  sort?: string;
  q?: string;
  page?: string;
}

/** Path dimensions plus whatever the query adds; the path wins a conflict. */
export function boardFilters(path: Pick<BoardFilters, 'category' | 'lang' | 'type'>, query: BoardQuery): BoardFilters {
  return {
    category: path.category ?? query.category,
    lang: path.lang ?? query.lang,
    type: path.type ?? query.type,
    license: query.license,
    sort: query.sort,
    q: query.q,
    page: Math.max(1, Number(query.page) || 1),
  };
}
