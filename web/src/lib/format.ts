import type { Source, TrackedItem } from './api';

export function formatNumber(n: number): string {
  if (Math.abs(n) >= 1_000_000) return (n / 1_000_000).toFixed(1).replace(/\.0$/, '') + 'M';
  if (Math.abs(n) >= 1_000) return (n / 1_000).toFixed(1).replace(/\.0$/, '') + 'k';
  return String(n);
}

export function signedNumber(n: number): string {
  return (n >= 0 ? '+' : '') + formatNumber(n);
}

export const sourceLabels: Record<Source, string> = {
  github: 'GitHub',
  appstore: 'App Store',
  chrome: 'Chrome',
  msstore: 'MS Store',
};

export function sourceColor(source: Source): string {
  return `rgb(var(--source-${source}))`;
}

/** The primary metric value used for ranking/display. */
export function primaryValue(item: TrackedItem): number {
  return item.metrics[item.primaryMetric] ?? 0;
}

export function increaseFor(item: TrackedItem, window: 'daily' | 'weekly' | 'monthly'): number | null {
  return item[`${window}Increase` as const];
}

/** owner/name -> path segments for the catch-all detail route. */
export function itemHref(locale: string, item: TrackedItem): string {
  return `/${locale}/item/${item.source}/${item.externalId}`;
}
