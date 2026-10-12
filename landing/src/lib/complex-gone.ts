/**
 * The page a deleted complex leaves at /c/<slug>: what it says, and the other courts of its
 * city. The HTML page and the Markdown twin both read this view, so they cannot say different
 * things. The copy itself is in complex-meta.ts.
 */
import { ApiError, type GoneComplex, type PublicApi } from './api.ts';
import { cityPath, toHubView, type HubComplexView, type HubData } from './hub.ts';
import { goneDescription, goneHeadline, goneLead, goneTitle } from './complex-meta.ts';
import { normalizeText } from './copy.ts';

/**
 * Cache policy of a deleted complex's answer, HTML and twin alike. The 410 is final until a new
 * complex takes the slug, so crawlers that come back to it are served from the shared cache for
 * five minutes instead of reaching the API each time. Browsers do not keep it (no max-age).
 */
export const GONE_CACHE_CONTROL = 'public, s-maxage=300';

export interface GoneView {
  /** The <title>. */
  title: string;
  /** The H1, which always keeps "por Vibe". */
  h1: string;
  lead: string;
  description: string;
  /** The city's name, as the API names it. */
  city: string;
  /** The other complexes of the city. Empty when there are none, or when the city could not be read. */
  alternatives: HubComplexView[];
  /** The city's hub path, only when the API answered it (even with no complexes); otherwise null. */
  cityHref: string | null;
}

/** The view of a deleted complex. `hub` is its city's hub, or null when that could not be read. */
export function goneView(gone: GoneComplex, hub: HubData | null): GoneView {
  const city = normalizeText(gone.city);
  const alternatives = hub ? toHubView(hub).complexes : [];
  const hasAlternatives = alternatives.length > 0;
  return {
    title: goneTitle(gone.name),
    h1: goneHeadline(gone.name),
    lead: goneLead(city, hasAlternatives),
    description: goneDescription(city, hasAlternatives),
    city,
    alternatives,
    cityHref: hub ? cityPath(gone.citySlug) : null,
  };
}

/**
 * Reads the city's hub for a deleted complex. A city the API does not know, or a hub the
 * landing cannot read, leaves the page with no courts to name; any other failure propagates.
 */
export async function loadGoneView(api: PublicApi, gone: GoneComplex): Promise<GoneView> {
  let hub: HubData | null = null;
  try {
    hub = await api.hubData(gone.citySlug);
  } catch (error) {
    if (!(error instanceof ApiError)) throw error;
    if (error.kind === 'unavailable') console.error('gone complex: city hub unavailable', error);
  }
  return goneView(gone, hub);
}
