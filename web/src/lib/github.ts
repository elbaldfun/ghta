// Server-side GitHub REST client for the 2a ranking UI.
// All calls go through Next's data cache (revalidate) to stay inside rate limits:
// authenticated search = 30 req/min, core = 5000 req/hr.

import { unstable_cache } from 'next/cache';
import { BASE_QUALIFIER, TAXONOMY, taxonomyQuery, type SortOption } from './rank-data';

const API = 'https://api.github.com';

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
  createdAt: string;
  htmlUrl: string;
}

export interface RepoDetail extends RepoSummary {
  subscribers: number;
}

export type Fetched<T> = { data: T; error: null } | { data: null; error: string };

function headers(extra: Record<string, string> = {}): Record<string, string> {
  const h: Record<string, string> = {
    Accept: 'application/vnd.github+json',
    'X-GitHub-Api-Version': '2022-11-28',
    ...extra,
  };
  const token = process.env.GITHUB_API_TOKEN;
  if (token) h.Authorization = `Bearer ${token}`;
  return h;
}

async function ghGet<T>(path: string, revalidate: number, accept?: string): Promise<Fetched<T>> {
  try {
    const res = await fetch(`${API}${path}`, {
      headers: headers(accept ? { Accept: accept } : {}),
      next: { revalidate },
    });
    if (!res.ok) return { data: null, error: `GitHub HTTP ${res.status}` };
    const isText = accept?.includes('html');
    const body = isText ? await res.text() : await res.json();
    return { data: body as T, error: null };
  } catch (e) {
    return { data: null, error: e instanceof Error ? e.message : 'network error' };
  }
}

/* eslint-disable @typescript-eslint/no-explicit-any */
function mapRepo(r: any): RepoSummary {
  return {
    owner: r.owner?.login ?? '',
    name: r.name,
    fullName: r.full_name,
    description: r.description,
    language: r.language,
    stars: r.stargazers_count,
    forks: r.forks_count,
    openIssues: r.open_issues_count,
    license: r.license?.spdx_id && r.license.spdx_id !== 'NOASSERTION' ? r.license.spdx_id : null,
    homepage: r.homepage || null,
    topics: r.topics ?? [],
    pushedAt: r.pushed_at,
    createdAt: r.created_at,
    htmlUrl: r.html_url,
  };
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

export function buildQuery(p: SearchParamsIn): string {
  const parts: string[] = [];
  if (p.q) parts.push(`${p.q} in:name,description`);
  const tax = taxonomyQuery(p.cat, p.sub);
  if (tax) parts.push(tax);
  if (p.language) parts.push(`language:"${p.language}"`);
  if (p.license) parts.push(`license:${p.license}`);
  // Searching by keyword relaxes the star floor so niche matches still appear.
  parts.push(p.q ? 'stars:>100' : BASE_QUALIFIER);
  return parts.join(' ');
}

export async function searchRepos(p: SearchParamsIn): Promise<Fetched<SearchResult>> {
  const q = new URLSearchParams({
    q: buildQuery(p),
    sort: p.sort === 'updated' ? 'updated' : p.sort === 'forks' ? 'forks' : 'stars',
    order: 'desc',
    per_page: String(p.perPage ?? 24),
    page: String(p.page ?? 1),
  });
  const res = await ghGet<any>(`/search/repositories?${q}`, 600);
  if (res.error !== null) return res;
  return {
    data: { items: res.data.items.map(mapRepo), totalCount: res.data.total_count },
    error: null,
  };
}

export async function getRepo(owner: string, name: string): Promise<Fetched<RepoDetail>> {
  const res = await ghGet<any>(`/repos/${owner}/${name}`, 600);
  if (res.error !== null) return res;
  return { data: { ...mapRepo(res.data), subscribers: res.data.subscribers_count ?? 0 }, error: null };
}

/** GitHub renders + sanitizes the README to HTML for us. */
export async function getReadmeHtml(owner: string, name: string): Promise<string | null> {
  const res = await ghGet<string>(`/repos/${owner}/${name}/readme`, 3600, 'application/vnd.github.html');
  return res.error === null ? res.data : null;
}

export interface StarPoint {
  t: number; // unix ms
  v: number; // cumulative stars
}

/**
 * Star history, cached for a day per repo. Tries REST page sampling first
 * (full-history approximation; needs a token with the starring scope), then
 * falls back to a GraphQL walk over the most recent ~800 stars (works with
 * metadata-only tokens; covers the full history of smaller repos).
 */
export async function getStarHistory(owner: string, name: string, currentStars: number): Promise<StarPoint[]> {
  const cached = unstable_cache(
    async () => {
      const rest = await starHistoryRest(owner, name, currentStars);
      if (rest.length >= 2) return rest;
      return starHistoryGraphql(owner, name, currentStars);
    },
    ['star-history', owner, name],
    { revalidate: 86400 },
  );
  return cached();
}

/**
 * Approximate star history by sampling the stargazer pages (starred_at).
 * The stargazers endpoint is capped at page 400 (40k stars) — for larger repos
 * the curve covers the first 40k stars and closes with today's total.
 */
async function starHistoryRest(owner: string, name: string, currentStars: number): Promise<StarPoint[]> {
  try {
    const first = await fetch(`${API}/repos/${owner}/${name}/stargazers?per_page=100&page=1`, {
      headers: headers({ Accept: 'application/vnd.github.star+json' }),
      next: { revalidate: 86400 },
    });
    if (!first.ok) return [];

    const lastPage = Math.min(parseLastPage(first.headers.get('link')) ?? 1, 400);
    const sampleCount = Math.min(lastPage, 10);
    const pages = new Set<number>();
    for (let i = 0; i < sampleCount; i++) {
      pages.add(Math.max(1, Math.round(1 + ((lastPage - 1) * i) / Math.max(1, sampleCount - 1))));
    }

    const firstBody = (await first.json()) as { starred_at: string }[];
    const points: StarPoint[] = [];
    if (firstBody[0]?.starred_at) {
      points.push({ t: Date.parse(firstBody[0].starred_at), v: 0 });
    }

    const rest = [...pages].filter((pg) => pg !== 1);
    const results = await Promise.all(
      rest.map(async (pg) => {
        const res = await fetch(`${API}/repos/${owner}/${name}/stargazers?per_page=100&page=${pg}`, {
          headers: headers({ Accept: 'application/vnd.github.star+json' }),
          next: { revalidate: 86400 },
        });
        if (!res.ok) return null;
        const body = (await res.json()) as { starred_at: string }[];
        if (!body[0]?.starred_at) return null;
        return { t: Date.parse(body[0].starred_at), v: (pg - 1) * 100 };
      }),
    );
    for (const p of results) if (p) points.push(p);

    points.push({ t: Date.now(), v: currentStars });
    points.sort((a, b) => a.t - b.t);
    // Cumulative counts must not decrease; drop out-of-order samples.
    const clean: StarPoint[] = [];
    for (const p of points) {
      if (clean.length === 0 || p.v >= clean[clean.length - 1].v) clean.push(p);
    }
    return clean.length >= 2 ? clean : [];
  } catch {
    return [];
  }
}

function parseLastPage(link: string | null): number | null {
  if (!link) return null;
  const m = link.match(/[?&]page=(\d+)>; rel="last"/);
  return m ? Number(m[1]) : null;
}

/**
 * GraphQL fallback: walk stargazers newest-first (sequential cursors only) for
 * up to 8 pages. Small repos (≤800 stars) get their complete history; larger
 * repos get the true recent-growth window ending at today's total.
 */
async function starHistoryGraphql(owner: string, name: string, currentStars: number): Promise<StarPoint[]> {
  const token = process.env.GITHUB_API_TOKEN;
  if (!token) return [];
  try {
    const starredAts: number[] = [];
    let cursor: string | null = null;
    let exhausted = false;

    for (let i = 0; i < 8; i++) {
      const after: string = cursor ? `, after: "${cursor}"` : '';
      const query = `query { repository(owner: "${owner}", name: "${name}") {
        stargazers(first: 100, orderBy: {field: STARRED_AT, direction: DESC}${after}) {
          pageInfo { hasNextPage endCursor }
          edges { starredAt }
        } } }`;
      const res: Response = await fetch(`${API}/graphql`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({ query }),
        cache: 'no-store',
      });
      if (!res.ok) return [];
      const body: any = await res.json();
      const conn = body.data?.repository?.stargazers;
      if (!conn) return [];
      for (const edge of conn.edges) starredAts.push(Date.parse(edge.starredAt));
      if (!conn.pageInfo.hasNextPage) {
        exhausted = true;
        break;
      }
      cursor = conn.pageInfo.endCursor;
    }

    if (starredAts.length === 0) return [];
    starredAts.sort((a, b) => a - b);
    const base = exhausted ? 0 : currentStars - starredAts.length;

    // Downsample to ~40 points for the polyline.
    const step = Math.max(1, Math.floor(starredAts.length / 40));
    const points: StarPoint[] = [];
    for (let i = 0; i < starredAts.length; i += step) {
      points.push({ t: starredAts[i], v: base + i + 1 });
    }
    points.push({ t: Date.now(), v: currentStars });
    return points.length >= 2 ? points : [];
  } catch {
    return [];
  }
}

