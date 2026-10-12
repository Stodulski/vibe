/**
 * Head copy of the complex page. The title and description name what the complex
 * offers and where, so the query "canchas de <deporte> en <ciudad>" finds the page. The
 * page passes in the labels it looked up, so this module has no package imports and
 * node:test can run it.
 */
import { DEFAULT_SHARE_IMAGE, STOREFRONT_ORIGIN } from './storefront.ts';
import { cutAtWord, joinSpanish, jsonLdScript, normalizeText } from './copy.ts';

export { joinSpanish, jsonLdScript };

const APP_NAME = 'Vibe';
/** Caps how many sports and services the copy names. */
const MAX_DESCRIPTION_SPORTS = 3;
const MAX_DESCRIPTION_SERVICES = 3;
/** The title keeps the sports only while it stays within this many characters. */
const TITLE_MAX = 60;
/** The meta description stays within this many characters. */
const DESCRIPTION_MAX = 155;

export interface ComplexHeadInput {
  slug: string;
  name: string;
  city: string;
  /** Online payments on: the description says "online"; otherwise "por WhatsApp". */
  online: boolean;
  /** Spanish labels of the active courts' sports, unique, in order. */
  sports: readonly string[];
  /** Spanish labels of the complex's known amenities, in order. */
  services: readonly string[];
  /** Active courts. Zero or absent leaves the count out of the description. */
  courtCount?: number | null | undefined;
  /** The complex's own picture: its cover, else its logo. */
  ownImage?: string | null | undefined;
}

export interface ComplexHead {
  title: string;
  description: string;
  canonical: string;
  image: string;
  locale: 'es_AR';
}

export function complexCanonical(slug: string): string {
  return `${STOREFRONT_ORIGIN}/c/${encodeURIComponent(slug)}`;
}

/**
 * "{Complex}: canchas de {sports} en {City} | Vibe". When that is over 60 characters the
 * top sport alone is named; when that still does not fit, or there is no city or no
 * sport, the title is the generic "{Complex} en {City} - Reservá tu cancha | Vibe".
 */
function complexTitle(name: string, city: string, sports: readonly string[]): string {
  const place = city ? `${name} en ${city}` : name;
  const generic = `${place} - Reservá tu cancha | ${APP_NAME}`;
  if (!city || sports.length === 0) return generic;

  const full = `${name}: canchas de ${joinSpanish(sports)} en ${city} | ${APP_NAME}`;
  if (full.length <= TITLE_MAX) return full;

  const top = `${name}: canchas de ${sports[0]} en ${city} | ${APP_NAME}`;
  return top.length <= TITLE_MAX ? top : generic;
}

export function complexHead(raw: ComplexHeadInput): ComplexHead {
  const input = {
    ...raw,
    name: normalizeText(raw.name),
    city: normalizeText(raw.city),
    sports: raw.sports.map(normalizeText).filter((sport) => sport !== ''),
    services: raw.services.map(normalizeText).filter((service) => service !== ''),
  };
  const sports = input.sports.slice(0, MAX_DESCRIPTION_SPORTS).map((sport) => sport.toLowerCase());
  const courts = input.courtCount && input.courtCount > 0 ? input.courtCount : null;
  const noun = courts === null ? 'canchas' : `${courts} ${courts === 1 ? 'cancha' : 'canchas'}`;
  const subject =
    sports.length > 0
      ? `Reservá ${noun} de ${joinSpanish(sports)} en `
      : courts === null
        ? 'Reservá tu cancha en '
        : `Reservá ${noun} en `;
  const where = input.city ? `${input.name}, ${input.city}` : input.name;
  const channel = input.online ? ' online' : ' por WhatsApp';
  const services = input.services.slice(0, MAX_DESCRIPTION_SERVICES);
  const detail = services.length > 0 ? `${services.join(', ')}.` : 'Horarios y ubicación.';

  // The services are the first thing dropped when the sentence is too long for the limit.
  const place = `${subject}${where}${channel}.`;
  const withDetail = `${place} ${detail}`;
  const description = withDetail.length <= DESCRIPTION_MAX ? withDetail : cutAtWord(place, DESCRIPTION_MAX);

  return {
    title: complexTitle(input.name, input.city, sports),
    description,
    canonical: complexCanonical(input.slug),
    image: input.ownImage || DEFAULT_SHARE_IMAGE,
    locale: 'es_AR',
  };
}

/** The headline of a deleted complex's page. It is the H1, and the title when it fits. */
export function goneHeadline(name: string): string {
  return `${normalizeText(name)} ya no toma reservas por Vibe`;
}

/**
 * The <title> of a deleted complex's page: the headline, or without " por Vibe" when that
 * is over TITLE_MAX characters, the same length rule the SEO gate applies to titles.
 */
export function goneTitle(name: string): string {
  const headline = goneHeadline(name);
  return headline.length <= TITLE_MAX ? headline : `${normalizeText(name)} ya no toma reservas`;
}

/** The lead under the headline: the other courts of the city, or the other cities when there are none. */
export function goneLead(city: string, hasAlternatives: boolean): string {
  return hasAlternatives
    ? `Pero hay otras canchas en ${normalizeText(city)} donde podés reservar online.`
    : 'Mirá las canchas de otras ciudades.';
}

/** The meta description of a deleted complex's page: one sentence that points to the courts that remain. */
export function goneDescription(city: string, hasAlternatives: boolean): string {
  return hasAlternatives
    ? `Mirá otras canchas en ${normalizeText(city)} que podés reservar online en Vibe.`
    : 'Mirá las canchas de otras ciudades en Vibe.';
}
