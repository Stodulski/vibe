/**
 * Pure parts of the city hub page: the API payload contract, the view model the
 * page renders and the JSON-LD it carries. No Astro or network imports, so the unit
 * tests run under node:test without a build.
 *
 * The copy mirrors backend/internal/publicsite/hub.go (renderCityHub) on purpose:
 * same title, description, H1 and ItemList, so the page reads the same before and
 * after the cutover.
 */
import { STOREFRONT_ORIGIN } from './storefront.ts';

/** Longest meta description a hub emits (hubDescriptionRunes in the backend). */
export const HUB_DESCRIPTION_MAX = 150;

/** How many complex names the description names before it counts the rest. */
const VISIBLE_NAMES = 3;

/** Spanish labels for the sports a court can carry (hubSportLabels in the backend). */
const SPORT_LABELS: Record<string, string> = {
  padel: 'Pádel',
  tennis: 'Tenis',
  soccer: 'Fútbol',
  basketball: 'Básquet',
  volleyball: 'Vóley',
  hockey: 'Hockey',
  pickleball: 'Pickleball',
};

export interface HubComplex {
  slug: string;
  name: string;
  address: string;
  /** Raw sport keys as the API sends them, e.g. "padel". */
  sports: string[];
}

/** The part of GET /api/v1/public/hubs/{city}/data that the page reads. */
export interface HubData {
  hub: { slug: string; name: string };
  complexes: HubComplex[];
}

export interface HubComplexView {
  slug: string;
  name: string;
  address: string;
  /** Relative link to the complex page, served by the same origin. */
  href: string;
  /** Spanish labels, in the API's order. */
  sports: string[];
}

export interface HubView {
  slug: string;
  city: string;
  title: string;
  description: string;
  h1: string;
  canonical: string;
  complexes: HubComplexView[];
  /** Serialized ItemList, safe to place inside a script element. */
  jsonLd: string;
}

export class InvalidHubData extends Error {
  constructor(reason: string) {
    super(`invalid hub payload: ${reason}`);
    this.name = 'InvalidHubData';
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function parseComplex(input: unknown): HubComplex {
  if (!isRecord(input) || typeof input.slug !== 'string' || typeof input.name !== 'string') {
    throw new InvalidHubData('a complex needs a slug and a name');
  }
  return {
    slug: input.slug,
    name: input.name,
    // A missing address should not turn a live hub into a 503; the page shows no line.
    address: typeof input.address === 'string' ? input.address : '',
    sports: Array.isArray(input.sports)
      ? input.sports.filter((sport): sport is string => typeof sport === 'string')
      : [],
  };
}

/** Validates the fields the page reads and ignores the rest of the payload. */
export function parseHubData(input: unknown): HubData {
  if (!isRecord(input)) throw new InvalidHubData('the body is not an object');
  const { hub, complexes } = input;
  if (!isRecord(hub) || typeof hub.slug !== 'string' || typeof hub.name !== 'string') {
    throw new InvalidHubData('the hub needs a slug and a name');
  }
  if (!Array.isArray(complexes)) throw new InvalidHubData('complexes is not a list');
  return {
    hub: { slug: hub.slug, name: hub.name },
    complexes: complexes.map(parseComplex),
  };
}

/** Spanish label for a sport key. An unknown key is shown as stored. */
export function sportLabel(sport: string): string {
  return Object.hasOwn(SPORT_LABELS, sport) ? SPORT_LABELS[sport] : sport;
}

/** Counts characters the way Go counts runes, so a cut never splits a letter. */
function truncateRunes(text: string, limit: number): string {
  const runes = Array.from(text);
  if (runes.length <= limit) return text;
  return `${runes.slice(0, limit - 3).join('')}...`;
}

/** "Reservá tu cancha en {City}: A, B y C." Names up to three, then "y N más". */
export function hubDescription(city: string, names: string[]): string {
  const count = names.length;
  let list = '';
  if (count > VISIBLE_NAMES) {
    list = `${names.slice(0, VISIBLE_NAMES).join(', ')} y ${count - VISIBLE_NAMES} más`;
  } else if (count > 1) {
    list = `${names.slice(0, count - 1).join(', ')} y ${names[count - 1]}`;
  } else if (count === 1) {
    list = names[0];
  }

  let description = `Reservá tu cancha en ${city}`;
  if (list) description += `: ${list}`;
  return truncateRunes(`${description}.`, HUB_DESCRIPTION_MAX);
}

/** Relative path of a complex page. Hub links use it; JSON-LD adds the origin. */
export function complexPath(slug: string): string {
  return `/c/${encodeURIComponent(slug)}`;
}

/**
 * Serializes a value for a script element. JSON.stringify does not escape "<", ">"
 * or "&", so an owner-supplied name could close the element; the escapes are still
 * valid JSON.
 */
export function jsonLdScript(value: unknown): string {
  return JSON.stringify(value).replace(
    /[<>&\u2028\u2029]/g,
    (char) => `\\u${char.charCodeAt(0).toString(16).padStart(4, '0')}`,
  );
}

/** The schema.org ItemList of a hub's complexes, with absolute URLs. */
export function hubJsonLd(city: string, complexes: HubComplex[], origin: string = STOREFRONT_ORIGIN): string {
  return jsonLdScript({
    '@context': 'https://schema.org',
    '@type': 'ItemList',
    name: `Canchas en ${city}`,
    itemListElement: complexes.map((complex, index) => ({
      '@type': 'ListItem',
      position: index + 1,
      url: `${origin}${complexPath(complex.slug)}`,
    })),
  });
}

/** Maps the API payload to what the page renders. */
export function toHubView(data: HubData, origin: string = STOREFRONT_ORIGIN): HubView {
  const { slug, name: city } = data.hub;
  return {
    slug,
    city,
    title: `Canchas en ${city} - Reservá tu cancha | Vibe`,
    description: hubDescription(city, data.complexes.map((complex) => complex.name)),
    h1: `Canchas en ${city}`,
    canonical: `${origin}/canchas/${encodeURIComponent(slug)}`,
    complexes: data.complexes.map((complex) => ({
      slug: complex.slug,
      name: complex.name,
      address: complex.address,
      href: complexPath(complex.slug),
      sports: complex.sports.map(sportLabel),
    })),
    jsonLd: hubJsonLd(city, data.complexes, origin),
  };
}
