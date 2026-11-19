/**
 * The story picture endpoint's answer. A malformed request is a 404 before anything is read. A
 * story with nothing true to show (no free hour, the freed slot taken again) is a 404 too, so the
 * publisher skips it; only the panel's preview of libres (?vista=1) draws a "no free hours left"
 * story instead. Nothing is cached, 200 or 404: the grid a story shows changes by the minute and
 * the publisher must see the current answer. The reads and the render are passed in, so node:test
 * can drive the answers with stubs.
 */
import { ApiError } from '../api.ts';
import type { ShareFacts } from '../complex-share.ts';
import { failure } from '../og/complex-image.ts';
import {
  emptyFreeText,
  needsPreviousNight,
  parseStoryRequest,
  previousDate,
  storyText,
  type StoryAvailability,
  type StoryText,
} from './story-text.ts';

export const STORY_CACHE_CONTROL = 'no-store';

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
  let previous: StoryAvailability | undefined;
  try {
    [facts, grid] = await Promise.all([
      deps.readFacts(slug),
      deps.readAvailability(slug, request.date, request.duration),
    ]);
    // An overnight venue files a freed 00:30 under the previous operating day.
    if (needsPreviousNight(grid, request, deps.today)) {
      previous = await deps.readAvailability(slug, previousDate(request.date), request.duration);
    }
  } catch (error) {
    return readFailure(error);
  }

  const preview = request.piece === 'libres' && query.get('vista') === '1';
  const text =
    storyText(facts, grid, slug, request, deps.today, deps.labels, previous) ??
    (preview ? emptyFreeText(facts, slug, request, deps.today) : null);
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
