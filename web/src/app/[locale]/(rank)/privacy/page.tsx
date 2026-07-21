import type { Metadata } from 'next';
import { getTranslations, setRequestLocale } from 'next-intl/server';

export async function generateMetadata({
  params: { locale },
}: {
  params: { locale: string };
}): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'privacy' });
  // Legal boilerplate has no search value and would dilute the site's index.
  return { title: t('title'), robots: { index: false, follow: true } };
}

// Section keys render in this order; each has a heading and a body in messages.
const SECTIONS = ['collect', 'cookies', 'thirdParty', 'ads', 'choices', 'contact'] as const;

export default async function PrivacyPage({
  params: { locale },
}: {
  params: { locale: string };
}) {
  setRequestLocale(locale);
  const t = await getTranslations('privacy');

  return (
    <div className="px-7 py-[22px]">
      <article className="max-w-[760px]">
        <h1 className="font-display text-[23px] font-extrabold">{t('title')}</h1>
        <p className="mt-1 text-xs text-muted">{t('updated')}</p>
        <p className="mt-5 text-[13px] leading-relaxed text-muted">{t('intro')}</p>

        {SECTIONS.map((key) => (
          <section key={key} className="mt-6">
            <h2 className="font-display text-base font-bold">{t(`sections.${key}.h`)}</h2>
            <p className="mt-2 whitespace-pre-line text-[13px] leading-relaxed text-muted">
              {t(`sections.${key}.p`)}
            </p>
          </section>
        ))}
      </article>
    </div>
  );
}
