/**
 * Markdown twin of a complex page (/c/<slug>). The payload is validated with the same schema
 * the HTML page uses, and the facts come from the same head copy and labels, so the two say the
 * same things. Status answers match the HTML page: 404 unknown, 503 when the API fails.
 */
import type { APIRoute } from 'astro';
import { PUBLIC_API_URL } from 'astro:env/server';
import { publicComplexResponseSchema } from '@vibe/booking/complex-response';
import { BOOKING_MESSAGES } from '@vibe/booking/messages';
import { formatPrice } from '@vibe/ui';
import { ApiError, API_TIMEOUT_MS, createPublicApi } from '../lib/api.ts';
import { citySlugOf } from '../lib/hub.ts';
import { complexCanonical } from '../lib/complex-meta.ts';
import { complexFactsOf, type ComplexLabels } from '../lib/markdown/complex-facts.ts';
import { complexMarkdown } from '../lib/markdown/complex.ts';
import { markdownError, markdownResponse } from '../lib/markdown/response.ts';

export const prerender = false;

const CACHE_CONTROL = 'public, s-maxage=60, stale-while-revalidate=300';

const labels: ComplexLabels = {
  sport: (key) => (BOOKING_MESSAGES.courts.sportTypes as Record<string, string>)[key] ?? key,
  weekday: (key) => (BOOKING_MESSAGES.complex.days as Record<string, string>)[key] ?? key,
  amenity: (key) => (BOOKING_MESSAGES.complex.amenities as Record<string, string>)[key] ?? key,
  price: (cents) => formatPrice(cents),
};

const COPY = {
  not_found: { title: 'Complejo no encontrado - Vibe', lead: 'No encontramos ese complejo en Vibe.' },
  unavailable: { title: 'Complejo no disponible - Vibe', lead: 'No podemos mostrar este complejo ahora. Probá de nuevo en un minuto.' },
};

export const GET: APIRoute = async ({ params }) => {
  const slug = params.slug ?? '';
  const canonical = complexCanonical(slug);

  try {
    const api = createPublicApi({ baseUrl: PUBLIC_API_URL, timeoutMs: API_TIMEOUT_MS });
    const raw = await api.complex(slug);
    const parsed = publicComplexResponseSchema.safeParse(raw);
    if (!parsed.success) throw new ApiError('unavailable', 200, { cause: parsed.error });

    const payload = {
      ...parsed.data,
      complex: { ...parsed.data.complex, city_slug: citySlugOf(raw) },
    };
    const facts = complexFactsOf(slug, payload, labels);
    return markdownResponse({
      status: 200,
      body: complexMarkdown(facts),
      canonical,
      cacheControl: CACHE_CONTROL,
    });
  } catch (error) {
    if (error instanceof ApiError && error.kind === 'not_found') return fail('not_found', canonical);
    console.error('complex page unavailable (markdown)', error);
    return fail('unavailable', canonical);
  }
};

function fail(kind: keyof typeof COPY, canonical: string): Response {
  const copy = COPY[kind];
  return markdownResponse({
    status: kind === 'not_found' ? 404 : 503,
    body: markdownError(copy.title, copy.lead, canonical),
    canonical,
    cacheControl: CACHE_CONTROL,
  });
}
