// Typed client for the GHTA Go backend. Types mirror api/openapi.yaml.

export type Source = 'github' | 'appstore' | 'chrome' | 'msstore';

export interface TrackedItem {
  id: string;
  source: Source;
  externalId: string;
  name: string;
  description: string;
  language?: string;
  categoryId: string[];
  categoryPath?: string;
  primaryMetric: string;
  metricDirection: 'desc-better' | 'asc-better';
  metrics: Record<string, number>;
  dailyIncrease: number | null;
  weeklyIncrease: number | null;
  monthlyIncrease: number | null;
  analysisStatus: 'pending' | 'done' | 'failed';
  sourceData?: Record<string, unknown>;
  fetchedAt: string;
}

export interface CategoryTree {
  id: string;
  name: string;
  path: string;
  children?: CategoryTree[];
}

const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:3000';

/** Result wrapper so pages can render loading/empty/error states uniformly. */
export type Fetched<T> = { data: T; error: null } | { data: null; error: string };

async function get<T>(path: string, revalidate = 300): Promise<Fetched<T>> {
  try {
    const res = await fetch(`${API_URL}${path}`, { next: { revalidate } });
    if (!res.ok) return { data: null, error: `HTTP ${res.status}` };
    const body = (await res.json()) as { data: T };
    return { data: body.data, error: null };
  } catch (e) {
    return { data: null, error: e instanceof Error ? e.message : 'network error' };
  }
}

export interface TrendingParams {
  source?: string;
  language?: string;
  category?: string;
  stars?: string;
  sort?: string;
  limit?: number;
}

export function fetchTrending(params: TrendingParams = {}): Promise<Fetched<TrackedItem[]>> {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== '') q.set(k, String(v));
  }
  const qs = q.toString();
  return get<TrackedItem[]>(`/trending${qs ? `?${qs}` : ''}`);
}

export type RisingWindow = 'daily' | 'weekly' | 'monthly';

export function fetchRising(
  window: RisingWindow = 'weekly',
  params: { source?: string; category?: string; limit?: number } = {},
): Promise<Fetched<TrackedItem[]>> {
  const q = new URLSearchParams({ window });
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== '') q.set(k, String(v));
  }
  return get<TrackedItem[]>(`/trending/rising?${q.toString()}`);
}

export function fetchCategories(): Promise<Fetched<CategoryTree[]>> {
  return get<CategoryTree[]>('/category');
}

export interface Snapshot {
  capturedAt: string;
  metrics: Record<string, number>;
}

export interface ItemDetail {
  item: TrackedItem;
  history: Snapshot[];
}

export function fetchItemDetail(source: string, externalId: string): Promise<Fetched<ItemDetail>> {
  const q = new URLSearchParams({ source, externalId });
  return get<ItemDetail>(`/trending/item?${q.toString()}`);
}
