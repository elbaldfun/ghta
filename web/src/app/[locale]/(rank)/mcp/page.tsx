import type { Metadata } from 'next';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { PageShell } from '@/components/rank/PageShell';
import { CopyCommand } from '@/components/rank/CopyCommand';
import { alternatesFor } from '@/lib/seo';

/** The public MCP endpoint, served by the api (internal/api/mcp.go). */
const MCP_URL = 'https://api.starrank.dev/mcp';
const CLAUDE_CODE = `claude mcp add --transport http starrank ${MCP_URL}`;
const CLIENT_CONFIG = JSON.stringify({ mcpServers: { starrank: { url: MCP_URL } } }, null, 2);

// Tool names render in this order; each has a line in messages.
const TOOLS = ['search_repos', 'top_repos', 'trending_today', 'get_repo', 'list_domains'] as const;

export async function generateMetadata({ params: { locale } }: { params: { locale: string } }): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'mcp' });
  return { alternates: alternatesFor(locale, '/mcp'), title: t('title'), description: t('metaDescription') };
}

export default async function McpPage({ params: { locale } }: { params: { locale: string } }) {
  setRequestLocale(locale);
  const t = await getTranslations('mcp');

  return (
    <PageShell className="py-[22px]">
      <article className="max-w-[760px]">
        <h1 className="font-display text-[23px] font-extrabold">{t('title')}</h1>
        <p className="mt-3 text-[13px] leading-relaxed text-muted">{t('intro')}</p>

        <section className="mt-6">
          <h2 className="font-display text-base font-bold">{t('installH')}</h2>
          <h3 className="mt-3 text-[13px] font-semibold">{t('claudeCode')}</h3>
          <div className="mt-2 grid rounded-md border border-border px-3 py-2">
            <CopyCommand command={CLAUDE_CODE} repo="starrank/mcp" />
          </div>
          <p className="mt-4 text-[13px] leading-relaxed text-muted">{t('otherClients')}</p>
          <div className="mt-2 grid rounded-md border border-border px-3 py-2">
            <CopyCommand command={MCP_URL} repo="starrank/mcp" />
          </div>
          <pre className="mt-2 overflow-x-auto rounded-md border border-border px-3 py-2 font-mono text-xs text-muted">
            {CLIENT_CONFIG}
          </pre>
        </section>

        <section className="mt-6">
          <h2 className="font-display text-base font-bold">{t('toolsH')}</h2>
          <ul className="mt-2 space-y-1.5 text-[13px] leading-relaxed text-muted">
            {TOOLS.map((name) => (
              <li key={name}>
                <code className="font-mono text-xs text-fg">{name}</code> — {t(`tools.${name}`)}
              </li>
            ))}
          </ul>
        </section>

        <section className="mt-6">
          <h2 className="font-display text-base font-bold">{t('examplesH')}</h2>
          <ul className="mt-2 list-disc space-y-1 pl-5 text-[13px] leading-relaxed text-muted">
            {t('examples')
              .split('\n')
              .map((q) => (
                <li key={q}>{q}</li>
              ))}
          </ul>
          <p className="mt-4 text-xs leading-relaxed text-muted">{t('limits')}</p>
        </section>
      </article>
    </PageShell>
  );
}
