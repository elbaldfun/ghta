// Blog posts are Markdown files in content/blog/<locale>/<slug>.md, kept in the
// repo so they are version-controlled and ship with a normal deploy.
//
// Posts may embed live data from our own database via fenced blocks:
//
//     ```starrank:languages
//     limit: 8
//     ```
//
// renderPostBody splits the body on those blocks so the page can interleave
// rendered Markdown with server components. The set of block types is fixed
// (see DataBlockName) — posts cannot execute arbitrary code.

import fs from 'node:fs/promises';
import path from 'node:path';
import matter from 'gray-matter';
import { marked } from 'marked';
import sanitizeHtml from 'sanitize-html';

const BLOG_DIR = path.join(process.cwd(), 'content', 'blog');

export const DATA_BLOCKS = ['languages', 'top-repos'] as const;
export type DataBlockName = (typeof DATA_BLOCKS)[number];

/** Language articles are authored in; everything else is translated from it. */
export const SOURCE_LOCALE = 'en';

export interface PostMeta {
  slug: string;
  title: string;
  description: string;
  /** ISO date; drives ordering and the article's published time. */
  date: string;
  tags: string[];
  readingMinutes: number;
  /** Locale the text was translated from, when it is not an original. */
  translatedFrom?: string;
  /** True when this locale has no file and the English original is shown instead. */
  isFallback?: boolean;
}

export interface Post extends PostMeta {
  body: string;
}

/** A rendered chunk of Markdown, or a placeholder for a live-data component. */
export type PostSegment =
  | { kind: 'html'; html: string }
  | { kind: 'data'; name: DataBlockName; params: Record<string, string> };

const FENCE_RE = /^```starrank:([a-z-]+)\n([\s\S]*?)```$/gm;

function sanitize(html: string): string {
  return sanitizeHtml(html, {
    allowedTags: sanitizeHtml.defaults.allowedTags.concat(['img', 'figure', 'figcaption']),
    allowedAttributes: {
      ...sanitizeHtml.defaults.allowedAttributes,
      img: ['src', 'alt', 'title', 'width', 'height'],
      a: ['href', 'target', 'rel'],
    },
  });
}

/** ~250 words/min for latin text; CJK has no spaces so fall back to characters. */
function readingMinutes(body: string): number {
  const cjk = (body.match(/[一-鿿]/g) ?? []).length;
  const words = body.split(/\s+/).filter(Boolean).length;
  return Math.max(1, Math.round(cjk / 400 + (cjk > 0 ? 0 : words / 250)));
}

async function readPostFile(locale: string, slug: string): Promise<Post | null> {
  const file = path.join(BLOG_DIR, locale, `${slug}.md`);
  let raw: string;
  try {
    raw = await fs.readFile(file, 'utf8');
  } catch {
    return null; // No such post for this locale — a normal 404.
  }

  try {
    const { data, content } = matter(raw);
    if (data.draft) return null;
    return {
      slug,
      title: String(data.title ?? slug),
      description: String(data.description ?? ''),
      // YAML parses an unquoted date into a Date object; keep the ISO day so it
      // renders and sorts as a plain YYYY-MM-DD rather than a locale timestamp.
      date: data.date instanceof Date ? data.date.toISOString().slice(0, 10) : String(data.date ?? ''),
      tags: Array.isArray(data.tags) ? data.tags.map(String) : [],
      readingMinutes: readingMinutes(content),
      translatedFrom: data.translatedFrom ? String(data.translatedFrom) : undefined,
      body: content,
    };
  } catch (e) {
    // Malformed front matter would otherwise make the post vanish from the site
    // with no signal at all — an unquoted "title: A: B" is enough to do it.
    console.error(`[blog] failed to parse ${file}:`, e instanceof Error ? e.message : e);
    return null;
  }
}

async function slugsFor(locale: string): Promise<string[]> {
  try {
    const files = await fs.readdir(path.join(BLOG_DIR, locale));
    return files.filter((f) => f.endsWith('.md')).map((f) => f.replace(/\.md$/, ''));
  } catch {
    return [];
  }
}

/**
 * Post metadata for a locale, newest first.
 *
 * The catalogue is defined by the source locale, and each entry falls back to
 * the original per post — so a newly published article is visible in every
 * language immediately instead of waiting on all eight translations.
 */
export async function listPosts(locale: string): Promise<PostMeta[]> {
  const slugs = new Set([...(await slugsFor(SOURCE_LOCALE)), ...(await slugsFor(locale))]);
  const posts = await Promise.all([...slugs].map((slug) => getPost(locale, slug)));
  return posts
    .filter((p): p is Post => p !== null)
    .sort((a, b) => b.date.localeCompare(a.date))
    .map(({ body: _body, ...meta }) => meta);
}

export async function getPost(locale: string, slug: string): Promise<Post | null> {
  // Slugs come from the URL; keep them to a flat, safe shape so a crafted path
  // can never escape the content directory.
  if (!/^[a-z0-9-]+$/.test(slug)) return null;

  const localized = await readPostFile(locale, slug);
  if (localized) return localized;
  if (locale === SOURCE_LOCALE) return null;

  const original = await readPostFile(SOURCE_LOCALE, slug);
  return original && { ...original, isFallback: true };
}

/** Split a post body into rendered Markdown and live-data placeholders. */
export async function renderPostBody(body: string): Promise<PostSegment[]> {
  const segments: PostSegment[] = [];
  let lastIndex = 0;

  const pushMarkdown = async (md: string) => {
    if (!md.trim()) return;
    segments.push({ kind: 'html', html: sanitize(await marked.parse(md, { gfm: true })) });
  };

  for (const match of body.matchAll(FENCE_RE)) {
    const [full, name, paramText] = match;
    await pushMarkdown(body.slice(lastIndex, match.index));
    lastIndex = match.index! + full.length;

    if (!DATA_BLOCKS.includes(name as DataBlockName)) {
      // Unknown block: skip it rather than rendering raw markup into the page.
      continue;
    }
    const params: Record<string, string> = {};
    for (const line of paramText.split('\n')) {
      const sep = line.indexOf(':');
      if (sep === -1) continue;
      params[line.slice(0, sep).trim()] = line.slice(sep + 1).trim();
    }
    segments.push({ kind: 'data', name: name as DataBlockName, params });
  }
  await pushMarkdown(body.slice(lastIndex));

  return segments;
}
