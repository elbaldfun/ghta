'use client';

import { useEffect, useRef, useState } from 'react';
import { useTranslations } from 'next-intl';
import { useRouter } from '@/i18n/navigation';
import { useSearchParams } from 'next/navigation';
import { SearchIcon, StarIcon } from './icons';
import { formatCompact } from '@/lib/rank-data';

interface Suggestion {
  externalId: string;
  name: string;
  stars: number;
  language?: string;
  iconUrl?: string;
}

/**
 * Header search with an as-you-type dropdown. Typing hits a same-origin
 * autocomplete proxy (debounced); a result navigates straight to its detail
 * page, while Enter runs the full relevance-ranked search on the home page.
 */
export function SearchBox() {
  const t = useTranslations('rank');
  const router = useRouter();
  const params = useSearchParams();
  const current = params.get('q') ?? '';

  const [value, setValue] = useState(current);
  const [items, setItems] = useState<Suggestion[]>([]);
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(-1); // keyboard-highlighted row
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => setValue(current), [current]);

  // Debounced autocomplete fetch. Aborts the in-flight request on each keystroke
  // so out-of-order responses can't clobber a newer query.
  useEffect(() => {
    const q = value.trim();
    if (q.length < 2) {
      setItems([]);
      setOpen(false);
      return;
    }
    const ctrl = new AbortController();
    const timer = setTimeout(async () => {
      try {
        const res = await fetch(`/api/suggest?q=${encodeURIComponent(q)}`, { signal: ctrl.signal });
        const json = await res.json();
        setItems(Array.isArray(json.data) ? json.data : []);
        setOpen(true);
        setActive(-1);
      } catch {
        /* aborted or failed — keep the previous list */
      }
    }, 180);
    return () => {
      clearTimeout(timer);
      ctrl.abort();
    };
  }, [value]);

  useEffect(() => {
    function onDown(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener('mousedown', onDown);
    return () => document.removeEventListener('mousedown', onDown);
  }, []);

  function fullSearch() {
    const q = value.trim();
    setOpen(false);
    router.push(q ? `/?q=${encodeURIComponent(q)}` : '/');
  }

  function goto(s: Suggestion) {
    const [owner = '', name = ''] = s.externalId.split('/');
    setOpen(false);
    setValue('');
    router.push(`/repo/${owner}/${name}`);
  }

  function onKeyDown(e: React.KeyboardEvent) {
    if (!open || items.length === 0) {
      if (e.key === 'Enter') fullSearch();
      return;
    }
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActive((i) => (i + 1) % items.length);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActive((i) => (i <= 0 ? items.length - 1 : i - 1));
    } else if (e.key === 'Enter') {
      if (active >= 0 && active < items.length) goto(items[active]);
      else fullSearch();
    } else if (e.key === 'Escape') {
      setOpen(false);
    }
  }

  return (
    <div ref={ref} className="relative flex max-w-[460px] flex-1">
      <div className="flex w-full items-center gap-2.5 rounded-full border border-border bg-surface2 px-4 py-[9px]">
        <SearchIcon size={15} className="shrink-0 text-muted" />
        <input
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={onKeyDown}
          onFocus={() => items.length > 0 && setOpen(true)}
          placeholder={t('searchPlaceholder')}
          className="w-full bg-transparent text-[13px] text-fg outline-none placeholder:text-muted"
          aria-label={t('searchPlaceholder')}
          autoComplete="off"
        />
      </div>

      {open && items.length > 0 && (
        <div className="absolute left-0 right-0 top-full z-40 mt-1.5 overflow-hidden rounded-xl border border-border bg-surface shadow-card-hover">
          {items.map((s, i) => {
            const [owner = '', name = ''] = s.externalId.split('/');
            return (
              <button
                key={s.externalId}
                type="button"
                onMouseEnter={() => setActive(i)}
                onClick={() => goto(s)}
                className={`flex w-full items-center gap-2.5 px-3 py-2 text-left ${i === active ? 'bg-surface2' : ''}`}
              >
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  src={s.iconUrl || `https://github.com/${owner}.png?size=48`}
                  alt=""
                  loading="lazy"
                  width={22}
                  height={22}
                  className="h-[22px] w-[22px] shrink-0 rounded-[5px] border border-border bg-surface2 object-cover"
                />
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-[12.5px] font-semibold">
                    <span className="text-muted">{owner}/</span>
                    {name}
                  </span>
                </span>
                {s.language && <span className="shrink-0 text-[10.5px] text-muted">{s.language}</span>}
                <span className="flex shrink-0 items-center gap-0.5 font-mono text-[11px] tabular-nums text-muted">
                  <StarIcon size={11} className="text-accent2" />
                  {formatCompact(s.stars)}
                </span>
              </button>
            );
          })}
          <button
            type="button"
            onClick={fullSearch}
            className="block w-full border-t border-border px-3 py-2 text-left text-[11.5px] font-semibold text-accent hover:bg-surface2"
          >
            {t('searchSeeAll', { q: value.trim() })}
          </button>
        </div>
      )}
    </div>
  );
}
