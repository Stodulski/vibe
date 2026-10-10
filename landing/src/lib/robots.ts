/**
 * robots.txt for vibe.com.ar. The base is the policy the site has always served, kept
 * byte for byte in robots-base.txt. The storefront sitemap line is added only while
 * STOREFRONT_INDEXABLE is on, so with the switch off the output equals the old static file.
 *
 * Pure: no Astro imports, so it runs under node:test.
 */
import { STOREFRONT_ORIGIN } from './storefront.ts';

/** Path of the storefront sitemap on this host. */
export const STOREFRONT_SITEMAP_PATH = '/sitemap-storefront.xml';

export function robotsTxt(base: string, indexable: boolean): string {
  const text = base.endsWith('\n') ? base : `${base}\n`;
  if (!indexable) return text;
  return `${text}Sitemap: ${STOREFRONT_ORIGIN}${STOREFRONT_SITEMAP_PATH}\n`;
}
