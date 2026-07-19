// Server-side client for the GHTA Go backend: all ranking data comes from the
// tracked_items database, not the GitHub API. Same-URL fetches are deduped by
// Next's fetch cache; revalidate keeps pages fresh without hammering Mongo.

import { marked } from 'marked';
import sanitizeHtml from 'sanitize-html';
import { LICENSE_NAMES, taxonomyTopics, TAXONOMY, type SortOption } from './rank-data';

const API = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:3000';

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
    topics: sd.topicNames ?? [],
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
  cat?: string;
  sub?: string;
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
  const topics = taxonomyTopics(p.cat, p.sub);
  if (topics.length > 0) params.set('topics', topics.join(','));
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

/** README markdown from the database, rendered and sanitized server-side. */
export async function getReadmeHtml(owner: string, name: string): Promise<string | null> {
  const res = await fetchItem(owner, name);
  const md: string = res.error === null ? (res.data.item.sourceData?.readme ?? '') : '';
  if (!md.trim()) return null;
  const html = await marked.parse(md, { gfm: true });
  return sanitizeHtml(html, {
    allowedTags: sanitizeHtml.defaults.allowedTags.concat(['img', 'details', 'summary', 'ins', 'del']),
    allowedAttributes: {
      ...sanitizeHtml.defaults.allowedAttributes,
      img: ['src', 'alt', 'title', 'width', 'height', 'align'],
      a: ['href', 'name', 'target', 'rel'],
      '*': ['align'],
    },
  });
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

export interface CategoryCounts {
  all: number | null;
  cats: Record<string, number | null>;
  subs: Record<string, number | null>;
}

/** Match totals per taxonomy node, heavily cached; nulls hide the badge. */
export async function getCategoryCounts(): Promise<CategoryCounts> {
  const count = async (topics: string[]): Promise<number | null> => {
    const params = new URLSearchParams({ source: 'github', limit: '1' });
    if (topics.length > 0) params.set('topics', topics.join(','));
    const res = await apiGet<{ total: number }>(`/trending?${params}`, 3600);
    return res.error === null ? res.data.total : null;
  };

  const catEntries = await Promise.all(
    TAXONOMY.map(async (g) => [g.id, await count(g.topics)] as const),
  );
  const subEntries = await Promise.all(
    TAXONOMY.flatMap((g) => g.subs ?? []).map(async (s) => [s.id, await count(s.topics)] as const),
  );
  return {
    all: await count([]),
    cats: Object.fromEntries(catEntries),
    subs: Object.fromEntries(subEntries),
  };
}
