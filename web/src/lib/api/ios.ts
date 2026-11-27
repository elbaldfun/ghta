// App Store charts and app pages (/v1/ios). Separate from the repo DTOs: a
// store app has no stars or owner/repo, but a price, a rating and a
// storefront.

import { apiGet, type Fetched } from './client';

/** Storefronts the ios job charts, in the order the switcher lists them. */
export const IOS_COUNTRIES = ['us', 'cn', 'jp', 'gb', 'de'] as const;
export type IOSCountry = (typeof IOS_COUNTRIES)[number];

export const IOS_CHARTS = ['free', 'paid', 'grossing'] as const;
export type IOSChart = (typeof IOS_CHARTS)[number];

/** Apple's top-level genres we chart (internal/source/appstore.Genres), in Apple's order. */
export const IOS_GENRES = [
  'books', 'business', 'developer-tools', 'education', 'entertainment', 'finance', 'food-drink',
  'games', 'graphics-design', 'health-fitness', 'lifestyle', 'magazines-newspapers', 'medical',
  'music', 'navigation', 'news', 'photo-video', 'productivity', 'reference', 'shopping',
  'social-networking', 'sports', 'travel', 'utilities', 'weather',
] as const;
export type IOSGenre = (typeof IOS_GENRES)[number];

export const isCountry = (s: string): s is IOSCountry => (IOS_COUNTRIES as readonly string[]).includes(s);
export const isChart = (s: string): s is IOSChart => (IOS_CHARTS as readonly string[]).includes(s);
export const isGenre = (s: string): s is IOSGenre => (IOS_GENRES as readonly string[]).includes(s);

/** Cookie that remembers the storefront a visitor last looked at. */
export const COUNTRY_COOKIE = 'sr_ios_country';

/** The storefront a visitor most likely wants, by interface language; gb only by choice. */
export function defaultCountry(locale: string): IOSCountry {
  switch (locale) {
    case 'zh':
      return 'cn';
    case 'ja':
      return 'jp';
    case 'de':
      return 'de';
    default:
      return 'us';
  }
}

/** A chart's page path: /ios/us/free, /ios/jp/grossing/games. */
export function chartPath(country: string, chart: string, genre?: string): string {
  return `/ios/${country}/${chart}${genre && genre !== 'all' ? `/${genre}` : ''}`;
}

/**
 * The new-apps view of a storefront: /ios/us/new. Not an Apple chart, so not
 * in IOS_CHARTS, and it has no genre segment.
 */
export const newPath = (country: string) => `/ios/${country}/new`;

export const appPath = (id: string) => `/ios/id${id}`;

/** The app's listing on Apple's site, in its home storefront. */
export const appStoreUrl = (id: string, country?: string) => `https://apps.apple.com/${country || 'us'}/app/id${id}`;

export interface IOSApp {
  id: string;
  name: string;
  developer?: string;
  iconUrl?: string;
  genre?: string;
  price: number;
  currency?: string;
  rating: number;
  ratings: number;
  /** Rating-count gain over 7 days in the home storefront; null until there is a week. */
  ratingsD7: number | null;
  /** The storefront the rating figures come from. */
  homeCountry?: string;
  contentRating?: string;
}

export interface ChartEntry {
  rank: number;
  /** Places gained since the previous snapshot, negative for lost. */
  change?: number;
  /** Not on the previous snapshot of this chart. */
  new?: boolean;
  app: IOSApp;
}

export interface ChartPage {
  country: IOSCountry;
  chart: IOSChart;
  genre: string; // "all" or a genre slug
  day: string;
  prevDay?: string;
  entries: ChartEntry[];
}

export interface ChartPlace {
  country: string;
  chart: string;
  genre: string;
  rank: number;
  change?: number;
  new?: boolean;
}

