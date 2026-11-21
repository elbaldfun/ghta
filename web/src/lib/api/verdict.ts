// The one-line pick at the top of a best-of shelf and a domain board. The
// boards job picks it by a fixed rule (internal/boards/verdict.go); the site
// only words it.

import { apiGet } from './client';
import type { OS } from './apps';

export interface VerdictEvidence {
  externalId: string;
  name: string;
  /** 1-based, in the page's own order. */
  position: number;
  stars: number;
  /** 30-day star gain; absent for a project younger than the window. */
  d30?: number;
  /** Whole days before the board was computed; -1 when unknown. */
  pushedDaysAgo: number;
  /** -1 when no release is known. */
  releaseDaysAgo: number;
  /** Platforms with an installer (shelves only). */
  downloads?: OS[];
  /** Star ranking in the page's domain leaf, now and on the previous ranking day. */
  rankNow?: number;
  rankPrev?: number;
}

export interface VerdictSkip {
  externalId: string;
  name: string;
  position: number;
  reason: 'reading' | 'stale' | 'no-build';
  pushedDaysAgo: number;
}

export interface Verdict {
  scope: 'domain' | 'shelf';
  key: string;
  /** The page order positions refer to: stars (domain board) or the best-of order. The pick itself always goes by stars. */
  basis: 'stars' | 'best';
  pool: number;
  /** How recent the last commit must be, in days. */
  freshDays: number;
  pick: VerdictEvidence;
  skipped: VerdictSkip[];
  riser?: VerdictEvidence;
}

/** A domain board's pick for its default order, and when it was computed. */
export async function getDomainVerdict(path: string): Promise<{ verdict: Verdict | null; computedAt: string | null }> {
  const res = await apiGet<{ data: Verdict | null; computedAt: string }>(
    `/verdict?domain=${encodeURIComponent(path)}`,
    1800,
  );
  if (res.error !== null) return { verdict: null, computedAt: null };
  return { verdict: res.data.data ?? null, computedAt: res.data.computedAt ?? null };
}
