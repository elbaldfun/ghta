// Run with `npm test`.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  community,
  daysSince,
  licenseKind,
  licenseShort,
  pace,
  peakDay,
  recentDaily,
  trackedPoints,
  upkeep,
} from './repo-facts.ts';

const DAY = 86_400_000;

test('monthly backfill points are not tracked readings', () => {
  const pts = [
    { t: Date.UTC(2026, 7, 1), v: 1 },
    { t: Date.UTC(2026, 8, 1), v: 22 },
    { t: Date.UTC(2026, 9, 5, 4, 58), v: 2724 },
    { t: Date.UTC(2026, 9, 6, 7, 56), v: 6360 },
  ];
  assert.deepEqual(trackedPoints(pts).map((p) => p.v), [2724, 6360]);
  assert.deepEqual(trackedPoints(pts.slice(0, 2)), []);
});

test('recent window ends on the last reading day and dates each value', () => {
  const t0 = Date.UTC(2026, 10, 1, 3, 30);
  const pts = [0, 1, 2, 3].map((d) => ({ t: t0 + d * DAY, v: d * 10 }));
  const w = recentDaily(pts, 2);
  assert.deepEqual(w.series, [10, 10]);
  assert.equal(w.dayOf(1), Date.UTC(2026, 10, 4));
  assert.equal(w.dayOf(0), Date.UTC(2026, 10, 3));
});

test('pace compares the last week with the window’s first', () => {
  const cooling = [...Array(7).fill(900), ...Array(17).fill(500), ...Array(7).fill(300)];
  assert.equal(pace(cooling).momentum, 'cooling');
  assert.equal(pace(cooling).before, 900);
  assert.equal(pace(cooling).recent, 300);
  const rising = [...Array(7).fill(10), ...Array(7).fill(40)];
  assert.equal(pace(rising).momentum, 'rising');
  assert.equal(pace(Array(20).fill(50)).momentum, 'steady');
  assert.equal(pace(Array(13).fill(50)).momentum, 'new');
  // A repo that gained nothing before is "rising" only from a real gain.
  assert.equal(pace([...Array(7).fill(0), ...Array(7).fill(1)]).momentum, 'steady');
});

test('upkeep follows the last push', () => {
  assert.equal(upkeep(0), 'active');
  assert.equal(upkeep(30), 'active');
  assert.equal(upkeep(31), 'slowing');
  assert.equal(upkeep(181), 'dormant');
  assert.equal(upkeep(null), 'unknown');
});

test('days since counts calendar days and rejects junk', () => {
  const now = Date.UTC(2026, 10, 22, 10);
  assert.equal(daysSince('2026-11-22T03:27:28Z', now), 0);
  assert.equal(daysSince('2026-11-21T23:59:00Z', now), 1);
  assert.equal(daysSince('', now), null);
  assert.equal(daysSince('not a date', now), null);
});

test('community by forks', () => {
  assert.equal(community(115_421), 'large');
  assert.equal(community(1_000), 'medium');
  assert.equal(community(120), 'small');
  assert.equal(community(3), 'tiny');
});

test('licenses', () => {
  assert.equal(licenseKind('MIT License'), 'permissive');
  assert.equal(licenseKind('GNU General Public License v3.0'), 'copyleft');
  assert.equal(licenseKind('GNU Lesser General Public License v2.1'), 'copyleft');
  assert.equal(licenseKind('Other'), 'other');
  assert.equal(licenseKind(null), 'none');
  assert.equal(licenseShort('MIT License'), 'MIT');
  assert.equal(licenseShort('Apache License 2.0'), 'Apache-2.0');
  assert.equal(licenseShort('GNU Affero General Public License v3.0'), 'AGPL-3.0');
  assert.equal(licenseShort('GNU General Public License v3.0'), 'GPL-3.0');
  assert.equal(licenseShort('Other'), 'Other');
});

test('peak day is only news when it is not today', () => {
  assert.equal(peakDay([5, 9, 3]), 1);
  assert.equal(peakDay([1, 2, 3]), null);
  assert.equal(peakDay([0, 0]), null);
});
