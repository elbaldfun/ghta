// Story selection for the home page ("头版"). Pure functions over data the
// page already fetched, so the rules the footer prints live in one place:
//   lead  — tracked 7+ days, most new stars today
//   fresh — tracked under 7 days, most new stars today
//   cool  — overall top 10 whose daily gain fell most against 30 days ago
// "Tracked under 7 days" is read from the trend series length: the list's
// 7-day growth is not reliably null for new entries.
import type { RepoSummary, TrendPoint } from '@/lib/data';

/** Cumulative star readings → stars gained per reading, oldest first. */
export function dailySeries(points: TrendPoint[] | undefined): number[] {
  if (!points || points.length < 2) return [];
  const out: number[] = [];
  for (let i = 1; i < points.length; i++) out.push(points[i].v - points[i - 1].v);
  return out;
}

export interface LeadStory {
  repo: RepoSummary;
  series: number[];
  /** Average daily gain before the last 7 days; null when too short to say. */
  prior: number | null;
  /** The last week ran at 5× the earlier pace or more. */
  breakout: boolean;
}

/** A repo with 7 or fewer daily readings joined the board this week. No
 * series at all (the trend call failed) means unknown, not new. */
export function isNewEntry(points: TrendPoint[] | undefined): boolean {
  return points != null && points.length <= 7;
}

export function pickLead(movers: RepoSummary[], trends: Record<string, TrendPoint[]>): LeadStory | null {
  const repo = movers.find((r) => trends[r.fullName] && !isNewEntry(trends[r.fullName]));
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

export function pickFresh(movers: RepoSummary[], trends: Record<string, TrendPoint[]>): FreshStory | null {
  const repo = movers.find((r) => isNewEntry(trends[r.fullName]));
  if (!repo) return null;
  const points = trends[repo.fullName] ?? [];
  return { repo, series: dailySeries(points), days: Math.max(1, points.length) };
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
