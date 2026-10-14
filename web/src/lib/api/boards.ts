// The nightly boards: developers, topics, the AI stack, the domain map, the
// category tree and facets, and the corpus statistics. Each is computed once
// a night and served whole, so these calls are cheap.

import { apiGet } from './client';

export interface RankedDeveloper {
  login: string;
  name?: string;
  twitterUsername?: string;
  followers: number;
  company?: string;
  avatarUrl?: string;
  bio?: string;
  repoCount: number;
  totalStars: number;
  topRepo?: string; // "owner/name" of their biggest repo
  topRepoStars: number;
  growth: number; // recent star gain (rising board)
  score: number;
}

export type DeveloperBoard = 'rising' | 'merit';

/**
 * "Developers to follow". board 'rising' = recent growth of owned repos;
 * 'merit' = log-summed stars across owned repos. domain is a top-level domain
 * or 'all'. Organizations are excluded. Empty on error.
 */
export async function getDevelopers(
  board: DeveloperBoard,
  domain: string,
  limit = 30,
  page = 1,
): Promise<{ items: RankedDeveloper[]; total: number }> {
  const res = await apiGet<{ data: RankedDeveloper[]; total: number }>(
    `/developers?board=${board}&domain=${encodeURIComponent(domain)}&limit=${limit}&page=${page}`,
    3600,
  );
  if (res.error !== null || !res.data) return { items: [], total: 0 };
  return { items: res.data.data ?? [], total: res.data.total ?? 0 };
}

export interface HotTopic {
  topic: string;
  repoCount: number;
  totalStars: number;
  growth: number;
  topRepo?: string;
  topRepoStars: number;
  score: number;
}

export type TopicSort = 'hot' | 'popular';

/**
 * Author topics within a domain. sort 'hot' = recent star growth of repos
 * carrying the topic; 'popular' = repo count. Empty on error.
 */
export async function getTopics(domain: string, sort: TopicSort, limit = 40): Promise<HotTopic[]> {
  const res = await apiGet<{ data: HotTopic[] }>(
    `/topics?domain=${encodeURIComponent(domain)}&sort=${sort}&limit=${limit}`,
    3600,
  );
  if (res.error !== null || !res.data) return [];
  return res.data.data ?? [];
}

export interface EcoItem {
  externalId: string;
  name: string;
  description?: string;
  language?: string;
  stars: number;
  forks: number;
  growth: number;
  type?: string;
  isSkill: boolean;
  isMcp: boolean;
  isAgent: boolean;
}

export type EcoPillar = 'all' | 'skill' | 'mcp' | 'agent';
export type EcoSort = 'hot' | 'popular';

/**
 * The "AI stack" board: skills ∪ MCP servers ∪ agent repos, each tagged with
 * the pillars it belongs to. sort 'hot' = recent star growth, 'popular' =
 * total stars. Empty on error.
 */
export async function getEcosystem(
  pillar: EcoPillar,
  sort: EcoSort,
  limit = 45,
  page = 1,
): Promise<{ items: EcoItem[]; total: number }> {
  const res = await apiGet<{ data: EcoItem[]; total: number }>(
    `/ecosystem?pillar=${pillar}&sort=${sort}&limit=${limit}&page=${page}`,
    3600,
  );
  if (res.error !== null || !res.data) return { items: [], total: 0 };
  return { items: res.data.data ?? [], total: res.data.total ?? 0 };
}

export interface HeatCell {
  path: string;
  name: string;
  nameEn?: string;
  repos: number;
  stars: number;
  growth: number;
  intensity: number; // daily stars per repo — the heat signal
  children?: HeatCell[];
}

/**
 * The domain map: every domain with its scale (repos) and current velocity
 * (growth, intensity). Empty on error.
 */
export async function getHeatmap(): Promise<HeatCell[]> {
  const res = await apiGet<{ data: HeatCell[] }>(`/heat`, 3600);
  if (res.error !== null || !res.data) return [];
  return res.data.data ?? [];
}

export interface CategoryNode {
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

/** The domain tree with per-node item counts. */
export async function getCategoryTree(): Promise<CategoryNode[]> {
  const res = await apiGet<{ data: CategoryNode[] }>(`/categories`, 3600);
  return res.error === null ? (res.data.data ?? []) : [];
}

/** Form-facet chips with counts, in the order the chips show them. */
export async function getFacets(): Promise<TypeFacet[]> {
  const res = await apiGet<{ data: TypeFacet[] }>(`/facets/type`, 3600);
  return res.error === null ? (res.data.data ?? []) : [];
}

/** Locale-aware display name: Chinese locales get the zh name, others fall back
 * to the English name (the backend supplies zh + en only). */
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

/** Per-language corpus totals, used by the site's own analysis posts. */
export async function getLanguageStats(limit = 12): Promise<LanguageStat[]> {
  const res = await apiGet<{ data: LanguageStat[] }>(`/stats/languages?limit=${limit}`, 3600);
  return res.error === null ? (res.data.data ?? []) : [];
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

/** Push-recency distribution of the live corpus. */
export async function getStaleness(examples = 0): Promise<Staleness | null> {
  const res = await apiGet<{ data: Staleness }>(`/stats/staleness?examples=${examples}`, 3600);
  return res.error === null ? res.data.data : null;
}

/** Live GitHub repositories tracked, for the "browse all" badge. The list
 * endpoint stops counting at 10,000, so this reads the staleness board's total. */
export async function getTotalCount(): Promise<number | null> {
  const s = await getStaleness(0);
  return s ? s.total : null;
}
