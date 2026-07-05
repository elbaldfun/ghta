import type { ReactNode } from 'react';
import type { Source, TrackedItem } from '@/lib/api';
import { formatNumber, signedNumber, sourceColor, sourceLabels, primaryValue } from '@/lib/format';
import { Link } from '@/i18n/navigation';

export function Card({ children, className = '' }: { children: ReactNode; className?: string }) {
  return (
    <div className={`rounded-card border border-border bg-surface ${className}`}>{children}</div>
  );
}

export function SourceBadge({ source }: { source: Source }) {
  return (
    <span
      className="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium"
      style={{ backgroundColor: `color-mix(in srgb, ${sourceColor(source)} 16%, transparent)`, color: sourceColor(source) }}
    >
      <span aria-hidden className="h-1.5 w-1.5 rounded-full" style={{ backgroundColor: sourceColor(source) }} />
      {sourceLabels[source]}
    </span>
  );
}

/** Growth badge: shape + sign so it never relies on color alone (a11y). */
export function GrowthBadge({ value }: { value: number | null }) {
  if (value === null) return <span className="text-xs text-muted">—</span>;
  const up = value >= 0;
  return (
    <span
      className="inline-flex items-center gap-0.5 text-xs font-semibold"
      style={{ color: up ? 'rgb(var(--up))' : 'rgb(var(--down))' }}
    >
      <span aria-hidden>{up ? '▲' : '▼'}</span>
      {signedNumber(value)}
    </span>
  );
}

export function ItemCard({
  item,
  href,
  rank,
  growth,
}: {
  item: TrackedItem;
  href: string;
  rank?: number;
  growth?: number | null;
}) {
  return (
    <Card className="flex items-start gap-3 p-4 transition-colors hover:border-accent">
      {rank !== undefined && (
        <div className="w-6 shrink-0 pt-0.5 text-sm font-semibold text-muted">{rank}</div>
      )}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <SourceBadge source={item.source} />
          {item.language && <span className="text-xs text-muted">{item.language}</span>}
        </div>
        <Link href={href} className="mt-1 block truncate font-semibold text-fg hover:text-accent">
          {item.externalId}
        </Link>
        {item.description && (
          <p className="mt-1 line-clamp-2 text-sm text-muted">{item.description}</p>
        )}
      </div>
      <div className="shrink-0 text-right">
        <div className="font-semibold tabular-nums">{formatNumber(primaryValue(item))}</div>
        {growth !== undefined && <div className="mt-1"><GrowthBadge value={growth} /></div>}
      </div>
    </Card>
  );
}

export function EmptyState({ message }: { message: string }) {
  return (
    <Card className="p-10 text-center text-muted">
      <p>{message}</p>
    </Card>
  );
}

export function ErrorState({ message }: { message: string }) {
  return (
    <Card className="p-10 text-center">
      <p className="text-down">{message}</p>
    </Card>
  );
}

export function PageHeader({ title, description }: { title: string; description?: string }) {
  return (
    <header className="mb-6">
      <h1 className="text-2xl font-bold tracking-tight">{title}</h1>
      {description && <p className="mt-1 text-muted">{description}</p>}
    </header>
  );
}
