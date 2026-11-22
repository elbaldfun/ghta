// The detail page's "worth using · will it last" answers. Pure functions
// over the repository and its star history, so each answer is a rule a reader
// can check on GitHub: no score, no weighting. Answers are message keys; the
// page translates them.
import type { StarPoint } from '@/lib/data';
import { dailySeries } from './front-page.ts';

const DAY = 86_400_000;

/** Whole days from an ISO time (or unix ms) to `now`, never negative; null when unknown. */
export function daysSince(at: string | number | null | undefined, now: number): number | null {
  if (at == null || at === '') return null;
  const ms = typeof at === 'number' ? at : Date.parse(at);
  if (!Number.isFinite(ms)) return null;
  return Math.max(0, Math.floor(now / DAY) - Math.floor(ms / DAY));
}

/**
 * The star history's own daily readings. Before a repository is tracked the
 * history is backfilled with one point at 00:00 UTC on the first of each
 * month; those are skipped so that a month is never spread into "days".
 */
export function trackedPoints(points: StarPoint[]): StarPoint[] {
  const i = points.findIndex((p) => {
    const d = new Date(p.t);
    return !(d.getUTCDate() === 1 && p.t % DAY === 0);
  });
  return i < 0 ? [] : points.slice(i);
}

export interface Window {
  /** Stars gained per day, oldest first, at most `days` long. */
  series: number[];
  /** Unix ms of the day each value belongs to. */
  dayOf: (i: number) => number;
}

/** The last `days` days of daily gains, from tracked readings only. */
export function recentDaily(points: StarPoint[], days = 31): Window {
  const tracked = trackedPoints(points);
  const series = dailySeries(tracked).slice(-days);
  const last = tracked.length ? Math.floor(tracked[tracked.length - 1].t / DAY) * DAY : 0;
  return { series, dayOf: (i) => last - (series.length - 1 - i) * DAY };
}

const avg = (xs: number[]) => (xs.length ? xs.reduce((a, b) => a + b, 0) / xs.length : 0);

export type Momentum = 'new' | 'rising' | 'cooling' | 'steady';

export interface Pace {
  momentum: Momentum;
  /** Daily average of the window's first 7 days and of its last 7. */
  before: number;
  recent: number;
}

/**
 * Rising or cooling: the last 7 days' daily average against the first 7
 * days of the window. Under two weeks of readings there is nothing to
 * compare, so the repo is "new". 1.5× either way counts as a change.
 */
export function pace(series: number[]): Pace {
  const recent = avg(series.slice(-7));
  if (series.length < 14) return { momentum: 'new', before: 0, recent };
  const before = avg(series.slice(0, 7));
  const base = Math.max(before, 1);
  const momentum = recent >= base * 1.5 ? 'rising' : recent <= before / 1.5 ? 'cooling' : 'steady';
  return { momentum, before, recent };
}

export type Upkeep = 'active' | 'slowing' | 'dormant' | 'unknown';

/** From the last push: within 30 days is maintained, within 180 is slowing. */
export function upkeep(pushedDays: number | null): Upkeep {
  if (pushedDays == null) return 'unknown';
  if (pushedDays <= 30) return 'active';
  if (pushedDays <= 180) return 'slowing';
  return 'dormant';
}

export type Community = 'large' | 'medium' | 'small' | 'tiny';

/** Forks are people who took the code to build on it. */
export function community(forks: number): Community {
  if (forks >= 10_000) return 'large';
  if (forks >= 1_000) return 'medium';
  if (forks >= 100) return 'small';
  return 'tiny';
}

export type LicenseKind = 'permissive' | 'copyleft' | 'other' | 'none';

const PERMISSIVE = /\b(MIT|Apache|BSD|ISC|Unlicense|Zlib|Boost|0BSD|CC0|WTFPL)\b/i;
const COPYLEFT = /\b(A?GPL|LGPL|GNU|MPL|Mozilla|EUPL|EPL|Eclipse|CDDL|OSL)\b/i;

export function licenseKind(license: string | null | undefined): LicenseKind {
  if (!license) return 'none';
  if (COPYLEFT.test(license)) return 'copyleft';
  if (PERMISSIVE.test(license)) return 'permissive';
  return 'other';
}

/** "MIT License" → "MIT", "Apache License 2.0" → "Apache-2.0"; others as given. */
export function licenseShort(license: string | null | undefined): string | null {
  if (!license) return null;
  const rules: [RegExp, string][] = [
    [/^MIT\b/i, 'MIT'],
    [/^Apache License 2\.0$/i, 'Apache-2.0'],
    [/Affero General Public License v3/i, 'AGPL-3.0'],
    [/Lesser General Public License v2\.1/i, 'LGPL-2.1'],
    [/Lesser General Public License v3/i, 'LGPL-3.0'],
    [/General Public License v3/i, 'GPL-3.0'],
    [/General Public License v2/i, 'GPL-2.0'],
    [/Mozilla Public License 2\.0/i, 'MPL-2.0'],
    [/BSD 3-Clause/i, 'BSD-3-Clause'],
    [/BSD 2-Clause/i, 'BSD-2-Clause'],
    [/^The Unlicense$/i, 'Unlicense'],
    [/^ISC License$/i, 'ISC'],
  ];
  for (const [re, short] of rules) if (re.test(license)) return short;
  return license;
}

/** Day with the most stars in the window, when that is not the last day. */
export function peakDay(series: number[]): number | null {
  if (series.length < 2) return null;
  let best = 0;
  for (let i = 1; i < series.length; i++) if (series[i] > series[best]) best = i;
  return best === series.length - 1 || series[best] <= 0 ? null : best;
}