export interface IOSDetail extends IOSApp {
  description: string;
  homepage?: string;
  bundleId?: string;
  version?: string;
  updatedAt?: string;
  releasedAt?: string;
  screenshots: string[];
  genres: string[];
  countries: string[];
  charts: ChartPlace[];
  chartsDay?: string;
  /** On a chart in the last week; a page that is not stays out of the index. */
  listed: boolean;
  fetchedAt: string;
}

export interface NewEntry {
  /** Place in this StarRank view, not an Apple rank. */
  rank: number;
  /** The app's best place today among the storefront's charts. */
  best: ChartPlace;
  /** How many of the storefront's charts it is on today. */
  charts: number;
  releasedAt: string;
  app: IOSApp;
}

export interface NewPage {
  country: IOSCountry;
  day: string;
  /** Always true: compiled by StarRank from Apple's charts, not one of them. */
  synthetic: boolean;
  sort: string;
  /** How recent a first release must be to count as new. */
  windowDays: number;
  total: number;
  entries: NewEntry[];
}

export interface IOSPoint {
  t: number;
  v: number;
}

export async function getChart(country: string, chart: string, genre = 'all'): Promise<Fetched<ChartPage>> {
  const q = genre && genre !== 'all' ? `?genre=${encodeURIComponent(genre)}` : '';
  const res = await apiGet<{ data: ChartPage }>(`/ios/charts/${country}/${chart}${q}`, 3600);
  if (res.error !== null) return res;
  return { data: { ...res.data.data, entries: res.data.data.entries ?? [] }, error: null };
}

/** A storefront's new apps; 404 until the ios-new job has built its first board. */
export async function getNew(country: string, limit = 100): Promise<Fetched<NewPage>> {
  const res = await apiGet<{ data: NewPage }>(`/ios/new/${country}?limit=${limit}`, 3600);
  if (res.error !== null) return res;
  return { data: { ...res.data.data, entries: res.data.data.entries ?? [] }, error: null };
}

export async function getIOSApp(id: string): Promise<Fetched<IOSDetail>> {
  const res = await apiGet<{ data: IOSDetail }>(`/ios/apps/${encodeURIComponent(id)}`, 3600);
  if (res.error !== null) return res;
  const d = res.data.data;
  return {
    data: { ...d, screenshots: d.screenshots ?? [], genres: d.genres ?? [], countries: d.countries ?? [], charts: d.charts ?? [] },
    error: null,
  };
}

/** The rating-count curve and best-rank days; empty on error so the section hides. */
export async function getIOSHistory(id: string): Promise<{ ratings: IOSPoint[]; bestRank: IOSPoint[] }> {
  const res = await apiGet<{ data: { ratings: IOSPoint[]; bestRank: IOSPoint[] } }>(
    `/ios/apps/${encodeURIComponent(id)}/history`,
    3600,
  );
  if (res.error !== null) return { ratings: [], bestRank: [] };
  return { ratings: res.data.data.ratings ?? [], bestRank: res.data.data.bestRank ?? [] };
}

/** "Free", or the price in the storefront's currency. */
export function formatPrice(price: number, currency: string | undefined, locale: string, free: string): string {
  if (!price) return free;
  try {
    return new Intl.NumberFormat(locale, { style: 'currency', currency: currency || 'USD' }).format(price);
  } catch {
    return `${price} ${currency ?? ''}`.trim();
  }
}

/** A storefront's name in the interface language ("Japan", "日本"). */
export function countryName(code: string, locale: string): string {
  try {
    return new Intl.DisplayNames([locale], { type: 'region' }).of(code.toUpperCase()) ?? code.toUpperCase();
  } catch {
    return code.toUpperCase();
  }
}

/** Apple's icon URLs end in a size ("…/512x512bb.jpg"); ask for a smaller one for rows. */
export function iconAt(url: string | undefined, px: number): string | undefined {
  return url?.replace(/\/\d+x\d+bb\.(jpg|png|webp)$/, `/${px}x${px}bb.$1`);
}
