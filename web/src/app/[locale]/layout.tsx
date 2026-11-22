import type { Metadata } from 'next';
import { NextIntlClientProvider } from 'next-intl';
import { getMessages, getTranslations, setRequestLocale } from 'next-intl/server';
import { notFound } from 'next/navigation';
import { Suspense } from 'react';
import { Sora, Manrope, IBM_Plex_Mono, Inter, Newsreader, JetBrains_Mono } from 'next/font/google';
import { routing } from '@/i18n/routing';
import { GoogleAnalytics } from '@/components/Analytics';
import '../globals.css';
import '@/styles/tokens.css';

const sora = Sora({ subsets: ['latin'], weight: ['400', '600', '700', '800'], variable: '--font-sora' });
const manrope = Manrope({ subsets: ['latin'], weight: ['400', '600', '700', '800'], variable: '--font-manrope' });
const plexMono = IBM_Plex_Mono({ subsets: ['latin'], weight: ['400', '600', '700'], variable: '--font-plex-mono' });
// Design tokens v1 faces (src/styles/tokens.css). Inter is preloaded because
// the site header uses it on every page; the other two render only on pages
// already moved to the new design, and browsers fetch a face only once text
// actually renders in it.
const inter = Inter({ subsets: ['latin'], variable: '--font-inter', display: 'swap' });
const newsreader = Newsreader({ subsets: ['latin'], axes: ['opsz'], variable: '--font-newsreader', preload: false, display: 'swap' });
const jbMono = JetBrains_Mono({ subsets: ['latin'], variable: '--font-jbmono', preload: false, display: 'swap' });

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
  return {
    metadataBase: new URL(SITE_URL),
    title: { default: `${t('name')} — ${t('tagline')}`, template: `%s · ${t('name')}` },
    description: t('tagline'),
    // No alternates here: each page declares its own canonical (lib/seo).
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
    <html
      lang={locale}
      suppressHydrationWarning
      className={`${sora.variable} ${manrope.variable} ${plexMono.variable} ${inter.variable} ${newsreader.variable} ${jbMono.variable}`}
    >
      <head>
        <script dangerouslySetInnerHTML={{ __html: noFlashTheme }} />
      </head>
      <body>
        <NextIntlClientProvider messages={messages}>
          {children}
        </NextIntlClientProvider>
        {/* useSearchParams needs a Suspense boundary to keep the route statically renderable. */}
        <Suspense>
          <GoogleAnalytics />
        </Suspense>
      </body>
    </html>
  );
}
