'use client';

import { useEffect, useRef, useState } from 'react';
import { useTranslations } from 'next-intl';
import { FileIcon } from './icons';

const CAP_PX = 440;

/**
 * README panel with the 2a collapse behavior: body capped to ~440px with a
 * bottom fade while collapsed; the toggle only appears when content overflows.
 * `html` is GitHub's own rendered + sanitized README HTML.
 */
export function ReadmeBlock({ html }: { html: string }) {
  const t = useTranslations('rank');
  const bodyRef = useRef<HTMLDivElement>(null);
  const [expanded, setExpanded] = useState(false);
  const [overflows, setOverflows] = useState(false);

  useEffect(() => {
    const el = bodyRef.current;
    if (!el) return;
    const check = () => setOverflows(el.scrollHeight > CAP_PX + 40);
    check();
    // README images load late and change the height.
    const observer = new ResizeObserver(check);
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  const collapsed = overflows && !expanded;

  return (
    <section className="mt-6 overflow-hidden rounded-card border border-border bg-surface">
      <div className="flex items-center justify-between gap-2.5 border-b border-border bg-surface2 py-[9px] pl-4 pr-3">
        <span className="flex items-center gap-2">
          <FileIcon size={15} className="text-muted" />
          <span className="text-xs font-bold tracking-wide">README</span>
        </span>
        {overflows && (
          <button
            onClick={() => setExpanded((v) => !v)}
            className="flex items-center gap-1 whitespace-nowrap rounded-md border border-border bg-surface px-2.5 py-[3px] text-[11px] font-semibold text-accent"
          >
            {expanded ? t('readmeCollapse') : t('readmeExpand')}
          </button>
        )}
      </div>
      <div className="relative" style={collapsed ? { maxHeight: CAP_PX, overflow: 'hidden' } : undefined}>
        <div
          ref={bodyRef}
          className="readme-body px-5 py-[18px]"
          dangerouslySetInnerHTML={{ __html: html }}
        />
        {collapsed && (
          <button
            onClick={() => setExpanded(true)}
            aria-label={t('readmeExpand')}
            className="absolute inset-x-0 bottom-0 h-20 cursor-pointer"
            style={{ background: 'linear-gradient(to bottom, transparent, rgb(var(--surface)))' }}
          />
        )}
      </div>
    </section>
  );
}
