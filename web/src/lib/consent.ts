// Cookie-consent state shared by the banner and the GA loader.
//
// Google Consent Mode v2: gtag loads immediately but with every storage type
// denied, so nothing is written until the visitor opts in. This is the shape
// Google requires for EEA traffic once AdSense is enabled, so wiring it now
// avoids redoing the integration later.

export const CONSENT_KEY = 'cookie-consent';
export type ConsentChoice = 'granted' | 'denied';

/* eslint-disable @typescript-eslint/no-explicit-any */
declare global {
  interface Window {
    dataLayer?: any[];
    gtag?: (...args: any[]) => void;
  }
}

export function readConsent(): ConsentChoice | null {
  if (typeof window === 'undefined') return null;
  try {
    const v = window.localStorage.getItem(CONSENT_KEY);
    return v === 'granted' || v === 'denied' ? v : null;
  } catch {
    // Storage can throw in private mode; treat it as "no choice recorded".
    return null;
  }
}

export function writeConsent(choice: ConsentChoice): void {
  try {
    window.localStorage.setItem(CONSENT_KEY, choice);
  } catch {
    // Non-fatal: consent just won't persist across visits.
  }
}

/** Push a Consent Mode update for the analytics + advertising storage types. */
export function applyConsent(choice: ConsentChoice): void {
  window.gtag?.('consent', 'update', {
    analytics_storage: choice,
    ad_storage: choice,
    ad_user_data: choice,
    ad_personalization: choice,
  });
}
