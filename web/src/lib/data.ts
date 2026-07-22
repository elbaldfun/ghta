// Server-side client for the GHTA Go backend: all ranking data comes from the
// tracked_items database, not the GitHub API. Same-URL fetches are deduped by
// Next's fetch cache; revalidate keeps pages fresh without hammering Mongo.

import { marked } from 'marked';
import sanitizeHtml from 'sanitize-html';
import { LICENSE_NAMES, type SortOption } from './rank-data';

// Server-only on purpose: without the NEXT_PUBLIC_ prefix the backend address
// is never inlined into the browser bundle. Every caller below runs on the server.
const API = process.env.API_URL || 'http://localhost:3000';

export interface RepoSummary {
  owner: string;
  name: string;
  fullName: string;
  description: string | null;
  language: string | null;
  stars: number;
  forks: number;
  openIssues: number;
  license: string | null;
  homepage: string | null;
  topics: string[];
  type: string | null; // form facet (cli|app|library|software|tutorial|awesome|interview|skill)
  categoryPath: string[]; // domain leaf paths (multi-label)
  pushedAt: string;
  createdAt: string | null;
  htmlUrl: string;
}

export interface RepoDetail extends RepoSummary {
  weeklyIncrease: number | null;
}

export type Fetched<T> = { data: T; error: null } | { data: null; error: string };

export interface StarPoint {
  t: number; // unix ms
  v: number; // cumulative stars
}

/* eslint-disable @typescript-eslint/no-explicit-any */

/** TrackedItem (backend shape) -> RepoSummary (UI shape). */
function mapItem(it: any): RepoSummary {
  const [owner = '', name = ''] = String(it.externalId ?? '').split('/');
  const sd = it.sourceData ?? {};
  // Prefer the author's own topics; fall back to LLM-generated tags so no-topic
  // repos are still searchable and get related-repo suggestions (change 12).
  const authorTopics: string[] = sd.topicNames ?? [];
  const topics = authorTopics.length > 0 ? authorTopics : (it.generatedTopics ?? []);
  return {
    owner,
    name: name || it.name,
    fullName: it.externalId,
    description: it.description || null,
    language: it.language || null,
    stars: it.metrics?.stars ?? 0,
    forks: it.metrics?.forks ?? 0,
    openIssues: it.metrics?.openIssues ?? 0,
    license: sd.license || null,
    homepage: sd.homepageUrl || null,
    topics,
    type: it.type || null,
    categoryPath: it.categoryPath ?? [],
    pushedAt: sd.pushedAt || it.fetchedAt,
    createdAt: null, // repo creation date is not tracked in the database
    htmlUrl: sd.url || `https://github.com/${it.externalId}`,
  };
}

async function apiGet<T>(path: string, revalidate: number): Promise<Fetched<T>> {
  try {
    const res = await fetch(`${API}${path}`, { next: { revalidate } });
    if (!res.ok) return { data: null, error: `backend HTTP ${res.status}` };
    return { data: (await res.json()) as T, error: null };
  } catch (e) {
    return { data: null, error: e instanceof Error ? e.message : 'network error' };
  }
}

export interface SearchParamsIn {
  category?: string; // domain path (leaf "a/b" or parent "a") or category id
  type?: string; // form facet (cli|app|library|software|tutorial|awesome|interview|skill)
  q?: string;
  language?: string;
  license?: string;
  sort?: SortOption;
  page?: number;
  perPage?: number;
}

export interface SearchResult {
  items: RepoSummary[];
  totalCount: number;
}

function listParams(p: SearchParamsIn): URLSearchParams {
  const params = new URLSearchParams({ source: 'github' });
  if (p.category) params.set('category', p.category);
  if (p.type) params.set('type', p.type);
  if (p.q) params.set('q', p.q);
  if (p.language) params.set('language', p.language);
  if (p.license && LICENSE_NAMES[p.license]) params.set('license', LICENSE_NAMES[p.license]);
  params.set('sort', `${p.sort === 'updated' ? 'updated' : (p.sort ?? 'stars')}:desc`);
  params.set('limit', String(p.perPage ?? 24));
  params.set('page', String(p.page ?? 1));
  return params;
}

