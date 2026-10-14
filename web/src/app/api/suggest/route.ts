import { NextResponse, type NextRequest } from 'next/server';
import { API } from '@/lib/api/client';

// Same-origin proxy for search autocomplete: the browser hits /api/suggest and
// this handler calls the backend server-side, so we don't need CORS on the API
// (the rest of the site also calls it only from the server).

export const dynamic = 'force-dynamic';

export async function GET(req: NextRequest) {
  const q = (req.nextUrl.searchParams.get('q') ?? '').trim();
  if (q.length < 2) return NextResponse.json({ data: [] });
  try {
    const res = await fetch(`${API}/v1/search/suggest?q=${encodeURIComponent(q)}`, { cache: 'no-store' });
    if (!res.ok) return NextResponse.json({ data: [] });
    const body = (await res.json()) as { data?: { fullName: string }[] };
    // The search box keys suggestions by externalId.
    return NextResponse.json({ data: (body.data ?? []).map((s) => ({ ...s, externalId: s.fullName })) });
  } catch {
    return NextResponse.json({ data: [] });
  }
}
