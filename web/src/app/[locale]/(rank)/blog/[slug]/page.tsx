import type { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { getTranslations, setRequestLocale } from 'next-intl/server';
import { Link } from '@/i18n/navigation';
import { getPost, renderPostBody } from '@/lib/blog';
import { DataBlock } from '@/components/blog/DataBlock';
import { BackIcon } from '@/components/rank/icons';

const SITE_URL = process.env.NEXT_PUBLIC_SITE_URL || 'http://localhost:3001';

interface Params {
  locale: string;
  slug: string;
}

export async function generateMetadata({ params }: { params: Params }): Promise<Metadata> {
  const post = await getPost(params.locale, params.slug);
  if (!post) return {};
  const url = `${SITE_URL}/${params.locale}/blog/${post.slug}`;
  return {
    title: post.title,
    description: post.description,
    alternates: { canonical: url },
    openGraph: {
      type: 'article',
      title: post.title,
      description: post.description,
      url,
      publishedTime: post.date,
      tags: post.tags,
    },
    twitter: { card: 'summary_large_image', title: post.title, description: post.description },
  };
}

export default async function BlogPost({ params }: { params: Params }) {
  setRequestLocale(params.locale);
  const t = await getTranslations('blog');

  const post = await getPost(params.locale, params.slug);
  if (!post) notFound();
  const segments = await renderPostBody(post.body);

  // Article structured data: helps the post surface as an article in search
  // rather than as an untyped page.
  const jsonLd = {
    '@context': 'https://schema.org',
    '@type': 'Article',
    headline: post.title,
    description: post.description,
    datePublished: post.date,
    keywords: post.tags.join(', '),
    inLanguage: params.locale,
    mainEntityOfPage: `${SITE_URL}/${params.locale}/blog/${post.slug}`,
    publisher: { '@type': 'Organization', name: 'StarRank Explorer' },
  };

  return (
    <div className="px-7 py-[22px]">
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
      />

      <Link
        href="/blog"
        className="mb-4 flex w-fit items-center gap-1.5 text-xs font-semibold text-muted hover:text-fg"
      >
        <BackIcon size={14} />
        {t('backToList')}
      </Link>

      <article className="max-w-[760px]">
        <h1 className="font-display text-[26px] font-extrabold leading-tight">{post.title}</h1>
        <div className="mt-2 flex flex-wrap items-center gap-x-2 gap-y-1 text-[11px] text-muted">
          <time dateTime={post.date}>{post.date}</time>
          <span aria-hidden>·</span>
          <span>{t('readingTime', { minutes: post.readingMinutes })}</span>
          {post.tags.map((tag) => (
            <span key={tag} className="rounded-full bg-surface2 px-2.5 py-0.5 font-semibold">
              {tag}
            </span>
          ))}
        </div>

        <div className="mt-6">
          {segments.map((seg, i) =>
            seg.kind === 'html' ? (
              <div key={i} className="readme-body" dangerouslySetInnerHTML={{ __html: seg.html }} />
            ) : (
              <DataBlock key={i} name={seg.name} params={seg.params} />
            ),
          )}
        </div>

        <p className="mt-8 border-t border-border pt-4 text-[11px] text-muted">{t('dataNote')}</p>
      </article>
    </div>
  );
}
