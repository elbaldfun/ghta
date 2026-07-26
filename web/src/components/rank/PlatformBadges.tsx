import type { OS } from '@/lib/data';

const LABEL: Record<OS, string> = {
  macos: 'macOS',
  windows: 'Windows',
  linux: 'Linux',
  android: 'Android',
  ios: 'iOS',
  web: 'Web',
};

const ORDER: OS[] = ['macos', 'windows', 'linux', 'android', 'ios', 'web'];

/**
 * The platforms an app ships for, as compact chips. Asset-verified platforms are
 * solid; topic/heuristic-inferred ones are outlined and dimmed, so a guess never
 * looks like a confirmed download target.
 */
export function PlatformBadges({
  platforms,
  inferred = false,
  inferredLabel,
}: {
  platforms: OS[];
  inferred?: boolean;
  inferredLabel?: string;
}) {
  if (!platforms || platforms.length === 0) return null;
  const sorted = ORDER.filter((p) => platforms.includes(p));
  return (
    <span className="inline-flex flex-wrap items-center gap-1">
      {sorted.map((p) =>
        inferred ? (
          <span
            key={p}
            title={inferredLabel}
            className="rounded-[3px] border border-dashed border-border px-1.5 py-px font-mono text-[9.5px] font-semibold text-muted"
          >
            {LABEL[p]}
          </span>
        ) : (
          <span
            key={p}
            className="rounded-[3px] border border-accent/40 bg-accent/10 px-1.5 py-px font-mono text-[9.5px] font-semibold text-accent"
          >
            {LABEL[p]}
          </span>
        ),
      )}
    </span>
  );
}
