'use client';

import { useState } from 'react';

/**
 * An app's icon with graceful fallback: the app's own brand icon (extracted from
 * its homepage), then the repo owner's GitHub avatar, then a colored letter tile — so a
 * broken or missing icon never leaves an empty box. Client-side because it reacts
 * to <img> load errors.
 */
export function AppIcon({
  iconUrl,
  owner,
  title,
  size = 44,
}: {
  iconUrl?: string;
  owner: string;
  title: string;
  size?: number;
}) {
  const avatar = `https://github.com/${owner}.png?size=88`;
  // Try the brand icon first, then the owner avatar, then give up to the letter.
  const [src, setSrc] = useState(iconUrl || avatar);
  const [failed, setFailed] = useState(false);

  if (failed) {
    // A hue picked from the name, so each app keeps the same color everywhere
    // and a row of fallbacks does not read as one grey block.
    let hash = 0;
    for (const ch of title) hash = (hash * 31 + ch.charCodeAt(0)) >>> 0;
    return (
      <span
        className="flex shrink-0 items-center justify-center rounded-[10px] font-display font-extrabold text-white"
        style={{ width: size, height: size, fontSize: size * 0.44, backgroundColor: `hsl(${hash % 360} 55% 42%)` }}
        aria-hidden="true"
      >
        {(title.match(/[\p{L}\p{N}]/u)?.[0] ?? '?').toUpperCase()}
      </span>
    );
  }

  return (
    // eslint-disable-next-line @next/next/no-img-element
    <img
      src={src}
      alt=""
      loading="lazy"
      width={size}
      height={size}
      onError={() => {
        if (src !== avatar) setSrc(avatar);
        else setFailed(true);
      }}
      className="shrink-0 rounded-[10px] border border-border bg-surface2 object-cover"
      style={{ width: size, height: size }}
    />
  );
}
