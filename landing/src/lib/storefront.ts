/**
 * Storefront switches shared by the pages the landing serves from the server.
 *
 * The app still serves the same public URLs until the cutover, so the Astro pages
 * must not compete with it in search yet. Flip STOREFRONT_INDEXABLE to true at the
 * cutover: the meta robots tag, the X-Robots-Tag header and the sitemap entries all
 * read this one constant.
 */
export const STOREFRONT_INDEXABLE = false;

/** Public origin of the storefront, used for canonical and JSON-LD URLs. */
export const STOREFRONT_ORIGIN = 'https://vibe.com.ar';

/**
 * The robots directive for a response with this status. Error answers are never
 * indexed, whatever the switch says.
 */
export function robotsDirective(status = 200): string {
  return STOREFRONT_INDEXABLE && status < 400 ? 'index, follow' : 'noindex';
}
