import { useTranslations } from 'next-intl';
import type { ModelItem } from '@/lib/data';
import { formatCompact } from '@/lib/rank-data';

function heatColor(growth: number): string {
  if (growth >= 50) return 'rgb(var(--heat4))';
  if (growth >= 20) return 'rgb(var(--heat3))';
  if (growth >= 8) return 'rgb(var(--heat2))';
  if (growth >= 2) return 'rgb(var(--heat1))';
  return 'rgb(var(--heat0))';
}

/**
 * One dense row of the HuggingFace model board: rank, model identity, task /
 * packaging chips, likes velocity as the heat figure, and 30-day downloads as
 * the adoption figure. Links out to the model page on HF — the model card is
 * HF's home turf; ours is the ranking and velocity.
 */
export function ModelRow({
  model,
  rank,
  taskLabel,
}: {
  model: ModelItem;
  rank: number;
  taskLabel?: string;
}) {
  const t = useTranslations('rank');
  const color = heatColor(model.growth);

  return (
    <a
      href={model.url}
      target="_blank"
      rel="noopener noreferrer"
      className="grid grid-cols-[36px_1fr_auto] items-center gap-2.5 border-b border-border px-3 py-2.5 transition-colors last:border-b-0 hover:bg-surface2 sm:grid-cols-[40px_1fr_110px_96px_80px] sm:gap-3.5 sm:px-4"
    >
      <span
        className="font-mono text-[15px] font-bold tabular-nums tracking-tight"
        style={{ color: rank <= 3 ? color : undefined }}
      >
        {rank}
      </span>

      <span className="min-w-0">
        <span className="flex items-center gap-2">
          <span className="truncate text-[13.5px] font-semibold">
            <span className="text-muted">{model.author ? `${model.author}/` : ''}</span>
            {model.name}
          </span>
          {model.gated && (
            <span className="shrink-0 rounded-[3px] border border-border bg-surface2 px-1 py-px font-mono text-[9px] font-semibold text-muted">
              🔒 {t('modelGated')}
            </span>
          )}
        </span>
        <span className="mt-1 flex flex-wrap gap-1">
          {taskLabel && (
            <span className="rounded-[3px] border border-accent bg-surface2 px-1.5 py-px font-mono text-[9.5px] text-accent">
              {taskLabel}
            </span>
          )}
          {model.library && (
            <span className="rounded-[3px] border border-border bg-surface2 px-1.5 py-px font-mono text-[9.5px] text-muted">
              {model.library}
            </span>
          )}
          {(model.quantFormats ?? []).slice(0, 3).map((q) => (
            <span key={q} className="rounded-[3px] bg-surface2 px-1.5 py-px font-mono text-[9.5px] text-muted">
              {q}
            </span>
          ))}
          {model.license && (
            <span className="hidden rounded-[3px] bg-surface2 px-1.5 py-px font-mono text-[9.5px] text-muted sm:inline">
              {model.license}
            </span>
          )}
        </span>
      </span>

      {/* velocity: daily likes gain */}
      <span className="text-right">
        {model.growth > 0 ? (
          <>
            <span className="block font-mono text-[15px] font-bold leading-none tabular-nums tracking-tight" style={{ color }}>
              +{formatCompact(model.growth)}
            </span>
            <span className="mt-0.5 block font-mono text-[9px] uppercase tracking-[0.06em] text-muted">
              ♥/{t('devPerDay')}
            </span>
          </>
        ) : (
          <span className="font-mono text-[12px] text-muted">—</span>
        )}
      </span>

      <span className="hidden text-right font-mono text-[12.5px] tabular-nums text-fg sm:block">
        {formatCompact(model.downloads30d)}
        <span className="block font-mono text-[9px] uppercase tracking-[0.06em] text-muted">{t('modelDl30d')}</span>
      </span>

      <span className="hidden text-right font-mono text-[12.5px] tabular-nums text-muted sm:block">
        ♥ {formatCompact(model.likes)}
      </span>
    </a>
  );
}
