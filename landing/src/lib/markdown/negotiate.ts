/**
 * Content negotiation for agents. The same URL answers with HTML for browsers and with the
 * page's Markdown twin for clients that ask for it.
 *
 * Pure: no framework and no network, so it runs under node:test and the Vercel middleware
 * imports it as is. landing/middleware.ts keeps a literal copy of MARKDOWN_MATCHER, because
 * Vercel reads `config.matcher` statically; tests/negotiate.test.ts keeps the two equal.
 */

/**
 * The routes the Vercel middleware runs on. A `:param` matches one non-empty segment.
 * A new public page needs its line here, or it keeps answering HTML to agents.
 */
export const MARKDOWN_MATCHER = [
  '/',
  '/privacidad',
  '/terminos',
  '/guias',
  '/guias/:slug',
  '/tutoriales',
  '/tutoriales/:slug',
  '/canchas',
  '/canchas/:city',
  '/c/:slug',
] as const;

/**
 * Whether the client prefers Markdown over HTML. Browsers send a long Accept list that never
 * names text/markdown, so they keep HTML. An agent that lists markdown with a higher quality
 * than HTML (or lists nothing else) gets the twin. A wildcard counts for HTML.
 */
export function wantsMarkdown(accept: string | null | undefined): boolean {
  if (!accept) return false;
  const markdown = qualityOf(accept, 'text/markdown');
  if (markdown <= 0) return false;
  return markdown > qualityOf(accept, 'text/html');
}

/** The q-value the header gives a media type: exact match first, then the wildcard, else 0. */
function qualityOf(accept: string, mediaType: string): number {
  let exact: number | null = null;
  let wildcard: number | null = null;
  for (const part of accept.split(',')) {
    const [rawType = '', ...params] = part.split(';');
    const type = rawType.trim().toLowerCase();
    const q = parseQuality(params);
    if (type === mediaType) exact = exact === null ? q : Math.max(exact, q);
    else if (type === '*/*') wildcard = wildcard === null ? q : Math.max(wildcard, q);
  }
  return exact ?? wildcard ?? 0;
}

function parseQuality(params: string[]): number {
  for (const param of params) {
    const [key = '', value = ''] = param.split('=');
    if (key.trim().toLowerCase() !== 'q') continue;
    const q = Number(value.trim());
    return Number.isFinite(q) ? Math.min(Math.max(q, 0), 1) : 1;
  }
  return 1;
}

/**
 * The flat file a route's Markdown twin lives in, the same rule build-markdown.mjs uses for
 * the static pages: "/" -> "/index.md", "/guias/algo" -> "/guias-algo.md". The SSR routes
 * follow it too, so "/c/la-zona-f5" -> "/c-la-zona-f5.md".
 */
export function markdownTwinPath(pathname: string): string {
  const route = pathname.replace(/\/+$/, '') || '/';
  return route === '/' ? '/index.md' : `/${route.slice(1).replace(/\//g, '-')}.md`;
}

/** Whether the Vercel matcher runs on this route (a path, not a twin). */
export function matcherCovers(route: string): boolean {
  const path = route.replace(/\/+$/, '') || '/';
  const segments = path.split('/');
  return MARKDOWN_MATCHER.some((pattern) => {
    const want = pattern.split('/');
    if (want.length !== segments.length) return false;
    return want.every((part, i) => (part.startsWith(':') ? segments[i] !== '' : part === segments[i]));
  });
}
