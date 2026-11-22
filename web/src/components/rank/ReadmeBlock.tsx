'use client';

import { useEffect, useRef, useState } from 'react';
import type { ReadmeHeading } from '@/lib/data';

// Tall enough to read a section without scrolling, capped so the page below
// (artifacts, related repos) stays reachable on short viewports.
const VIEWPORT = 'min(72vh, 760px)';

/**
 * README panel: scrollable body with an outline alongside it.
 *
 * The body scrolls inside its own box rather than the page, so the outline
 * scrolls that container directly instead of using href anchors — a plain
 * `#id` link would also jump the page and leave a hash in the URL.
 */
export function ReadmeBlock({ html, toc }: { html: string; toc: ReadmeHeading[] }) {
  const bodyRef = useRef<HTMLDivElement>(null);
  const [activeId, setActiveId] = useState<string | null>(toc[0]?.id ?? null);

  useEffect(() => {
    const body = bodyRef.current;
    if (!body || toc.length === 0) return;

    // Highlight the heading nearest the top of the scroll box.
    const observer = new IntersectionObserver(
      (entries) => {
        const visible = entries.filter((e) => e.isIntersecting);
        if (visible.length > 0) {
          const top = visible.reduce((a, b) =>
            a.boundingClientRect.top < b.boundingClientRect.top ? a : b,
          );
          setActiveId(top.target.id);
        }
      },
      // Narrow band near the top of the container: a heading is "current" once
      // it reaches the top and stays so until the next one arrives.
      { root: body, rootMargin: '0px 0px -80% 0px', threshold: 0 },
    );

    for (const h of toc) {
      const el = body.querySelector(`#${CSS.escape(h.id)}`);
      if (el) observer.observe(el);
    }
    return () => observer.disconnect();
  }, [toc]);

  function jumpTo(id: string) {
    const body = bodyRef.current;
    const target = body?.querySelector(`#${CSS.escape(id)}`);
    if (!body || !target) return;
    // Position within the scroll box, offset by where the box currently sits.
    const top =
      target.getBoundingClientRect().top - body.getBoundingClientRect().top + body.scrollTop - 12;
    // Plain assignment, and deliberately no smooth scrolling: both
    // scrollTo({behavior:'smooth'}) and CSS scroll-behavior turned out to be
    // silent no-ops on this container in Chromium, which left outline clicks
    // doing nothing at all. An instant jump always lands.
    body.scrollTop = top;
    setActiveId(id);
  }

  return (
    <section aria-labelledby="readme" className="flex min-w-0 flex-col">
      <h2 id="readme" className="border-t-2 border-ink py-3 text-[18px] font-semibold">
        README
      </h2>

      <div className="flex overflow-hidden rounded border border-rule bg-sr-surface" style={{ height: VIEWPORT }}>
        {toc.length > 1 && (
          <nav
            aria-label="README outline"
            className="hidden w-56 shrink-0 overflow-y-auto overscroll-contain border-r border-rule py-3 md:block"
          >
            <ul className="flex flex-col gap-px pr-2">
              {toc.map((h) => (
                <li key={h.id}>
                  <button
                    onClick={() => jumpTo(h.id)}
                    title={h.text}
                    aria-current={activeId === h.id ? 'true' : undefined}
                    className={`block w-full truncate rounded-r-md border-l-2 py-1 pr-2 text-left text-[12px] leading-snug transition-colors ${
                      activeId === h.id
                        ? 'border-ink bg-sunk font-semibold text-ink'
                        : 'border-transparent text-ink-3 hover:bg-sunk hover:text-ink'
                    }`}
                    // Indent by heading level; h1 sits flush with the panel edge.
                    style={{ paddingLeft: `${(h.depth - 1) * 12 + 12}px` }}
                  >
                    {h.text}
                  </button>
                </li>
              ))}
            </ul>
          </nav>
        )}

        <div
          ref={bodyRef}
          className="readme-body min-w-0 flex-1 overflow-y-auto overscroll-contain px-5 py-[18px]"
          dangerouslySetInnerHTML={{ __html: html }}
        />
      </div>
    </section>
  );
}
