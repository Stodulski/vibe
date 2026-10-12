/**
 * The share image endpoint's answer. A complex the API reads gets its card as a PNG, cached
 * by the CDN for a day. A failure is never cached: a crawler that stored a 404, a 410, a 503
 * or a 500 as the image of a complex would keep showing no card, or the wrong one, for as long
 * as the cache lasts. So every failure says no-store, and the crawler asks again.
 *
 * The API read and the render are passed in, so node:test can drive the answers with stubs.
 */
import { ApiError } from '../api.ts';
import type { ShareFacts } from '../complex-share.ts';

export interface ComplexImageDeps {
  /** Reads the complex and returns its card facts. Throws ApiError as the public API client does. */
  readFacts(slug: string): Promise<ShareFacts>;
  render(facts: ShareFacts): Promise<ArrayBuffer>;
}

const IMAGE_CACHE_CONTROL = 'public, s-maxage=86400, stale-while-revalidate=604800';
const FAILURE_HEADERS = { 'Cache-Control': 'no-store', 'Content-Type': 'text/plain; charset=utf-8' };

function failure(status: number, message: string, extra: Record<string, string> = {}): Response {
  return new Response(message, { status, headers: { ...FAILURE_HEADERS, ...extra } });
}

export async function complexImageResponse(slug: string, deps: ComplexImageDeps): Promise<Response> {
  let facts: ShareFacts;
  try {
    facts = await deps.readFacts(slug);
  } catch (error) {
    if (error instanceof ApiError && error.kind === 'not_found') return failure(404, 'Complejo no encontrado');
    if (error instanceof ApiError && error.kind === 'gone') return failure(410, 'Complejo dado de baja');
    if (error instanceof ApiError) {
      console.error('share image: complex read failed', error.kind, error.status);
      return failure(503, 'Imagen no disponible por ahora', { 'Retry-After': '60' });
    }
    console.error('share image: unexpected read failure', error);
    return failure(500, 'Error al generar la imagen');
  }

  try {
    const png = await deps.render(facts);
    return new Response(png, {
      status: 200,
      headers: { 'Content-Type': 'image/png', 'Cache-Control': IMAGE_CACHE_CONTROL },
    });
  } catch (error) {
    console.error('share image: render failed', error);
    return failure(500, 'Error al generar la imagen');
  }
}
