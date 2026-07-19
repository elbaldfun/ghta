'use client';

import { useState } from 'react';
import { useTranslations } from 'next-intl';
import { useRouter } from '@/i18n/navigation';
import { useSearchParams } from 'next/navigation';
import { TAXONOMY, formatCompact } from '@/lib/rank-data';
import type { CategoryCounts } from '@/lib/data';
import { GridIcon } from './icons';

/**
 * Two-level taxonomy tree (2a sidebar). Selecting a node filters the grid and
 * resets to page 1; other filters (language/license/sort) are preserved.
 */
export function CategoryTree({ counts }: { counts: CategoryCounts }) {
  const t = useTranslations('rank');
  const router = useRouter();
  const params = useSearchParams();
  const activeCat = params.get('cat');
  const activeSub = params.get('sub');
  const [expanded, setExpanded] = useState<Record<string, boolean>>(() =>
    activeCat ? { [activeCat]: true } : { frontend: true, ai: true },
  );

  function navigate(cat: string | null, sub: string | null) {
    const next = new URLSearchParams(params.toString());
    next.delete('page');
    next.delete('q');
    if (cat) next.set('cat', cat);
    else next.delete('cat');
    if (sub) next.set('sub', sub);
    else next.delete('sub');
    const qs = next.toString();
    router.push(qs ? `/?${qs}` : '/');
  }

  const badge = (n: number | null | undefined) =>
    n === null || n === undefined ? null : (
      <span className="rounded-full bg-surface2 px-2 py-px text-[11px] text-muted">{formatCompact(n)}</span>
    );

  const allActive = !activeCat;

  return (
    <aside className="border-r border-border bg-surface px-4 py-5">
      <button
        onClick={() => navigate(null, null)}
        className={`mb-2 flex w-full items-center justify-between rounded-lg px-2.5 py-2 ${
          allActive ? 'bg-surface2' : 'hover:bg-surface2/60'
        }`}
      >
        <span
          className={`flex items-center gap-2 text-[13px] ${allActive ? 'font-bold text-accent' : 'font-semibold text-fg'}`}
        >
          <GridIcon size={15} />
          {t('browseAll')}
        </span>
        {badge(counts.all)}
      </button>

      {TAXONOMY.map((group) => {
        const isOpen = !!expanded[group.id];
        const isActiveCat = activeCat === group.id && !activeSub;
        return (
          <div key={group.id} className="mt-0.5">
            <button
              onClick={() => {
                setExpanded((e) => ({ ...e, [group.id]: activeCat === group.id ? !isOpen : true }));
                navigate(group.id, null);
              }}
              className={`flex w-full items-center justify-between gap-1.5 rounded-lg px-2.5 py-2 ${
                isActiveCat ? 'bg-surface2' : 'hover:bg-surface2/60'
              }`}
            >
              <span
                className={`flex items-center gap-[7px] text-[13px] ${
                  isActiveCat ? 'font-bold text-accent' : 'font-semibold text-fg'
                }`}
              >
                <span className="inline-block w-2.5 text-[9px] text-muted">{isOpen ? '▾' : '▸'}</span>
                {t(`cats.${group.id}`)}
              </span>
              <span className="text-[11px] text-muted">
                {counts.cats[group.id] !== null ? formatCompact(counts.cats[group.id]!) : ''}
              </span>
            </button>
            {isOpen && group.subs && (
              <div className="ml-4 mt-0.5 flex flex-col gap-0.5 border-l border-border pl-1.5">
                {group.subs.map((node) => {
                  const isActive = activeSub === node.id;
                  return (
                    <button
                      key={node.id}
                      onClick={() => navigate(group.id, node.id)}
                      className={`flex items-center justify-between rounded-[7px] px-2.5 py-1.5 ${
                        isActive ? 'bg-surface2' : 'hover:bg-surface2/60'
                      }`}
                    >
                      <span
                        className={`text-[12.5px] ${isActive ? 'font-bold text-accent' : 'font-medium text-fg'}`}
                      >
                        {t(`subs.${node.id}`)}
                      </span>
                      <span className="text-[11px] text-muted">
                        {counts.subs[node.id] !== null ? formatCompact(counts.subs[node.id]!) : ''}
                      </span>
                    </button>
                  );
                })}
              </div>
            )}
          </div>
        );
      })}
    </aside>
  );
}
