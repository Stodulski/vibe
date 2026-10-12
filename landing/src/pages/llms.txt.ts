/**
 * GET /llms.txt: the llmstxt.org index. Answered on every request, so a city or complex created
 * since the last deploy is listed without one. The static part is src/data/llms-static.json,
 * which the build writes from the built pages; the city and complex sections come from the
 * public API. When the API cannot answer, the index still goes out with the static sections.
 *
 * The CDN keeps an answer for an hour, so a new complex can take that long to show up here.
 */
import type { APIRoute } from 'astro';
import { PUBLIC_API_URL } from 'astro:env/server';
import { API_TIMEOUT_MS, createPublicApi } from '../lib/api.ts';
import { llmsTxt } from '../lib/markdown/llms-txt.ts';
import llmsStatic from '../data/llms-static.json';

export const prerender = false;

const CACHE_CONTROL = 'public, s-maxage=3600, stale-while-revalidate=86400';

export const GET: APIRoute = async () => {
  const api = createPublicApi({ baseUrl: PUBLIC_API_URL, timeoutMs: API_TIMEOUT_MS });
  const body = await llmsTxt(llmsStatic, api, (message, detail) => console.error(message, detail));
  return new Response(body, {
    status: 200,
    headers: {
      'Content-Type': 'text/plain; charset=utf-8',
      'Cache-Control': CACHE_CONTROL,
    },
  });
};
