// Thin wrapper over the GA4 gtag queue for product conversion events. Safe to
// call from any client event handler: a no-op when gtag isn't loaded (local
// dev / preview with no NEXT_PUBLIC_GA_ID, or an ad-blocked visitor).

/* eslint-disable @typescript-eslint/no-explicit-any */
declare global {
  interface Window {
    dataLayer?: any[];
    gtag?: (...args: any[]) => void;
  }
}

/** Send one GA4 event. Fire-and-forget; never throws in the caller's path. */
export function track(event: string, params?: Record<string, unknown>): void {
  if (typeof window === 'undefined') return;
  try {
    window.gtag?.('event', event, params ?? {});
  } catch {
    /* analytics must never break a user interaction */
  }
}
