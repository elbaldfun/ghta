import type { Metadata } from 'next';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { fetchCategories, type CategoryTree } from '@/lib/api';
import { Link } from '@/i18n/navigation';
import { Card, EmptyState, ErrorState, PageHeader } from '@/components/ui';

export async function generateMetadata({
  params: { locale },
}: {
  params: { locale: string };
}): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'categories' });
  return { title: t('title'), description: t('description') };
}

function Tree({ nodes, locale }: { nodes: CategoryTree[]; locale: string }) {
  return (
    <ul className="space-y-1">
      {nodes.map((n) => (
        <li key={n.id}>
          <Link
            href={`/trending?category=${n.id}`}
            className="inline-block rounded px-2 py-0.5 text-fg hover:bg-surface hover:text-accent"
          >
            {n.name}
            <span className="ml-2 text-xs text-muted">{n.path}</span>
          </Link>
          {n.children && n.children.length > 0 && (
            <div className="ml-4 border-l border-border pl-3">
              <Tree nodes={n.children} locale={locale} />
            </div>
          )}
        </li>
      ))}
    </ul>
  );
}

export default async function CategoriesPage({ params: { locale } }: { params: { locale: string } }) {
  setRequestLocale(locale);
  const t = await getTranslations();
  const res = await fetchCategories();

  return (
    <div>
      <PageHeader title={t('categories.title')} description={t('categories.description')} />
      {res.error ? (
        <ErrorState message={t('common.error')} />
      ) : !res.data || res.data.length === 0 ? (
        <EmptyState message={t('common.empty')} />
      ) : (
        <Card className="p-5">
          <Tree nodes={res.data} locale={locale} />
        </Card>
      )}
    </div>
  );
}
