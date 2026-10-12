import { defineMiddleware } from 'astro:middleware';
import {
  isBookPath,
  STOREFRONT_CSP_REPORT_ONLY,
  STOREFRONT_ORIGIN,
  STOREFRONT_REPORTING_ENDPOINTS,
  storefrontRobots,
} from './lib/storefront.ts';
import { markdownTwinPath, matcherCovers, wantsMarkdown } from './lib/markdown/negotiate.ts';

/**
 * Headers for the server-rendered routes. vercel.json's header rules are written for
 * static files; the same policy is set here so it does not depend on how Vercel
 * merges those rules into a function response. Static pages never reach this code.
 */
const SERVER_RENDERED_PREFIXES = ['/canchas/', '/c/'];
/** Server-rendered routes without a trailing segment: the city index. */
const SERVER_RENDERED_PATHS = ['/canchas'];

export const onRequest = defineMiddleware(async (context, next) => {
  const { pathname } = context.url;
  const serverRendered =
    SERVER_RENDERED_PATHS.includes(pathname) || SERVER_RENDERED_PREFIXES.some((prefix) => pathname.startsWith(prefix));

  // Agents asking for Markdown get the route's twin. landing/middleware.ts does this for the
  // static pages, but on Vercel it never runs for a function route, so the server-rendered
  // pages negotiate here instead.
  if (
    serverRendered &&
    matcherCovers(pathname) &&
    !isBookPath(pathname) &&
    wantsMarkdown(context.request.headers.get('accept'))
  ) {
    return context.rewrite(markdownTwinPath(pathname));
  }

  const response = await next();
  if (!serverRendered) return response;

  const headers = new Headers(response.headers);
  headers.set('X-Content-Type-Options', 'nosniff');
  headers.set('X-Frame-Options', 'DENY');
  headers.set('Referrer-Policy', 'strict-origin-when-cross-origin');
  // A page that set its own robots directive (a hub with few complexes) keeps it.
  if (!headers.has('X-Robots-Tag')) headers.set('X-Robots-Tag', storefrontRobots(pathname, response.status));
  if (pathname.startsWith('/c/')) {
    headers.set('Content-Security-Policy-Report-Only', STOREFRONT_CSP_REPORT_ONLY);
    headers.set('Reporting-Endpoints', STOREFRONT_REPORTING_ENDPOINTS);
  }
  // The HTML answer varies on Accept (agents get the twin), and names that twin. Booking steps
  // have no twin, so they say nothing about one. A 410 is cached, so it varies the same way.
  if ((response.status === 200 || response.status === 410) && !isBookPath(pathname)) {
    headers.set('Vary', 'Accept');
    headers.set('Link', `<${STOREFRONT_ORIGIN}${markdownTwinPath(pathname)}>; rel="alternate"; type="text/markdown"`);
  }

  return new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers,
  });
});
