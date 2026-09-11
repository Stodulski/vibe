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
 * The .md twins are produced at build time by scripts/build-markdown.mjs, which
 * flattens every route into a single file: "/" -> index.md and
 * "/guias/algo" -> guias-algo.md. The mapping is derived here with the same rule
 * instead of being listed, because a list would have to be edited by hand every
 * time a guide is added and the failure is silent: the page keeps working and
 * only agents get the HTML.
 *
 * `matcher` still has to be literal — Vercel reads it statically at build time —
 * so a NEW top-level section (not a new guide) does need a line here.
 */
export const config = {
  matcher: ['/', '/privacidad', '/terminos', '/guias', '/guias/:slug'],
};

/** "/" -> "/index.md", "/guias/algo" -> "/guias-algo.md". */
function markdownTwin(pathname: string): string {
  const route = pathname.replace(/\/+$/, '') || '/';
  return route === '/' ? '/index.md' : `/${route.slice(1).replace(/\//g, '-')}.md`;
}

export default function middleware(request: Request) {
  const { pathname } = new URL(request.url);
  /* Browsers send a long Accept list and never name text/markdown, so this only
     matches clients that asked for it on purpose. */
  const wantsMarkdown = (request.headers.get('accept') || '').includes('text/markdown');

  if (!wantsMarkdown) return next();

  return rewrite(new URL(markdownTwin(pathname), request.url), {
    headers: { 'Content-Type': 'text/markdown; charset=utf-8' },
  });
}
