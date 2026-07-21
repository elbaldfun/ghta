import { getLanguageStats, searchRepos } from '@/lib/data';
import { formatCompact, langColor } from '@/lib/rank-data';
import type { DataBlockName } from '@/lib/blog';
import { Link } from '@/i18n/navigation';

/**
 * Renders a post's live-data block. Every number here is read from our own
 * database at request time, so an article never goes stale the way a pasted
 * table would.
 */
export async function DataBlock({
  name,
  params,
}: {
  name: DataBlockName;
  params: Record<string, string>;
}) {
  if (name === 'languages') return <LanguageTable limit={num(params.limit, 10, 25)} />;
  if (name === 'top-repos') {
    return <TopRepos language={params.language} limit={num(params.limit, 5, 20)} />;
  }
  return null;
}

function num(raw: string | undefined, fallback: number, max: number): number {
  const n = Number(raw);
  return Number.isFinite(n) && n > 0 ? Math.min(n, max) : fallback;
}

async function LanguageTable({ limit }: { limit: number }) {
  const stats = await getLanguageStats(limit);
  if (stats.length === 0) return null;

  // Bars are scaled against the largest value so the comparison is visual, not
  // just numeric — the point of the table is relative size.
  const maxRepos = Math.max(...stats.map((s) => s.repos));

  return (
    <figure className="my-6 overflow-x-auto rounded-card border border-border">
      <table className="w-full min-w-[560px] border-collapse text-[13px]">
        <thead>
          <tr className="bg-surface2 text-left text-[11px] uppercase tracking-wide text-muted">
            <th className="px-4 py-2.5 font-semibold">Language</th>
            <th className="px-4 py-2.5 text-right font-semibold">Repos</th>
            <th className="px-4 py-2.5 text-right font-semibold">Total stars</th>
            <th className="px-4 py-2.5 text-right font-semibold">Median</th>
            <th className="px-4 py-2.5 font-semibold">Top repo</th>
          </tr>
        </thead>
        <tbody>
          {stats.map((s) => (
            <tr key={s.language} className="border-t border-border">
              <td className="px-4 py-2.5">
                <span className="flex items-center gap-2 font-semibold">
                  <span
                    className="inline-block h-2.5 w-2.5 shrink-0 rounded-full"
                    style={{ backgroundColor: langColor(s.language) }}
                  />
                  {s.language}
                </span>
              </td>
              <td className="px-4 py-2.5 text-right tabular-nums">
                <span className="flex items-center justify-end gap-2">
                  <span
                    aria-hidden
                    className="h-1.5 rounded-full bg-accent/30"
                    style={{ width: `${Math.round((s.repos / maxRepos) * 56)}px` }}
                  />
                  {s.repos.toLocaleString()}
                </span>
              </td>
              <td className="px-4 py-2.5 text-right tabular-nums text-muted">
                {formatCompact(s.totalStars)}
              </td>
              {/* Exact, not compacted: the medians cluster within a few hundred
                  of each other, and rounding them to "2k" erases the comparison. */}
              <td className="px-4 py-2.5 text-right font-semibold tabular-nums">
                {Math.round(s.medianStars).toLocaleString()}
              </td>
              <td className="px-4 py-2.5">
                <Link
                  href={`/repo/${s.topRepo}`}
                  className="truncate font-medium text-accent hover:underline"
                >
                  {s.topRepo}
                </Link>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </figure>
  );
}

async function TopRepos({ language, limit }: { language?: string; limit: number }) {
  const res = await searchRepos({ language, perPage: limit, sort: 'stars' });
  if (res.error !== null || res.data.items.length === 0) return null;

  return (
    <figure className="my-6 overflow-hidden rounded-card border border-border">
      <ol className="divide-y divide-border">
        {res.data.items.map((r, i) => (
          <li key={r.fullName} className="flex items-center gap-3 px-4 py-2.5 text-[13px]">
            <span className="w-5 shrink-0 text-right tabular-nums text-muted">{i + 1}</span>
            <span
              className="inline-block h-2.5 w-2.5 shrink-0 rounded-full"
              style={{ backgroundColor: langColor(r.language) }}
            />
            <Link
              href={`/repo/${r.owner}/${r.name}`}
              className="min-w-0 flex-1 truncate font-semibold hover:text-accent"
            >
              {r.fullName}
            </Link>
            <span className="hidden min-w-0 flex-[2] truncate text-muted sm:block">
              {r.description}
            </span>
            <span className="shrink-0 tabular-nums font-semibold">{formatCompact(r.stars)}</span>
          </li>
        ))}
      </ol>
    </figure>
  );
}
