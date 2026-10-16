// Repositories: lists, search, the detail page and its curves, and GitHub's
// trending lists. The /v1 DTOs are mapped onto the UI shapes the pages use.

import { marked } from 'marked';
import sanitizeHtml from 'sanitize-html';
import { LICENSE_NAMES, type SortOption } from '../rank-data';
import { apiGet, type Fetched } from './client';
import type { Alternative, OS } from './apps';

export type { Fetched } from './client';

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
  dailyIncrease: number | null;
  weeklyIncrease: number | null;
}

export interface ReleaseAsset {
  name: string;
  platform: OS;
  url?: string;
  size?: number;
}

/** One leaderboard position for the detail page's rank line. */
export interface RepoRank {
  scope: 'overall' | 'language' | 'category';
  key?: string; // language name or category path
  rank: number; // 1-based
}

export interface RepoDetail extends RepoSummary {
  ranks: RepoRank[];
  screenshotUrl?: string;
  shotSource?: string; // readme | og
  platforms: OS[];
  platformSource?: string;
  releaseAssets: ReleaseAsset[];
  latestRelease?: { tag?: string; publishedAt?: string | null } | null;
  alternativeTo: Alternative[];
}

export interface StarPoint {
  t: number; // unix ms
  v: number; // cumulative stars
}

/** A /v1 repository card. */
interface Card {
  fullName: string;
  owner: string;
  name: string;
  description?: string;
  language?: string;
  license?: string;
  homepage?: string;
  topics?: string[];
  type?: string;
  domain?: string[];
  stars: number;
  forks: number;
  openIssues: number;
  growth?: { d1: number | null; d7: number | null; d30: number | null };
  pushedAt?: string;
}

function mapCard(c: Card): RepoSummary {
  return {
    owner: c.owner,
    name: c.name,
    fullName: c.fullName,
    description: c.description || null,
    language: c.language || null,
    stars: c.stars ?? 0,
    forks: c.forks ?? 0,
    openIssues: c.openIssues ?? 0,
    license: c.license || null,
    homepage: c.homepage || null,
    topics: c.topics ?? [],
    type: c.type || null,
    categoryPath: c.domain ?? [],
    pushedAt: c.pushedAt ?? '',
    createdAt: null, // repo creation date is not tracked
    htmlUrl: `https://github.com/${c.fullName}`,
    dailyIncrease: c.growth?.d1 ?? null,
    weeklyIncrease: c.growth?.d7 ?? null,
  };
}

export type HotWindow = 'daily' | 'weekly' | 'monthly';

export interface HotRepo {
  repo: RepoSummary;
  starsGained: number; // stars gained during the window
  inCorpus: boolean; // true when we track & classify it; false = GitHub-only
}

/**
 * GitHub's own trending list for a window, as the backend scraped it last
 * night, joined to our labels. Returns [] on error so the section hides.
 */
export async function getHot(since: HotWindow, limit = 12): Promise<HotRepo[]> {
  const res = await apiGet<{ data: HotRow[] }>(`/hot?since=${since}&limit=${limit}`, 3600);
  if (res.error !== null || !Array.isArray(res.data?.data)) return [];
  return res.data.data.map((it) => ({
    starsGained: it.starsGained ?? 0,
    inCorpus: Boolean(it.tracked),
    repo: {
      owner: it.owner,
      name: it.name,
      fullName: it.fullName,
      description: it.description || null,
      language: it.language || null,
      stars: it.stars ?? 0,
      forks: it.forks ?? 0,
      openIssues: 0,
      license: null,
      homepage: null,
      topics: [],
      type: it.type || null,
      categoryPath: it.domain ?? [],
      pushedAt: '',
      createdAt: null,
      htmlUrl: `https://github.com/${it.fullName}`,
      dailyIncrease: null,
      weeklyIncrease: null,
    },
  }));
}

interface HotRow {
  fullName: string;
  owner: string;
  name: string;
  description?: string;
  language?: string;
  stars: number;
  forks: number;
  starsGained: number;
  tracked: boolean;
  domain?: string[];
  type?: string;
}

export interface TrendPoint {
  t: number; // unix ms
  v: number; // stars
}

/**
 * Daily star series per repo (never interpolated), for sparklines. Repos with
 * no readings in the window are absent from the map. Empty on error.
 */
export async function getTrends(ids: string[], days = 30): Promise<Record<string, TrendPoint[]>> {
  if (ids.length === 0) return {};
  const res = await apiGet<{ data: Record<string, TrendPoint[]> }>(
    `/trend?ids=${ids.map(encodeURIComponent).join(',')}&days=${days}`,
    3600,
  );
  if (res.error !== null || !res.data) return {};
  return res.data.data ?? {};
}

export interface SearchParamsIn {
  category?: string; // domain path: a leaf "a/b" or a top-level "a"
  type?: string; // form facet (cli|app|library|software|tutorial|awesome|interview|skill)
  q?: string;
  language?: string;
  license?: string;
  sort?: SortOption;
  page?: number;
  perPage?: number;
}

/** The list endpoint stops counting here; a total this size means "or more". */
export const LIST_TOTAL_CAP = 10000;

/** A result count for display: "10,000+" once the count reaches the cap. */
export function formatTotal(n: number): string {
  return n >= LIST_TOTAL_CAP ? `${LIST_TOTAL_CAP.toLocaleString()}+` : n.toLocaleString();
}

