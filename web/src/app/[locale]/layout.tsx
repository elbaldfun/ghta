import type { Metadata } from 'next';
import { NextIntlClientProvider } from 'next-intl';
import { getMessages, getTranslations, setRequestLocale } from 'next-intl/server';
import { notFound } from 'next/navigation';
import { routing } from '@/i18n/routing';
import { Nav } from '@/components/Nav';
import '../globals.css';

const SITE_URL = process.env.NEXT_PUBLIC_SITE_URL || 'http://localhost:3001';

export function generateStaticParams() {
  return routing.locales.map((locale) => ({ locale }));
}

export async function generateMetadata({
  params: { locale },
}: {
  params: { locale: string };
}): Promise<Metadata> {
  const t = await getTranslations({ locale, namespace: 'site' });
  const languages = Object.fromEntries(
    routing.locales.map((l) => [l, `${SITE_URL}/${l}`]),
  );
  return {
    metadataBase: new URL(SITE_URL),
    title: { default: `${t('name')} — ${t('tagline')}`, template: `%s · ${t('name')}` },
    description: t('tagline'),
    alternates: { canonical: `/${locale}`, languages: { ...languages, 'x-default': SITE_URL } },
    openGraph: { title: t('name'), description: t('tagline'), type: 'website', locale },
    twitter: { card: 'summary_large_image', title: t('name'), description: t('tagline') },
  };
}

// Set the theme before paint to avoid a flash of the wrong theme.
const noFlashTheme = `(function(){try{var t=localStorage.getItem('theme');if(t)document.documentElement.setAttribute('data-theme',t);}catch(e){}})();`;

export default async function LocaleLayout({
  children,
  params: { locale },
}: {
  children: React.ReactNode;
  params: { locale: string };
}) {
  if (!routing.locales.includes(locale as (typeof routing.locales)[number])) notFound();
  setRequestLocale(locale);
  const messages = await getMessages();

  return (
    <html lang={locale} suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: noFlashTheme }} />
      </head>
      <body>
        <NextIntlClientProvider messages={messages}>
          <Nav />
          <main className="mx-auto max-w-5xl px-4 py-8">{children}</main>
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
