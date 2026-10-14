// The open-source app store: the directory, best-of collections and the
// alternatives pages.

import { apiGet } from './client';

export type OS = 'macos' | 'windows' | 'linux' | 'android' | 'ios' | 'web';
export type AppKind = 'app' | 'cli';
export type AppSort = 'hot' | 'popular' | 'new';

export interface AppDownload {
  os: OS;
  url: string;
  name: string;
}

export interface Alternative {
  name: string;
  slug: string;
  kind?: string;
}

export interface AppItem {
  externalId: string;
  name: string;
  description?: string;
  language?: string;
  iconUrl?: string;
  screenshotUrl?: string;
  homepage?: string;
  license?: string;
  stars: number;
  growth: number;
  type?: string;
  kind: AppKind;
  shelf?: string; // store shelf slug, e.g. "system/screenshot"
  taglineZh?: string;
  taglineEn?: string;
  platforms: OS[];
  latestReleaseAt?: string | null;
  downloads?: AppDownload[];
  alternativeTo?: Alternative[];
  hasDownloads: boolean;
}

export interface AltTarget {
  name: string;
  slug: string;
  kind?: string;
  count: number;
}

/** A /v1 store row; it names the products it replaces "alternatives". */
type AppDTO = Omit<AppItem, 'alternativeTo'> & { alternatives?: Alternative[] };

function mapApp({ alternatives, ...a }: AppDTO): AppItem {
  return { ...a, platforms: a.platforms ?? [], alternativeTo: alternatives ?? [] };
}

/** Paid products with the most open-source alternatives (the /alternatives index). */
export async function getAltTargets(): Promise<AltTarget[]> {
  const res = await apiGet<{ data: AltTarget[] }>(`/alternatives`, 3600);
  return res.error === null ? (res.data.data ?? []) : [];
}

/** Open-source apps that replace one product, plus its display name. */
export async function getAppsByAlternative(slug: string): Promise<{ items: AppItem[]; name: string }> {
  const res = await apiGet<{ data: AppDTO[]; name: string }>(`/alternatives/${encodeURIComponent(slug)}`, 3600);
  if (res.error !== null || !res.data) return { items: [], name: slug };
  return { items: (res.data.data ?? []).map(mapApp), name: res.data.name ?? slug };
}

/**
 * The app directory: apps and CLIs filtered by OS, form, domain and shelf, and
 * sorted hot|popular|new. Empty on error so the page shows its empty state.
 */
export async function getApps(params: {
  os?: string;
  kind?: string;
  category?: string;
  shelf?: string;
  sort?: AppSort;
  limit?: number;
  page?: number;
}): Promise<{ items: AppItem[]; total: number }> {
  const q = new URLSearchParams();
  if (params.os) q.set('os', params.os);
  if (params.kind) q.set('kind', params.kind);
  if (params.category) q.set('domain', params.category);
  if (params.shelf) q.set('shelf', params.shelf);
  q.set('sort', params.sort ?? 'hot');
  q.set('limit', String(params.limit ?? 30));
  q.set('page', String(params.page ?? 1));
  const res = await apiGet<{ data: AppDTO[]; total: number }>(`/apps?${q.toString()}`, 3600);
  if (res.error !== null || !res.data) return { items: [], total: 0 };
  return { items: (res.data.data ?? []).map(mapApp), total: res.data.total ?? 0 };
}

/** The best-of collection for one shelf (/apps/best/<major>/<sub>). */
export async function getBestOf(shelf: string): Promise<AppItem[]> {
  const res = await apiGet<{ data: AppDTO[] }>(`/apps/best/${shelf}`, 1800);
  if (res.error !== null || !Array.isArray(res.data?.data)) return [];
  return res.data.data.map(mapApp);
}