export interface SearchResult {
  items: RepoSummary[];
  totalCount: number;
}

function listParams(p: SearchParamsIn): URLSearchParams {
  const params = new URLSearchParams();
  if (p.category) params.set('domain', p.category);
  if (p.type) params.set('type', p.type);
  if (p.q) params.set('q', p.q);
  if (p.language) params.set('language', p.language);
  if (p.license && LICENSE_NAMES[p.license]) params.set('license', LICENSE_NAMES[p.license]);
  // With a query and no concrete sort the backend ranks by relevance;
  // without a query it defaults to stars.
  if (p.sort && p.sort !== 'relevance') params.set('sort', p.sort);
  params.set('limit', String(p.perPage ?? 24));
  params.set('page', String(p.page ?? 1));
  return params;
}

export async function searchRepos(p: SearchParamsIn): Promise<Fetched<SearchResult>> {
  const res = await apiGet<{ data: Card[]; total: number }>(`/repos?${listParams(p)}`, 300);
  if (res.error !== null) return res;
  return { data: { items: res.data.data.map(mapCard), totalCount: res.data.total }, error: null };
}

const repoPath = (owner: string, name: string) =>
  `/repos/${encodeURIComponent(owner)}/${encodeURIComponent(name)}`;

interface DetailDTO extends Card {
  ranks: { scope: string; key?: string; rank: number }[];
  screenshotUrl?: string;
  shotSource?: string;
  platforms: OS[];
  platformSource?: string;
  releaseAssets: ReleaseAsset[];
  latestRelease: { tag?: string; publishedAt?: string } | null;
  alternatives: Alternative[];
}

export async function getRepo(owner: string, name: string): Promise<Fetched<RepoDetail>> {
  const res = await apiGet<{ data: DetailDTO }>(repoPath(owner, name), 600);
  if (res.error !== null) return res;
  const d = res.data.data;
  return {
    data: {
      ...mapCard(d),
      ranks: (d.ranks ?? [])
        .filter((r) => r.rank > 0)
        .map((r) => ({ scope: r.scope === 'domain' ? 'category' : (r.scope as RepoRank['scope']), key: r.key, rank: r.rank })),
      screenshotUrl: d.screenshotUrl || undefined,
      shotSource: d.shotSource || undefined,
      platforms: d.platforms ?? [],
      platformSource: d.platformSource || undefined,
      releaseAssets: d.releaseAssets ?? [],
      latestRelease: d.latestRelease,
      alternativeTo: d.alternatives ?? [],
    },
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
 * The README, fetched on its own, rendered and sanitized server-side, plus an
 * outline for the sidebar.
 *
 * Anchor ids are injected *after* sanitizing rather than allowing `id` through
 * the whitelist: the values are slugs we generate ourselves, so untrusted
 * markup can never place an id of its choosing on the page.
 */
export async function getReadme(owner: string, name: string): Promise<Readme | null> {
  const res = await apiGet<{ data: { markdown: string } }>(`${repoPath(owner, name)}/readme`, 600);
  const md = res.error === null ? (res.data.data?.markdown ?? '') : '';
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

/** The star curve: the long backfilled history joined to the daily readings. */
export async function getStarHistory(owner: string, name: string): Promise<StarPoint[]> {
  const res = await apiGet<{ data: StarPoint[] }>(`${repoPath(owner, name)}/history`, 600);
  if (res.error !== null || !Array.isArray(res.data?.data)) return [];
  const points = res.data.data.filter((p) => Number.isFinite(p.t));
  points.sort((a, b) => a.t - b.t);
  return points.length >= 2 ? points : [];
}

/** Related repos: same leading topic, falling back to language. */
export async function getRelatedRepos(repo: RepoSummary, limit = 12): Promise<RepoSummary[]> {
  const params = new URLSearchParams({ sort: 'stars', limit: String(limit + 1) });
  const topic = repo.topics[0];
  if (topic) params.set('topics', topic);
  else if (repo.language) params.set('language', repo.language);
  else return [];
  const res = await apiGet<{ data: Card[] }>(`/repos?${params}`, 3600);
  if (res.error !== null) return [];
  return res.data.data
    .map(mapCard)
    .filter((r) => r.fullName !== repo.fullName)
    .slice(0, limit);
}

/** The owner's other repositories, most starred first. */
export async function getOwnerRepos(repo: RepoSummary, limit = 6): Promise<RepoSummary[]> {
  const params = new URLSearchParams({ owner: repo.owner, sort: 'stars', limit: String(limit + 1) });
  const res = await apiGet<{ data: Card[] }>(`/repos?${params}`, 3600);
  if (res.error !== null) return [];
  return res.data.data
    .map(mapCard)
    .filter((r) => r.fullName !== repo.fullName)
    .slice(0, limit);
}

/** Projects in the repo's first domain gaining the most stars this week. */
export async function getDomainPeers(repo: RepoSummary, limit = 6): Promise<RepoSummary[]> {
  const domain = repo.categoryPath[0];
  if (!domain) return [];
  const res = await searchRepos({ category: domain, sort: 'weekly', perPage: limit + 1 });
  if (res.error !== null) return [];
  return res.data.items.filter((r) => r.fullName !== repo.fullName).slice(0, limit);
}
