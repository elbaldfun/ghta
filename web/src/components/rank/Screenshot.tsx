'use client';

import { useState } from 'react';

/**
 * The detail page's app screenshot (change 15 v2b). Heuristics run at ~80%
 * precision, so this stays a detail-page feature (full context around it) and
 * a broken or blocked image removes itself instead of leaving a hole.
 */
export function Screenshot({ url, alt }: { url: string; alt: string }) {
  const [failed, setFailed] = useState(false);
  if (failed) return null;

  return (
    <a
      href={url}
      target="_blank"
      rel="noopener noreferrer"
      className="block overflow-hidden rounded-card border border-border bg-surface"
    >
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src={url}
        alt={alt}
        loading="lazy"
        onError={() => setFailed(true)}
        className="max-h-[440px] w-full object-contain"
      />
    </a>
  );
}
