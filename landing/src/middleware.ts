import { defineMiddleware } from 'astro:middleware';
import { robotsDirective } from './lib/storefront.ts';

/**
 * Headers for the server-rendered routes. vercel.json's header rules are written for
 * static files; the same policy is set here so it does not depend on how Vercel
 * merges those rules into a function response. Static pages never reach this code.
 */
const SERVER_RENDERED_PREFIXES = ['/canchas/'];

export const onRequest = defineMiddleware(async (context, next) => {
  const response = await next();
  const { pathname } = context.url;
  if (!SERVER_RENDERED_PREFIXES.some((prefix) => pathname.startsWith(prefix))) return response;

  const headers = new Headers(response.headers);
  headers.set('X-Content-Type-Options', 'nosniff');
  headers.set('X-Frame-Options', 'DENY');
  headers.set('Referrer-Policy', 'strict-origin-when-cross-origin');
  headers.set('X-Robots-Tag', robotsDirective(response.status));

  return new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers,
  });
});
