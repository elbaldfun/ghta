// Server-side client for the backend's /v1 interface. Every caller runs on the
// server; same-URL fetches are deduped by Next's fetch cache, and revalidate
// keeps pages fresh without a request per view.

// Server-only on purpose: without the NEXT_PUBLIC_ prefix the backend address
// is never inlined into the browser bundle.
export const API =
  process.env.API_URL ||
  // A production build without API_URL (e.g. a preview deploy that only has the
  // variable set for Production) would otherwise call localhost and render every
  // page empty. Fall back to the public API there; keep localhost for dev.
  (process.env.NODE_ENV === 'production' ? 'https://api.starrank.dev' : 'http://localhost:3000');

export type Fetched<T> = { data: T; error: null } | { data: null; error: string };

/** GET a /v1 path; revalidate is the page cache lifetime in seconds. */
export async function apiGet<T>(path: string, revalidate: number): Promise<Fetched<T>> {
  try {
    const res = await fetch(`${API}/v1${path}`, { next: { revalidate } });
    // A 404 is an answer (no such repo); anything else is a fault worth a log
    // line, since the page degrades to an empty state and hides it.
    if (!res.ok) {
      if (res.status !== 404) console.warn(`api: GET /v1${path}: HTTP ${res.status}`);
      return { data: null, error: `backend HTTP ${res.status}` };
    }
    return { data: (await res.json()) as T, error: null };
  } catch (e) {
    console.warn(`api: GET /v1${path}:`, e);
    return { data: null, error: e instanceof Error ? e.message : 'network error' };
  }
}
