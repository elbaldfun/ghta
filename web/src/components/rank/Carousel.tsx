'use client';

import { useRef, type ReactNode } from 'react';
import { ChevronLeft, ChevronRight } from './icons';

/**
 * Horizontal card scroller with prev/next arrows absolutely positioned OVER the
 * card layer (2a pattern). Arrows scroll by ~85% of the visible width.
 */
export function Carousel({ children, ariaLabel }: { children: ReactNode; ariaLabel?: string }) {
  const rowRef = useRef<HTMLDivElement>(null);

  function scrollRow(dir: -1 | 1) {
    const el = rowRef.current;
    if (el) el.scrollBy({ left: dir * Math.max(260, Math.round(el.clientWidth * 0.85)), behavior: 'smooth' });
  }

  const arrowClass =
    'absolute top-1/2 z-[5] flex h-[38px] w-[38px] -translate-y-1/2 items-center justify-center ' +
    'rounded-full border border-border bg-surface text-fg shadow-arrow';

  return (
    <div className="relative" role="region" aria-label={ariaLabel}>
      <button onClick={() => scrollRow(-1)} className={`${arrowClass} left-[-6px]`} aria-label="Scroll left">
        <ChevronLeft size={16} />
      </button>
      <div
        ref={rowRef}
        className="scrollbar-hide flex gap-3.5 overflow-x-auto scroll-smooth px-px pb-1.5 pt-[3px]"
      >
        {children}
      </div>
      <button onClick={() => scrollRow(1)} className={`${arrowClass} right-[-6px]`} aria-label="Scroll right">
        <ChevronRight size={16} />
      </button>
    </div>
  );
}
