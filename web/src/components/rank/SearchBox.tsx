'use client';

import { useEffect, useRef, useState } from 'react';
import { useTranslations } from 'next-intl';
import { useRouter } from '@/i18n/navigation';
import { useSearchParams } from 'next/navigation';
import { SearchIcon, StarIcon } from './icons';
import { formatCompact } from '@/lib/rank-data';
import { track } from '@/lib/analytics';

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
    if (q) track('search', { search_term: q });
    setOpen(false);
    router.push(q ? `/github?q=${encodeURIComponent(q)}` : '/github');
  }

  function goto(s: Suggestion) {
    const [owner = '', name = ''] = s.externalId.split('/');
    track('search_result_click', { repo: s.externalId, search_term: value.trim() });
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
    <div ref={ref} className="relative flex w-full flex-1">
      <div className="flex h-11 w-full items-center gap-2.5 rounded border border-rule bg-sunk px-3 focus-within:border-ink-3 md:h-9">
        <SearchIcon size={15} className="shrink-0 text-ink-3" />
        <input
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={onKeyDown}
          onFocus={() => items.length > 0 && setOpen(true)}
          placeholder={t('searchPlaceholder')}
          className="w-full bg-transparent text-sr-small text-ink outline-none placeholder:text-ink-3"
          aria-label={t('searchPlaceholder')}
          autoComplete="off"
        />
      </div>

      {open && items.length > 0 && (
        <div className="absolute left-0 right-0 top-full z-40 mt-1.5 overflow-hidden rounded-lg border border-rule bg-sr-surface shadow-sr-pop">
          {items.map((s, i) => {
            const [owner = '', name = ''] = s.externalId.split('/');
            return (
              <button
                key={s.externalId}
                type="button"
                onMouseEnter={() => setActive(i)}
                onClick={() => goto(s)}
                className={`flex min-h-11 w-full items-center gap-2.5 px-3 py-2 text-left ${i === active ? 'bg-sunk' : ''}`}
              >
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  src={s.iconUrl || `https://github.com/${owner}.png?size=48`}
                  alt=""
                  loading="lazy"
                  width={22}
                  height={22}
                  className="h-[22px] w-[22px] shrink-0 rounded border border-rule bg-sunk object-cover"
                />
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-sr-small font-semibold text-ink">
                    <span className="font-normal text-ink-3">{owner}/</span>
                    {name}
                  </span>
                </span>
                {s.language && <span className="shrink-0 text-sr-caption text-ink-3">{s.language}</span>}
                <span className="flex shrink-0 items-center gap-0.5 text-sr-caption tabular-nums text-ink-3">
                  <StarIcon size={11} />
                  {formatCompact(s.stars)}
                </span>
              </button>
            );
          })}
          <button
            type="button"
            onClick={fullSearch}
            className="block min-h-11 w-full border-t border-rule px-3 py-2 text-left text-sr-small font-semibold text-ink hover:bg-sunk"
          >
            {t('searchSeeAll', { q: value.trim() })}
          </button>
        </div>
      )}
    </div>
  );
}
