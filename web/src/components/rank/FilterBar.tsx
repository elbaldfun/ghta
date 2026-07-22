'use client';

import { useTranslations } from 'next-intl';
import { useRouter } from '@/i18n/navigation';
import { useSearchParams } from 'next/navigation';
import { FILTER_LANGS, FILTER_LICENSES, SORT_OPTIONS } from '@/lib/rank-data';
import type { TypeFacet } from '@/lib/data';

const selectClass =
  'cursor-pointer appearance-none rounded-lg border border-border bg-surface py-1.5 pl-[11px] pr-[30px] text-xs font-semibold text-fg outline-none ' +
  "bg-[url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='10' height='10' viewBox='0 0 24 24' fill='none' stroke='%238a94a6' stroke-width='3'%3E%3Cpolyline points='6 9 12 15 18 9'/%3E%3C/svg%3E\")] bg-[length:10px] bg-[right_10px_center] bg-no-repeat";

function LabeledSelect({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (v: string) => void;
}) {
  return (
    <label className="flex items-center gap-1.5">
      <span className="whitespace-nowrap text-[11px] font-semibold text-muted">{label}</span>
      <select value={value} onChange={(e) => onChange(e.target.value)} className={selectClass}>
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </label>
  );
}

/** Form-type chips + fine-grained language / license / sort selects. */
export function FilterBar({ types = [] }: { types?: TypeFacet[] }) {
  const t = useTranslations('rank');
  const router = useRouter();
  const params = useSearchParams();
  const activeType = params.get('type');

  function setParam(key: string, value: string) {
    const next = new URLSearchParams(params.toString());
    next.delete('page');
    if (value === 'all') next.delete(key);
    else next.set(key, value);
    const qs = next.toString();
    router.push(qs ? `/?${qs}` : '/');
  }

  const all = { value: 'all', label: t('all') };
  const sortLabels: Record<string, string> = {
    stars: t('sortStars'),
    forks: t('sortForks'),
    updated: t('sortUpdated'),
  };

  return (
    <div className="flex flex-wrap items-center gap-3.5">
      {types.length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5">
          {types
            .filter((ty) => ty.count > 0)
            .map((ty) => {
              const on = activeType === ty.key;
              return (
                <button
                  key={ty.key}
                  onClick={() => setParam('type', on ? 'all' : ty.key)}
                  className={`rounded-full border px-2.5 py-1 text-[11px] font-semibold ${
                    on
                      ? 'border-accent bg-accent/10 text-accent'
                      : 'border-border text-muted hover:bg-surface2/60'
                  }`}
                >
                  {ty.name}
                </button>
              );
            })}
        </div>
      )}
      <LabeledSelect
        label={t('filterFine')}
        value={params.get('lang') ?? 'all'}
        options={[all, ...FILTER_LANGS.map((l) => ({ value: l, label: l }))]}
        onChange={(v) => setParam('lang', v)}
      />
      <LabeledSelect
        label={t('filterLicense')}
        value={params.get('license') ?? 'all'}
        options={[all, ...FILTER_LICENSES.map((l) => ({ value: l, label: l.toUpperCase() }))]}
        onChange={(v) => setParam('license', v)}
      />
      <LabeledSelect
        label={t('sortBy')}
        value={params.get('sort') ?? 'stars'}
        options={SORT_OPTIONS.map((s) => ({ value: s, label: sortLabels[s] }))}
        onChange={(v) => setParam('sort', v)}
      />
    </div>
  );
}
