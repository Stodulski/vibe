import { next, rewrite } from '@vercel/functions';

/**
 * Content negotiation for agents: the same URL answers with HTML for browsers and
 * with Markdown for clients that ask for it, so an agent reading vibe.com.ar gets
 * plain prose instead of the landing page's markup.
 *
 * This runs as middleware rather than a `rewrites` entry in vercel.json because
 * those are evaluated only after the filesystem lookup — index.html always wins
 * first, and the rewrite never fires.
 *
 * The twin is the flat file of the route (see markdownTwinPath). Static pages get a
 * .md file written by scripts/build-markdown.mjs; the server-rendered storefront
 * routes (/canchas, /canchas/<city>, /c/<slug>, the .md endpoints under src/pages)
 * answer the same names from the function.
 *
 * This file imports nothing from src/: the Edge bundle does not follow an import of
 * a local .ts module (the deploy fails with "referencing unsupported modules"). So
 * the negotiation helpers live here, and src/lib/markdown/negotiate.ts re-exports
 * them for the Astro middleware and the tests.
 *
 * `matcher` has to be literal, because Vercel reads it statically at build time. A
 * NEW public page needs its line here, or it keeps answering HTML to agents.
 */
export const config = {
  matcher: [
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
  ],
};

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

export default function middleware(request: Request) {
  const { pathname } = new URL(request.url);
  /* Browsers send a long Accept list and never name text/markdown, so this only
     matches clients that asked for it on purpose. */
  if (!wantsMarkdown(request.headers.get('accept'))) return next();

  return rewrite(new URL(markdownTwinPath(pathname), request.url), {
    headers: { 'Content-Type': 'text/markdown; charset=utf-8' },
  });
}
