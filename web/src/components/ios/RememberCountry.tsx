'use client';

import { useEffect } from 'react';

/**
 * Remembers the storefront being viewed, so /ios opens on it next time. A
 * cookie rather than storage: the redirect at /ios is decided on the server.
 */
export function RememberCountry({ name, country }: { name: string; country: string }) {
  useEffect(() => {
    try {
      document.cookie = `${name}=${country}; path=/; max-age=31536000; samesite=lax`;
    } catch {
      // Cookies blocked: /ios falls back to the interface language.
    }
  }, [name, country]);
  return null;
}
