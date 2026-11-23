'use client';

import { useEffect, useRef, useState } from 'react';

/**
 * An app's screenshot at 16:10. With no screenshot, or one that fails to
 * load, the frame stays and says so, so the grid keeps its rhythm.
 */
export function AppShot({ src, alt, missing }: { src?: string; alt: string; missing: string }) {
  const [failed, setFailed] = useState(false);
  const img = useRef<HTMLImageElement>(null);
  // The image can fail before hydration attaches onError (a blocked
  // cross-origin file fails fast); catch that once on mount.
  useEffect(() => {
    const el = img.current;
    if (el && el.complete && el.naturalWidth === 0) setFailed(true);
  }, [src]);
  const show = src && !failed;
  return (
    <div className="relative aspect-[16/10] overflow-hidden rounded-lg border border-rule bg-sunk [background-image:repeating-linear-gradient(135deg,transparent_0_14px,rgb(var(--sr-rule))_14px_15px)]">
      {show ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          ref={img}
          src={src}
          alt={alt}
          loading="lazy"
          decoding="async"
          onError={() => setFailed(true)}
          className="absolute inset-0 h-full w-full bg-sunk object-cover object-top"
        />
      ) : (
        <span className="absolute bottom-3 left-3 rounded border border-rule bg-paper px-1.5 py-0.5 font-code text-[11px] text-ink-3">
          {missing}
        </span>
      )}
    </div>
  );
}
