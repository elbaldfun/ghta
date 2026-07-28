#!/usr/bin/env node
// Layout guard: page width and horizontal gutter live ONLY in PageShell.
// Hand-written magic paddings drifted three ways (28px header vs 26px pages vs
// 16+10px sidebar) and every layout change re-broke edge alignment — so any
// re-introduction now fails the build with a pointer to the right primitive.
import { readdirSync, readFileSync } from 'node:fs';
import { join, relative } from 'node:path';

const ROOT = join(import.meta.dirname, '..', 'src');
const ALLOWED = new Set(['components/rank/PageShell.tsx']);

// Banned outside PageShell: the container width, the legacy gutter values,
// and raw px-gutter in page files (pages compose PageShell instead; the token
// is allowed only in components that align full-bleed sections' inner rows).
const RULES = [
  { re: /max-w-screen-xl/, msg: 'page width belongs to <PageShell> only' },
  { re: /px-\[26px\]/, msg: 'legacy 26px gutter — use <PageShell> (28px)' },
  { re: /(?<![\w[-])px-7(?![\w.\]-])/, msg: 'raw px-7 gutter — use <PageShell>' },
];

const violations = [];
function walk(dir) {
  for (const e of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, e.name);
    if (e.isDirectory()) walk(p);
    else if (/\.(tsx|ts|css)$/.test(e.name)) check(p);
  }
}
function check(file) {
  const rel = relative(ROOT, file).replaceAll('\\', '/');
  if (ALLOWED.has(rel)) return;
  const lines = readFileSync(file, 'utf8').split('\n');
  lines.forEach((line, i) => {
    for (const { re, msg } of RULES) {
      if (re.test(line)) violations.push(`${rel}:${i + 1}  ${msg}\n    ${line.trim().slice(0, 100)}`);
    }
  });
}

walk(ROOT);
if (violations.length) {
  console.error(`layout-guard: ${violations.length} violation(s):\n`);
  for (const v of violations) console.error('  ' + v + '\n');
  process.exit(1);
}
console.log('layout-guard: OK');
