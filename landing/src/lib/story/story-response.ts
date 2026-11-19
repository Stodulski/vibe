/**
 * The story picture endpoint's answer. A malformed request is a 404 before anything is read. A
 * story with nothing true to show (no free hour, the freed slot taken again) is a 404 too, never
 * cached, so the publisher skips it. A drawn story is a JPEG cached for a minute: Instagram reads
 * it once when the story is created, and the grid it shows changes by the minute. The reads and
 * the render are passed in, so node:test can drive the answers with stubs.
 */
import { ApiError } from '../api.ts';
import type { ShareFacts } from '../complex-share.ts';
import { failure } from '../og/complex-image.ts';
import { parseStoryRequest, storyText, type StoryAvailability, type StoryText } from './story-text.ts';

export const STORY_CACHE_CONTROL = 'public, max-age=60, s-maxage=60';

export interface StoryImageDeps {
  readFacts(slug: string): Promise<ShareFacts>;
  readAvailability(slug: string, date: string, duration?: number): Promise<StoryAvailability>;
  render(text: StoryText, facts: ShareFacts): Promise<ArrayBuffer>;
  /** Today's venue date, YYYY-MM-DD. */
  today: string;
  /** Spanish sport labels by API value. */
  labels: Readonly<Record<string, string>>;
}

function readFailure(error: unknown): Response {
  if (error instanceof ApiError && error.kind === 'not_found') return failure(404, 'Complejo no encontrado');
  if (error instanceof ApiError && error.kind === 'gone') return failure(410, 'Complejo dado de baja');
  if (error instanceof ApiError) {
    console.error('story image: read failed', error.kind, error.status);
    return failure(503, 'Imagen no disponible por ahora', { 'Retry-After': '60' });
  }
  console.error('story image: unexpected read failure', error);
  return failure(500, 'Error al generar la imagen');
}

export async function storyImageResponse(
  slug: string,
  piece: string,
  query: URLSearchParams,
  deps: StoryImageDeps,
): Promise<Response> {
  const request = parseStoryRequest(piece, query, deps.today);
  if (!request) return failure(404, 'Imagen no encontrada');

  let facts: ShareFacts;
  let grid: StoryAvailability;
  try {
    [facts, grid] = await Promise.all([
      deps.readFacts(slug),
      deps.readAvailability(slug, request.date, request.duration),
    ]);
  } catch (error) {
    return readFailure(error);
  }

  const text = storyText(facts, grid, slug, request, deps.today, deps.labels);
  if (!text) return failure(404, 'No hay turnos para mostrar');

  try {
    const jpeg = await deps.render(text, facts);
    return new Response(jpeg, {
      status: 200,
      headers: { 'Content-Type': 'image/jpeg', 'Cache-Control': STORY_CACHE_CONTROL },
    });
  } catch (error) {
    console.error('story image: render failed', error);
    return failure(500, 'Error al generar la imagen');
  }
}
