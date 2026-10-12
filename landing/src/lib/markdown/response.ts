/**
 * The HTTP envelope of a Markdown answer from a server-rendered route. Same status semantics
 * as the HTML page it twins (404 unknown, 410 deleted, 503 when the API fails), and headers that
 * keep the twin from competing with the page in search: noindex, plus a canonical link to the HTML.
 * A 410 is the exception to the noindex: it follows the same robots rule as its HTML page.
 */

import { robotsDirective } from '../storefront.ts';

export interface MarkdownAnswer {
  status: 200 | 404 | 410 | 503;
  body: string;
  /** The HTML page this twin describes. */
  canonical: string;
  /** Same cache policy as the HTML answer of the route. */
  cacheControl: string;
}

export function markdownResponse({ status, body, canonical, cacheControl }: MarkdownAnswer): Response {
  // A 410 caches like a 200: it is final until a new complex takes the slug.
  const cacheable = status === 200 || status === 410;
  const headers = new Headers({
    'Content-Type': 'text/markdown; charset=utf-8',
    'Cache-Control': cacheable ? cacheControl : 'no-store',
    'X-Robots-Tag': status === 410 ? robotsDirective(status) : 'noindex',
    'X-Content-Type-Options': 'nosniff',
    Vary: 'Accept',
    Link: `<${canonical}>; rel="canonical"`,
  });
  if (status === 503) headers.set('Retry-After', '60');
  return new Response(body, { status, headers });
}

/** The body of an error answer: a title and a line, as the HTML error page says them. */
export function markdownError(title: string, lead: string, canonical: string): string {
  return `---\ntitle: ${JSON.stringify(title)}\ncanonical: ${canonical}\n---\n\n# ${title}\n\n${lead}\n`;
}
