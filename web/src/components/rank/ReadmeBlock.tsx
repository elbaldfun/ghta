import { FileIcon } from './icons';

const VIEWPORT_PX = 440;

/**
 * README panel: body height is capped and the content scrolls inside its own
 * viewport (overflow-y) instead of a click-to-expand toggle.
 * `html` is GitHub's own rendered + sanitized README HTML.
 */
export function ReadmeBlock({ html }: { html: string }) {
  return (
    <section className="mt-6 overflow-hidden rounded-card border border-border bg-surface">
      <div className="flex items-center gap-2 border-b border-border bg-surface2 py-[9px] px-4">
        <FileIcon size={15} className="text-muted" />
        <span className="text-xs font-bold tracking-wide">README</span>
      </div>
      <div
        className="readme-body overscroll-contain px-5 py-[18px]"
        style={{ maxHeight: VIEWPORT_PX, overflowY: 'auto' }}
        dangerouslySetInnerHTML={{ __html: html }}
      />
    </section>
  );
}
