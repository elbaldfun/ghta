'use client';

import { useState, type ReactNode } from 'react';

/**
 * Two server-rendered views of one figure (daily gains, cumulative stars) and
 * a segmented switch between them. Both are in the HTML; the switch only
 * hides one, so the page stays static and crawlable.
 */
export function ChartToggle({
  caption,
  label,
  views,
}: {
  caption: ReactNode;
  /** Names the switch for screen readers. */
  label: string;
  views: { key: string; label: string; body: ReactNode }[];
}) {
  const [on, setOn] = useState(views[0]?.key);
  return (
    <figure className="m-0 flex flex-col gap-2.5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <figcaption className="min-w-0 text-[16px] font-semibold leading-snug">{caption}</figcaption>
        <div role="group" aria-label={label} className="flex shrink-0 overflow-hidden rounded border border-ink">
          {views.map((v) => (
            <button
              key={v.key}
              type="button"
              aria-pressed={on === v.key}
              onClick={() => setOn(v.key)}
              className={`h-11 px-3 text-[13px] transition-colors duration-150 md:h-8 ${
                on === v.key ? 'bg-ink text-paper' : 'text-ink-2 hover:bg-sunk hover:text-ink'
              }`}
            >
              {v.label}
            </button>
          ))}
        </div>
      </div>
      {views.map((v) => (
        <div key={v.key} hidden={on !== v.key}>
          {v.body}
        </div>
      ))}
    </figure>
  );
}
