/**
 * GET /robots.txt, prerendered at build time. The base is the same bytes the static
 * public/robots.txt served before. The storefront sitemap line is added only when
 * STOREFRONT_INDEXABLE is on, which the build reads.
 */
import { robotsTxt } from '../lib/robots.ts';
import { STOREFRONT_INDEXABLE } from '../lib/storefront.ts';
import base from '../lib/robots-base.txt?raw';

export const prerender = true;

export function GET() {
  return new Response(robotsTxt(base, STOREFRONT_INDEXABLE), {
    headers: { 'Content-Type': 'text/plain; charset=utf-8' },
  });
}
