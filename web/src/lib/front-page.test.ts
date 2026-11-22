// Run with `npm test` (node's built-in runner; Node 22 strips the types).
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { dailySeries, daysTracked, isNewEntry, pickFresh, pickLead } from './front-page.ts';

const DAY = 86_400_000;
const T0 = Date.UTC(2026, 10, 1, 7, 30); // 2026-11-01 07:30 UTC
const at = (day: number, v: number, hour = 0) => ({ t: T0 + day * DAY + hour * 3_600_000, v });

test('daily readings give one gain per day', () => {
  assert.deepEqual(dailySeries([at(0, 100), at(1, 110), at(2, 125)]), [10, 15]);
});

test('every-other-day readings are spread over both days', () => {
  // 100 → 120 over two days, then 120 → 121 over two days.
  assert.deepEqual(dailySeries([at(0, 100), at(2, 120), at(4, 121)]), [10, 10, 0, 1]);
});

test('the last day of a span matches ComputeGrowth d1 (Go rounding)', () => {
  // d1 = math.Round(1/2) = 1 and math.Round(-1/2) = -1 in Go.
  assert.deepEqual(dailySeries([at(0, 100), at(2, 101)]), [0, 1]);
  assert.deepEqual(dailySeries([at(0, 100), at(2, 99)]), [0, -1]);
  assert.deepEqual(dailySeries([at(0, 0), at(3, 5)]).at(-1), 2); // round(5/3)
});

test('spread gains still add up to the real gain', () => {
  const s = dailySeries([at(0, 0), at(3, 10)]);
  assert.equal(s.length, 3);
  assert.equal(s.reduce((a, b) => a + b, 0), 10);
});

test('a long gap fills every missing day', () => {
  const s = dailySeries([at(0, 0), at(8, 800), at(17, 1700)]);
  assert.equal(s.length, 17);
  assert.ok(s.every((v) => v === 100));
});

test('two readings on one day: the later one counts', () => {
  assert.deepEqual(dailySeries([at(0, 100), at(1, 110), at(1, 112, 5), at(2, 120)]), [12, 8]);
  // Also after a gap: day 2's later reading replaces its earlier one.
  assert.deepEqual(dailySeries([at(0, 100), at(2, 120), at(2, 124, 5)]), [12, 12]);
  // And on the first day.
  assert.deepEqual(dailySeries([at(0, 90), at(0, 100, 5), at(1, 110)]), [10]);
});

test('days split at UTC midnight', () => {
  const late = Date.UTC(2026, 10, 1, 23, 0);
  const early = Date.UTC(2026, 10, 2, 1, 0); // two hours later, next UTC day
  assert.deepEqual(dailySeries([{ t: late, v: 0 }, { t: early, v: 5 }]), [5]);
});

test('losses are kept', () => {
  assert.deepEqual(dailySeries([at(0, 100), at(2, 96)]), [-2, -2]);
});

test('fewer than two readings give no series', () => {
  assert.deepEqual(dailySeries(undefined), []);
  assert.deepEqual(dailySeries([at(0, 1)]), []);
});

test('days tracked count the first reading as day 1', () => {
  assert.equal(daysTracked([at(0, 1)], T0), 1);
  assert.equal(daysTracked([at(0, 1)], T0 + 6 * DAY), 7);
});

test('new means first read under 7 days ago, not few readings', () => {
  const now = T0 + 13 * DAY;
  // Read every other day for two weeks: only 7 readings, but 14 days old.
  const oldSparse = [0, 2, 4, 6, 8, 10, 12].map((d) => at(d, d * 10));
  assert.equal(oldSparse.length, 7);
  assert.equal(isNewEntry(oldSparse, now), false);
  // Four readings 8–9 days apart: a month old, not new.
  assert.equal(isNewEntry([at(-13, 0), at(-5, 1), at(4, 2), at(13, 3)], now), false);
  // First read 5 days ago.
  assert.equal(isNewEntry([at(8, 0), at(10, 5), at(12, 9)], now), true);
  // No readings at all is unknown, not new.
  assert.equal(isNewEntry(undefined, now), false);
  assert.equal(isNewEntry([], now), false);
});

const repo = (fullName: string, d1: number, d7: number | null) => {
  const [owner, name] = fullName.split('/');
  return {
    owner, name, fullName, description: null, language: null, stars: 1000, forks: 0, openIssues: 0,
    license: null, homepage: null, topics: [], type: null, categoryPath: [], pushedAt: '', createdAt: null,
    htmlUrl: '', dailyIncrease: d1, weeklyIncrease: d7,
  };
};

test('lead and fresh skip candidates without a trend', () => {
  const now = T0 + 30 * DAY;
  const movers = [repo('a/no-trend', 900, 2000), repo('b/old', 800, 1500), repo('c/new', 700, null)];
  const trends = {
    'b/old': Array.from({ length: 16 }, (_, i) => at(i * 2, i * 100)),
    'c/new': [at(28, 0), at(30, 700)],
  };
  assert.equal(pickLead(movers, trends, now)?.repo.fullName, 'b/old');
  const fresh = pickFresh(movers, trends, now);
  assert.equal(fresh?.repo.fullName, 'c/new');
  assert.equal(fresh?.days, 3);
});
