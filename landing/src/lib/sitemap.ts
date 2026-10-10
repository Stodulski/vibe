/**
 * The storefront sitemap: the backend's list of complex pages and city hubs, kept to
 * the URLs this landing serves on its own host.
 *
 * Pure: no Astro and no network, so it runs under node:test. The endpoint in
 * src/pages wires it to the API and to the flag.
 */
import { STOREFRONT_ORIGIN } from './storefront.ts';

/** The only address prefixes the storefront answers for: complex pages and city hubs. */
const STOREFRONT_PREFIXES = [`${STOREFRONT_ORIGIN}/c/`, `${STOREFRONT_ORIGIN}/canchas/`];

/** Exactly one path segment after the prefix: no further path, query, fragment or whitespace. */
const SLUG = /^[^/?#\s]+$/;

/** Whether a <loc> is a complex page or a city hub on this origin. */
export function isStorefrontLoc(loc: string): boolean {
  return STOREFRONT_PREFIXES.some((prefix) => loc.startsWith(prefix) && SLUG.test(loc.slice(prefix.length)));
}

/** The backend answered with something that is not a complete urlset. */
export class InvalidSitemap extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'InvalidSitemap';
  }
}

export interface FilteredSitemap {
  xml: string;
  /** The <loc> values that were kept. */
  kept: string[];
  /** The <loc> values that were dropped. */
  dropped: string[];
}

const URL_BLOCK = /<url>[\s\S]*?<\/url>/g;
const LOC = /<loc>\s*([^<]*?)\s*<\/loc>/;

/**
 * Keeps the storefront <url> entries of the backend's sitemap, each with its lastmod,
 * changefreq and priority, and drops every other entry. Throws InvalidSitemap when the
 * document is not a complete urlset, so a bad answer is never passed on.
 */
export function filterStorefrontSitemap(xml: string): FilteredSitemap {
  if (!/<urlset[\s>]/.test(xml) || !/<\/urlset>\s*$/.test(xml)) {
    throw new InvalidSitemap('backend sitemap is not a complete urlset');
  }

  const entries: string[] = [];
  const kept: string[] = [];
  const dropped: string[] = [];
  for (const block of xml.match(URL_BLOCK) ?? []) {
    const loc = block.match(LOC)?.[1] ?? '';
    if (isStorefrontLoc(loc)) {
      entries.push(`  ${block}`);
      kept.push(loc);
    } else {
      dropped.push(loc);
    }
  }

  const out = [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
    ...entries,
    '</urlset>',
    '',
  ].join('\n');

  return { xml: out, kept, dropped };
}

/** A built sitemap may be reused by crawlers for an hour, and for ten minutes more while it refreshes. */
export const SITEMAP_CACHE_CONTROL = 'public, s-maxage=3600, stale-while-revalidate=600';

/** Seconds a crawler should wait before it asks again after a failed read. */
export const SITEMAP_RETRY_AFTER = '300';

export interface SitemapAnswer {
  status: 200 | 404 | 503;
  headers: Record<string, string>;
  body: string;
}

export interface SitemapOptions {
  /** STOREFRONT_INDEXABLE. While false the route answers 404 and the API is not called. */
  indexable: boolean;
  /** Reads the backend sitemap document. */
  load: () => Promise<string>;
  /** Receives operator notices: a failed read, or URLs dropped by the filter. */
  report?: (message: string, detail: unknown) => void;
}

const PLAIN_TEXT = 'text/plain; charset=utf-8';

/**
 * The response for GET /sitemap-storefront.xml.
 *
 * 404 while the switch is off. 503 with Retry-After when the backend read fails or its
 * answer is not a complete urlset, never a stale or partial document. 200 otherwise,
 * with only the storefront URLs; the 200 may list none, which is the state before the
 * cutover, when the backend still publishes app.vibe.com.ar URLs.
 */
export async function storefrontSitemapAnswer({
  indexable,
  load,
  report = () => {},
}: SitemapOptions): Promise<SitemapAnswer> {
  if (!indexable) {
    return { status: 404, headers: { 'Content-Type': PLAIN_TEXT, 'Cache-Control': 'no-store' }, body: 'Not found\n' };
  }

  let filtered: FilteredSitemap;
  try {
    filtered = filterStorefrontSitemap(await load());
  } catch (error) {
    report('storefront sitemap unavailable', error);
    return {
      status: 503,
      headers: { 'Content-Type': PLAIN_TEXT, 'Cache-Control': 'no-store', 'Retry-After': SITEMAP_RETRY_AFTER },
      body: 'Sitemap temporarily unavailable\n',
    };
  }

  if (filtered.dropped.length > 0) {
    report('storefront sitemap dropped URLs outside the storefront prefixes', filtered.dropped);
  }

  return {
    status: 200,
    headers: { 'Content-Type': 'application/xml; charset=utf-8', 'Cache-Control': SITEMAP_CACHE_CONTROL },
    body: filtered.xml,
  };
}
