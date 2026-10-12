/**
 * Markdown twin of a city hub (/canchas/<city>). Same data, builder and status answers as the
 * HTML page: 200 for a city with complexes or whose last complex left, 404 for a city that never
 * had any, 503 when the API fails.
 */
import type { APIRoute } from 'astro';
import { PUBLIC_API_URL } from 'astro:env/server';
import { ApiError, API_TIMEOUT_MS, createPublicApi } from '../lib/api.ts';
import { cityPath, toHubView } from '../lib/hub.ts';
import { hubMarkdown } from '../lib/markdown/hub.ts';
import { markdownError, markdownResponse } from '../lib/markdown/response.ts';
import { STOREFRONT_ORIGIN } from '../lib/storefront.ts';

export const prerender = false;

/** Longer city segments cannot name a city; they are answered as not found, as on the HTML page. */
const MAX_CITY_LENGTH = 100;
const CACHE_CONTROL = 'public, s-maxage=300, stale-while-revalidate=60';

const COPY = {
  not_found: {
    title: 'Canchas no encontradas - Vibe',
    lead: 'Todavía no hay complejos de Vibe en esta ciudad.',
  },
  unavailable: {
    title: 'Canchas no disponibles - Vibe',
    lead: 'Tuvimos un problema para cargar esta ciudad. Probá de nuevo en un minuto.',
  },
};

export const GET: APIRoute = async ({ params }) => {
  const city = params.city ?? '';
  const canonical = `${STOREFRONT_ORIGIN}${cityPath(city)}`;

  if (city.length === 0 || city.length > MAX_CITY_LENGTH) {
    return fail('not_found', canonical);
  }

  try {
    const api = createPublicApi({ baseUrl: PUBLIC_API_URL, timeoutMs: API_TIMEOUT_MS });
    const view = toHubView(await api.hubData(city));
    return markdownResponse({
      status: 200,
      body: hubMarkdown(view),
      canonical: view.canonical,
      cacheControl: CACHE_CONTROL,
    });
  } catch (error) {
    if (error instanceof ApiError && error.kind === 'not_found') return fail('not_found', canonical);
    console.error('city hub unavailable (markdown)', error);
    return fail('unavailable', canonical);
  }
};

function fail(kind: keyof typeof COPY, canonical: string): Response {
  const copy = COPY[kind];
  return markdownResponse({
    status: kind === 'not_found' ? 404 : 503,
    body: markdownError(copy.title, copy.lead, canonical),
    canonical,
    cacheControl: CACHE_CONTROL,
  });
}
