// Story selection for the home page ("头版"). Pure functions over data the
// page already fetched, so the rules the footer prints live in one place:
//   lead  — tracked 7+ days, most new stars today
//   fresh — tracked under 7 days, most new stars today
//   cool  — overall top 10 whose daily gain fell most against 30 days ago
// "Tracked under 7 days" is read from the date of the first reading: the
// list's 7-day growth is not reliably null for new entries, and repos under
// 3,000 stars are read every other day, so counting readings misleads too.
import type { RepoSummary, TrendPoint } from '@/lib/data';

const DAY = 86_400_000;

/** Whole UTC days from a to b (unix ms). */
function daysBetween(a: number, b: number): number {
  return Math.floor(b / DAY) - Math.floor(a / DAY);
}

/**
 * Cumulative star readings → stars gained per calendar day, oldest first,
 * ending on the day of the last reading. Readings two or more days apart
 * (every-other-day fetches, a missed run) have their gain spread evenly over
 * the days between them, the way the backend's ComputeGrowth prorates a
 * window (internal/item/growth.go). Rounding is cumulative so the days still
 * add up to the real gain. Two readings on one day: the later one counts.
 */
export function dailySeries(points: TrendPoint[] | undefined): number[] {
  if (!points || points.length < 2) return [];
  const out: number[] = [];
  let prev = points[0];
  for (let i = 1; i < points.length; i++) {
    const p = points[i];
    const span = daysBetween(prev.t, p.t);
    if (span <= 0) {
      if (out.length === 0) prev = p;
      else {
        out[out.length - 1] += p.v - prev.v;
        prev = p;
      }
      continue;
    }
    const gain = p.v - prev.v;
    for (let k = 1; k <= span; k++) {
      out.push(Math.round((gain * k) / span) - Math.round((gain * (k - 1)) / span));
    }
    prev = p;
  }
  return out;
}

/** Days on the board as of `now`, counting the first reading's day as day 1. */
export function daysTracked(points: TrendPoint[], now: number): number {
  return daysBetween(points[0].t, now) + 1;
}

export interface LeadStory {
  repo: RepoSummary;
  series: number[];
  /** Average daily gain before the last 7 days; null when too short to say. */
  prior: number | null;
  /** The last week ran at 5× the earlier pace or more. */
  breakout: boolean;
}

/** First read under 7 days before `now`. No readings at all (the trend
 * call failed, or the repo was not asked for) means unknown, not new. */
export function isNewEntry(points: TrendPoint[] | undefined, now: number): boolean {
  return points != null && points.length > 0 && daysTracked(points, now) <= 7;
}

/** Candidates must come with their trend: one without is unknown, skipped. */
export function pickLead(
  movers: RepoSummary[],
  trends: Record<string, TrendPoint[]>,
  now: number,
): LeadStory | null {
  const repo = movers.find((r) => trends[r.fullName]?.length && !isNewEntry(trends[r.fullName], now));
  if (!repo) return null;
  const series = dailySeries(trends[repo.fullName]);
  const before = series.slice(0, -7);
  const prior = before.length >= 7 ? before.reduce((a, b) => a + b, 0) / before.length : null;
  const weekAvg = (repo.weeklyIncrease ?? 0) / 7;
  const breakout = prior != null && weekAvg >= Math.max(prior, 1) * 5;
  return { repo, series, prior, breakout };
}

export interface FreshStory {
  repo: RepoSummary;
  series: number[];
  /** Days on the board, counting the first reading as day 1. */
  days: number;
}

export function pickFresh(
  movers: RepoSummary[],
  trends: Record<string, TrendPoint[]>,
  now: number,
): FreshStory | null {
  const repo = movers.find((r) => isNewEntry(trends[r.fullName], now));
  if (!repo) return null;
  const points = trends[repo.fullName];
  return { repo, series: dailySeries(points), days: daysTracked(points, now) };
}

export interface CoolStory {
  repo: RepoSummary;
  series: number[];
  from: number;
  to: number;
}

export function pickCool(
  top: RepoSummary[],
  trends: Record<string, TrendPoint[]>,
  exclude: Set<string>,
): CoolStory | null {
  let best: CoolStory | null = null;
  for (const repo of top.slice(0, 10)) {
    if (exclude.has(repo.fullName)) continue;
    const series = dailySeries(trends[repo.fullName]);
    if (series.length < 20) continue;
    const from = series[0];
    const to = series[series.length - 1];
    if (to >= from) continue;
    if (!best || from - to > best.from - best.to) best = { repo, series, from, to };
  }
  return best;
}

/** 5_300_000_000 → "5.3B", 74_000_000 → "74M". */
export function fmtParams(n: number | undefined): string | null {
  if (!n || n <= 0) return null;
  if (n >= 1e9) return `${(n / 1e9).toFixed(1)}B`;
  if (n >= 1e6) return `${Math.round(n / 1e6)}M`;
  return `${Math.round(n / 1e3)}K`;
}
