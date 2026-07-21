import type { Metadata } from 'next';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import { listPosts } from '@/lib/blog';

export async function generateMetadata({
  params: { locale },
}: {
  params: { locale: string };
}): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'blog' });
  return { title: t('title'), description: t('subtitle') };
}

export default async function BlogIndex({
  params: { locale },
}: {
  params: { locale: string };
}) {
  setRequestLocale(locale);
  const t = await getTranslations('blog');
  const posts = await listPosts(locale);

  return (
    <div className="px-7 py-[22px]">
      <header className="mb-6 max-w-[760px]">
        <h1 className="font-display text-[23px] font-extrabold">{t('title')}</h1>
        <p className="mt-1.5 text-[13px] text-muted">{t('subtitle')}</p>
      </header>

      {posts.length === 0 ? (
        <p className="py-10 text-[13px] text-muted">{t('empty')}</p>
      ) : (
        <ul className="grid gap-3.5 sm:grid-cols-2 lg:grid-cols-3">
          {posts.map((p) => (
            <li key={p.slug}>
              <Link
                href={`/blog/${p.slug}`}
                className="flex h-full flex-col gap-2 rounded-card border border-border bg-surface p-4 transition-[box-shadow,border-color] hover:border-accent hover:shadow-card-hover"
              >
                <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-[11px] text-muted">
                  <time dateTime={p.date}>{p.date}</time>
                  <span aria-hidden>·</span>
                  <span>{t('readingTime', { minutes: p.readingMinutes })}</span>
                </div>
                <h2 className="font-display text-[15px] font-bold leading-snug">{p.title}</h2>
                <p className="line-clamp-3 text-[12.5px] leading-relaxed text-muted">
                  {p.description}
                </p>
                {p.tags.length > 0 && (
                  <div className="mt-auto flex flex-wrap gap-1.5 pt-1">
                    {p.tags.map((tag) => (
                      <span
                        key={tag}
                        className="rounded-full bg-surface2 px-2.5 py-0.5 text-[10px] font-semibold text-muted"
                      >
                        {tag}
                      </span>
                    ))}
                  </div>
                )}
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
