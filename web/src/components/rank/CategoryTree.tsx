'use client';

import { useState } from 'react';
import { useTranslations, useLocale } from 'next-intl';
import { useRouter } from '@/i18n/navigation';
import { useSearchParams } from 'next/navigation';
import { formatCompact } from '@/lib/rank-data';
import { categoryLabel, type CategoryNode } from '@/lib/data';
import { GridIcon } from './icons';
import { track } from '@/lib/analytics';

/**
 * Two-level domain tree (2a sidebar), rendered from the backend GET /category
 * tree. Selecting a node sets the `category` param (leaf path or parent segment),
 * resets to page 1; other filters (language/license/sort/type) are preserved.
 */
export function CategoryTree({ tree, total }: { tree: CategoryNode[]; total: number | null }) {
  const t = useTranslations('rank');
  const locale = useLocale();
  const router = useRouter();
  const params = useSearchParams();
  const active = params.get('category');
  const [expanded, setExpanded] = useState<Record<string, boolean>>(() => {
    // Expand the ancestor of the active node, else a sensible default.
    const parent = active?.includes('/') ? active.split('/')[0] : active;
    return parent ? { [parent]: true } : { ai: true, web: true };
  });

  function navigate(path: string | null) {
    track('category_select', { category: path ?? 'all' });
    const next = new URLSearchParams(params.toString());
    next.delete('page');
    next.delete('q');
    if (path) next.set('category', path);
    else next.delete('category');
    const qs = next.toString();
    router.push(qs ? `/?${qs}` : '/');
  }

  const count = (n: number) => (n > 0 ? <span className="text-[11px] text-muted">{formatCompact(n)}</span> : null);
  const allActive = !active;

  // Mobile: the full tree would push the ranking below the fold, so it
  // collapses into one native select (parents + indented leaves).
  const mobilePicker = (
    <div className="border-b border-border bg-surface px-4 py-3 lg:hidden">
      <select
        value={active ?? ''}
        onChange={(e) => navigate(e.target.value || null)}
        aria-label={t('browseAll')}
        className="w-full appearance-none rounded-lg border border-border bg-surface2 px-3 py-2 text-[13px] font-semibold text-fg outline-none"
      >
        <option value="">
          {t('browseAll')}
          {total ? ` (${formatCompact(total)})` : ''}
        </option>
        {tree.map((group) => (
          <optgroup key={group.path} label={categoryLabel(group, locale)}>
            <option value={group.path}>
              {categoryLabel(group, locale)}
              {group.count > 0 ? ` (${formatCompact(group.count)})` : ''}
            </option>
            {group.children?.map((node) => (
              <option key={node.path} value={node.path}>
                {'  '}
                {categoryLabel(node, locale)}
                {node.count > 0 ? ` (${formatCompact(node.count)})` : ''}
              </option>
            ))}
          </optgroup>
        ))}
      </select>
    </div>
  );

  return (
    <>
      {mobilePicker}
      <aside className="hidden border-r border-border bg-surface px-4 py-5 lg:block">
      <button
        onClick={() => navigate(null)}
        className={`mb-2 flex w-full items-center justify-between rounded-lg px-3 py-2 ${
          allActive ? 'bg-surface2' : 'hover:bg-surface2/60'
        }`}
      >
        <span
          className={`flex items-center gap-2 text-[13px] ${allActive ? 'font-bold text-accent' : 'font-semibold text-fg'}`}
        >
          <GridIcon size={15} />
          {t('browseAll')}
        </span>
        {total !== null && count(total)}
      </button>

      {tree.map((group) => {
        const isOpen = !!expanded[group.path];
        const isActiveCat = active === group.path;
        return (
          <div key={group.path} className="mt-0.5">
            <button
              onClick={() => {
                setExpanded((e) => ({ ...e, [group.path]: active === group.path ? !isOpen : true }));
                navigate(group.path);
              }}
              className={`flex w-full items-center justify-between gap-1.5 rounded-lg px-3 py-2 ${
                isActiveCat ? 'bg-surface2' : 'hover:bg-surface2/60'
              }`}
            >
              <span
                className={`flex items-center gap-[7px] text-[13px] ${
                  isActiveCat ? 'font-bold text-accent' : 'font-semibold text-fg'
                }`}
              >
                <span className="inline-block w-2.5 text-[9px] text-muted">{isOpen ? '▾' : '▸'}</span>
                {categoryLabel(group, locale)}
              </span>
              {count(group.count)}
            </button>
            {isOpen && group.children && (
              <div className="ml-4 mt-0.5 flex flex-col gap-0.5 border-l border-border pl-1.5">
                {group.children.map((node) => {
                  const isActive = active === node.path;
                  return (
                    <button
                      key={node.path}
                      onClick={() => navigate(node.path)}
                      className={`flex items-center justify-between rounded-[7px] px-2.5 py-1.5 ${
                        isActive ? 'bg-surface2' : 'hover:bg-surface2/60'
                      }`}
                    >
                      <span
                        className={`text-[12.5px] ${isActive ? 'font-bold text-accent' : 'font-medium text-fg'}`}
                      >
                        {categoryLabel(node, locale)}
                      </span>
                      {count(node.count)}
                    </button>
                  );
                })}
              </div>
            )}
          </div>
        );
      })}
      </aside>
    </>
  );
}
