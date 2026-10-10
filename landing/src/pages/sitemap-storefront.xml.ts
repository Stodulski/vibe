/**
 * GET /sitemap-storefront.xml: the complex pages and city hubs the storefront serves,
 * taken from the backend sitemap. Answered on demand so each request sees the API.
 *
 * The route is 404 until STOREFRONT_INDEXABLE flips at the cutover. The `.xml` filename
 * routes as-is: Astro serves `name.xml.ts` at `/name.xml`, the same as `rss.xml.ts`.
 */
import { PUBLIC_API_URL } from 'astro:env/server';
import { createPublicApi, SITEMAP_TIMEOUT_MS } from '../lib/api.ts';
import { storefrontSitemapAnswer } from '../lib/sitemap.ts';
import { STOREFRONT_INDEXABLE } from '../lib/storefront.ts';

export const prerender = false;

export async function GET() {
  const api = createPublicApi({ baseUrl: PUBLIC_API_URL, timeoutMs: SITEMAP_TIMEOUT_MS });
  const answer = await storefrontSitemapAnswer({
    indexable: STOREFRONT_INDEXABLE,
    load: () => api.sitemapXml(),
    report: (message, detail) => console.warn(message, detail),
  });
  return new Response(answer.body, { status: answer.status, headers: answer.headers });
}
