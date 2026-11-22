// Number formatting for design tokens v1. One rule per kind of number so a
// column never mixes styles: increments are written in full (▲1,429), totals
// are compacted to one decimal (471.6k, 1.2M).
import { formatCompact } from '@/lib/rank-data';

/** A daily gain at or above this is "hot" and earns the signal colour. */
export const HOT_DAILY = 1000;

const int = new Intl.NumberFormat('en-US');

/** 1429 → "1,429". Grouping is fixed to en-US so columns align in every locale. */
export function fmtInt(n: number): string {
  return int.format(Math.round(n));
}

/** 471630 → "471.6k". */
export function fmtTotal(n: number): string {
  return formatCompact(n);
}