export async function searchRepos(p: SearchParamsIn): Promise<Fetched<SearchResult>> {
  const res = await apiGet<{ data: any[]; total: number }>(`/trending?${listParams(p)}`, 300);
  if (res.error !== null) return res;
  return {
    data: { items: res.data.data.map(mapItem), totalCount: res.data.total },
    error: null,
  };
}

/** One item + its snapshot history; identical URLs dedupe within a render. */
async function fetchItem(
  owner: string,
  name: string,
): Promise<Fetched<{ item: any; history: any[] }>> {
  const params = new URLSearchParams({ source: 'github', externalId: `${owner}/${name}` });
  const res = await apiGet<{ data: { item: any; history: any[] } }>(
    `/trending/item?${params}`,
    600,
  );
  if (res.error !== null) return res;
  return { data: res.data.data, error: null };
}

export async function getRepo(owner: string, name: string): Promise<Fetched<RepoDetail>> {
  const res = await fetchItem(owner, name);
  if (res.error !== null) return res;
  const it = res.data.item;
  return {
    data: { ...mapItem(it), weeklyIncrease: it.weeklyIncrease ?? null },
    error: null,
  };
}

export interface ReadmeHeading {
  id: string;
  text: string;
  depth: number;
}

export interface Readme {
  html: string;
  toc: ReadmeHeading[];
}

// h1–h3 only: deeper levels bury the outline in long READMEs.
const HEADING_RE = /<h([1-3])([^>]*)>([\s\S]*?)<\/h\1>/g;

/** Visible text of a heading, with inline markup and entities removed. */
function headingText(inner: string): string {
  return inner
    .replace(/<[^>]+>/g, '')
    .replace(/&nbsp;/g, ' ')
    .replace(/&amp;/g, '&')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/\s+/g, ' ')
    .trim();
}

/** Anchor slug; \p{L} keeps CJK headings addressable instead of collapsing them to "section". */
function slugify(text: string, used: Set<string>): string {
  const base =
    text
      .toLowerCase()
      .replace(/[^\p{L}\p{N}]+/gu, '-')
      .replace(/^-+|-+$/g, '') || 'section';
  let slug = base;
  for (let i = 2; used.has(slug); i++) slug = `${base}-${i}`;
  used.add(slug);
  return slug;
}

/**
 * README markdown from the database, rendered and sanitized server-side, plus
 * an outline for the sidebar.
 *
 * Anchor ids are injected *after* sanitizing rather than allowing `id` through
 * the whitelist: the values are slugs we generate ourselves, so untrusted
 * markup can never place an id of its choosing on the page.
 */
export async function getReadme(owner: string, name: string): Promise<Readme | null> {
  const res = await fetchItem(owner, name);
  const md: string = res.error === null ? (res.data.item.sourceData?.readme ?? '') : '';
  if (!md.trim()) return null;
  const rendered = await marked.parse(md, { gfm: true });
  const clean = sanitizeHtml(rendered, {
    allowedTags: sanitizeHtml.defaults.allowedTags.concat(['img', 'details', 'summary', 'ins', 'del']),
    allowedAttributes: {
      ...sanitizeHtml.defaults.allowedAttributes,
      img: ['src', 'alt', 'title', 'width', 'height', 'align'],
      a: ['href', 'name', 'target', 'rel'],
      '*': ['align'],
    },
  });

  const toc: ReadmeHeading[] = [];
  const used = new Set<string>();
  const html = clean.replace(HEADING_RE, (match, depth: string, attrs: string, inner: string) => {
    const text = headingText(inner);
    if (!text) return match;
    const id = slugify(text, used);
    toc.push({ id, text, depth: Number(depth) });
    return `<h${depth}${attrs} id="${id}">${inner}</h${depth}>`;
  });

  return { html, toc };
}

