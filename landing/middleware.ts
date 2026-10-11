import { next, rewrite } from '@vercel/functions';
import { markdownTwinPath, wantsMarkdown } from './src/lib/markdown/negotiate.ts';

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
 * `matcher` still has to be literal — Vercel reads it statically at build time — so
 * it is kept equal to MARKDOWN_MATCHER by tests/negotiate.test.ts. A NEW public page
 * needs its line here, or it keeps answering HTML to agents.
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

export default function middleware(request: Request) {
  const { pathname } = new URL(request.url);
  /* Browsers send a long Accept list and never name text/markdown, so this only
     matches clients that asked for it on purpose. */
  if (!wantsMarkdown(request.headers.get('accept'))) return next();

  return rewrite(new URL(markdownTwinPath(pathname), request.url), {
    headers: { 'Content-Type': 'text/markdown; charset=utf-8' },
  });
}
