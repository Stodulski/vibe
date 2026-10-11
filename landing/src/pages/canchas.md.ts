/**
 * Markdown twin of the city index (/canchas). Same data and builder as the HTML page, and the
 * same 503 when the API cannot answer. Reached by content negotiation through the Vercel
 * middleware, or directly at /canchas.md.
 */
import type { APIRoute } from 'astro';
import { PUBLIC_API_URL } from 'astro:env/server';
import { API_TIMEOUT_MS, createPublicApi } from '../lib/api.ts';
import { toHubIndexView } from '../lib/hub-index.ts';
import { hubIndexMarkdown } from '../lib/markdown/hub-index.ts';
import { markdownError, markdownResponse } from '../lib/markdown/response.ts';
import { STOREFRONT_ORIGIN } from '../lib/storefront.ts';

export const prerender = false;

const CANONICAL = `${STOREFRONT_ORIGIN}/canchas`;
const CACHE_CONTROL = 'public, s-maxage=300, stale-while-revalidate=60';
const UNAVAILABLE = {
  title: 'Canchas no disponibles - Vibe',
  lead: 'Tuvimos un problema para cargar las ciudades. Probá de nuevo en un minuto.',
};

export const GET: APIRoute = async () => {
  try {
    const api = createPublicApi({ baseUrl: PUBLIC_API_URL, timeoutMs: API_TIMEOUT_MS });
    const view = toHubIndexView(await api.hubIndex());
    return markdownResponse({
      status: 200,
      body: hubIndexMarkdown(view),
      canonical: view.canonical,
      cacheControl: CACHE_CONTROL,
    });
  } catch (error) {
    console.error('city index unavailable (markdown)', error);
    return markdownResponse({
      status: 503,
      body: markdownError(UNAVAILABLE.title, UNAVAILABLE.lead, CANONICAL),
      canonical: CANONICAL,
      cacheControl: CACHE_CONTROL,
    });
  }
};