/** Star history from metric snapshots (grows one point per daily fetch). */
export async function getStarHistory(owner: string, name: string): Promise<StarPoint[]> {
  const res = await fetchItem(owner, name);
  if (res.error !== null) return [];
  const points: StarPoint[] = res.data.history
    .map((s: any) => ({ t: Date.parse(s.capturedAt), v: s.metrics?.stars ?? 0 }))
    .filter((p: StarPoint) => Number.isFinite(p.t));
  points.sort((a, b) => a.t - b.t);
  return points.length >= 2 ? points : [];
}

/** Related repos: same leading topic, falling back to language. */
export async function getRelatedRepos(repo: RepoSummary, limit = 12): Promise<RepoSummary[]> {
  const params = new URLSearchParams({ source: 'github', sort: 'stars:desc', limit: String(limit + 1) });
  const topic = repo.topics[0];
  if (topic) params.set('topics', topic);
  else if (repo.language) params.set('language', repo.language);
  else return [];
  const res = await apiGet<{ data: any[] }>(`/trending?${params}`, 3600);
  if (res.error !== null) return [];
  return res.data.data
    .map(mapItem)
    .filter((r) => r.fullName !== repo.fullName)
    .slice(0, limit);
}

export interface CategoryNode {
  id: string;
  name: string;
  nameEn?: string;
  path: string;
  count: number;
  children?: CategoryNode[];
}

export interface TypeFacet {
  key: string;
  name: string;
  count: number;
}

/** The controlled domain tree with per-node item counts (backend GET /category).
 * Replaces the old hardcoded TAXONOMY and its 22-request count fan-out. */
export async function getCategoryTree(): Promise<CategoryNode[]> {
  const res = await apiGet<{ data: CategoryNode[] }>(`/category`, 3600);
  return res.error === null ? res.data.data : [];
}

/** Total item count across the corpus, for the "browse all" badge. */
export async function getTotalCount(): Promise<number | null> {
  const res = await apiGet<{ total: number }>(`/trending?source=github&limit=1`, 3600);
  return res.error === null ? res.data.total : null;
}

/** Form-facet chips with counts (backend GET /category/facets), in priority order. */
export async function getFacets(): Promise<TypeFacet[]> {
  const res = await apiGet<{ data: { type: TypeFacet[] } }>(`/category/facets`, 3600);
  return res.error === null ? res.data.data.type : [];
}

/** Locale-aware display name: Chinese locales get the zh name, others fall back
 * to the English name (backend supplies zh + en only). */
export function categoryLabel(node: { name: string; nameEn?: string }, locale: string): string {
  return locale.startsWith('zh') ? node.name : node.nameEn || node.name;
}

/** Find a node by its path anywhere in the tree (for headings/breadcrumbs). */
export function findCategory(tree: CategoryNode[], path: string): CategoryNode | undefined {
  for (const g of tree) {
    if (g.path === path) return g;
    const hit = g.children?.find((c) => c.path === path);
    if (hit) return hit;
  }
  return undefined;
}

export interface LanguageStat {
  language: string;
  repos: number;
  totalStars: number;
  medianStars: number;
  topRepo: string;
  topStars: number;
}

/**
 * Per-language corpus totals, used by the site's own analysis posts.
 * Cached for an hour: the underlying numbers only move once a day.
 */
export async function getLanguageStats(limit = 12): Promise<LanguageStat[]> {
  const res = await apiGet<{ data: LanguageStat[] }>(`/stats/languages?limit=${limit}`, 3600);
  return res.error === null ? res.data.data : [];
}

export interface StalenessBucket {
  bucket: 'active' | 'slowing' | 'dormant' | 'stale';
  repos: number;
  medianStars: number;
  medianIssues: number;
  /** Open issues per 1,000 stars — backlog normalised against audience size. */
  issuesPerKStar: number;
}

export interface StaleRepo {
  externalId: string;
  language: string;
  stars: number;
  openIssues: number;
  pushedAt: string;
}

export interface Staleness {
  total: number;
  buckets: StalenessBucket[];
  examples: StaleRepo[];
}

/** Push-recency distribution. Cached for an hour; the corpus moves once a day. */
export async function getStaleness(examples = 0): Promise<Staleness | null> {
  const res = await apiGet<{ data: Staleness }>(`/stats/staleness?examples=${examples}`, 3600);
  return res.error === null ? res.data.data : null;
}
