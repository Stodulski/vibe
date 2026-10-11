/**
 * The HTTP envelope of a Markdown answer from a server-rendered route. Same status semantics
 * as the HTML page it twins (404 unknown, 503 when the API fails), and headers that keep the
 * twin from competing with the page in search: noindex, plus a canonical link to the HTML.
 */

export interface MarkdownAnswer {
  status: 200 | 404 | 503;
  body: string;
  /** The HTML page this twin describes. */
  canonical: string;
  /** Same cache policy as the HTML answer of the route. */
  cacheControl: string;
}

export function markdownResponse({ status, body, canonical, cacheControl }: MarkdownAnswer): Response {
  const headers = new Headers({
    'Content-Type': 'text/markdown; charset=utf-8',
    'Cache-Control': status === 200 ? cacheControl : 'no-store',
    'X-Robots-Tag': 'noindex',
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