/** Related repos: same leading topic (falling back to language). */
export async function getRelatedRepos(repo: RepoSummary, limit = 12): Promise<RepoSummary[]> {
  // A single topic keeps the result set broad enough to fill the carousel.
  const topic = repo.topics[0];
  const q = topic
    ? `topic:${topic} stars:>500`
    : repo.language
      ? `language:"${repo.language}" stars:>2000`
      : null;
  if (!q) return [];
  const res = await ghGet<any>(
    `/search/repositories?${new URLSearchParams({ q, sort: 'stars', per_page: String(limit + 1) })}`,
    3600,
  );
  if (res.error !== null) return [];
  return res.data.items
    .map(mapRepo)
    .filter((r: RepoSummary) => r.fullName !== repo.fullName)
    .slice(0, limit);
}

export interface CategoryCounts {
  all: number | null;
  cats: Record<string, number | null>;
  subs: Record<string, number | null>;
}

/** total_count per taxonomy node, heavily cached; nulls hide the badge on failure. */
export async function getCategoryCounts(): Promise<CategoryCounts> {
  const count = async (qualifiers: string): Promise<number | null> => {
    const q = new URLSearchParams({ q: `${qualifiers} ${BASE_QUALIFIER}`.trim(), per_page: '1' });
    const res = await ghGet<any>(`/search/repositories?${q}`, 86400);
    return res.error === null ? (res.data.total_count as number) : null;
  };

  const catEntries = await Promise.all(
    TAXONOMY.map(async (g) => [g.id, await count(g.query)] as const),
  );
  const subEntries = await Promise.all(
    TAXONOMY.flatMap((g) => g.subs ?? []).map(async (s) => [s.id, await count(s.query)] as const),
  );
  return {
    all: await count(''),
    cats: Object.fromEntries(catEntries),
    subs: Object.fromEntries(subEntries),
  };
}
