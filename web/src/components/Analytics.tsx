'use client';

import Script from 'next/script';
import { useEffect, useRef } from 'react';
import { usePathname, useSearchParams } from 'next/navigation';

const GA_ID = process.env.NEXT_PUBLIC_GA_ID;

/**
 * Google Analytics 4 with Consent Mode v2.
 *
 * Renders nothing unless NEXT_PUBLIC_GA_ID is set, so local dev and previews
 * stay untracked. Consent defaults to denied for every storage type — the
 * cookie banner grants it — which is the shape Google requires for EEA
 * visitors once AdSense is enabled.
 */
export function GoogleAnalytics() {
  const pathname = usePathname();
  const searchParams = useSearchParams();
  // gtag's own config sends the first page_view. Sending it here too would
  // double-count it, and the effect can also run before gtag.js has loaded.
  const skippedInitial = useRef(false);

  useEffect(() => {
    if (!GA_ID) return;
    if (!skippedInitial.current) {
      skippedInitial.current = true;
      return;
    }
    // App Router navigations are client-side, so each route change needs its
    // own event. Queue through dataLayer directly in case gtag.js is still
    // loading — it replays the queue once ready.
    window.dataLayer = window.dataLayer || [];
    if (typeof window.gtag !== 'function') {
      window.gtag = function gtag() {
        // eslint-disable-next-line prefer-rest-params
        window.dataLayer!.push(arguments);
      };
    }
    const qs = searchParams.toString();
    window.gtag('event', 'page_view', {
      page_path: qs ? `${pathname}?${qs}` : pathname,
      page_location: window.location.href,
      page_title: document.title,
    });
  }, [pathname, searchParams]);

  if (!GA_ID) return null;

  return (
    <>
      <Script src={`https://www.googletagmanager.com/gtag/js?id=${GA_ID}`} strategy="afterInteractive" />
      <Script id="ga-init" strategy="afterInteractive">
        {`
          window.dataLayer = window.dataLayer || [];
          function gtag(){dataLayer.push(arguments);}
          window.gtag = gtag;
          gtag('consent', 'default', {
            analytics_storage: 'denied',
            ad_storage: 'denied',
            ad_user_data: 'denied',
            ad_personalization: 'denied',
            wait_for_update: 500
          });
          try {
            if (localStorage.getItem('cookie-consent') === 'granted') {
              gtag('consent', 'update', {
                analytics_storage: 'granted',
                ad_storage: 'granted',
                ad_user_data: 'granted',
                ad_personalization: 'granted'
              });
            }
          } catch (e) {}
          gtag('js', new Date());
          gtag('config', '${GA_ID}');
        `}
      </Script>
    </>
  );
}
