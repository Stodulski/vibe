/**
 * Pure parts of the city hub page: the API payload contract, the view model the page
 * renders, the SEO copy (title, H1, description, intro, FAQ), the robots rule and the
 * JSON-LD. No Astro or network imports, so the unit tests run under node:test without a
 * build.
 *
 * The API sends sport labels already in Spanish ("Pádel"). Fields that older payloads
 * lack are optional: a payload without them renders the page with the same rules, with
 * fewer facts.
 */
import { cutAtWord, joinSpanish, jsonLdScript, normalizeText } from './copy.ts';
import { robotsDirective, STOREFRONT_ORIGIN } from './storefront.ts';

export { jsonLdScript };

/** Longest meta description a hub emits, in characters. */
export const HUB_DESCRIPTION_MAX = 155;

/** Longest title that keeps the province, in characters. */
const TITLE_MAX = 60;

/** How many sports the copy names. */
const SPORTS_IN_COPY = 3;

/** How many complex names the description names before it counts the rest. */
const VISIBLE_NAMES = 3;

/**
 * A hub with fewer complexes is followed but not indexed. One complex funnels the query
 * to a single page, which search engines read as a doorway page; the complex page
 * carries that query instead.
 */
export const MIN_INDEXABLE_HUB_COMPLEXES = 2;

/** Maps a raw sport key to its label. The API sends labels; this covers a raw key only. */
const SPORT_LABELS: Record<string, string> = {
  padel: 'Pádel',
  tennis: 'Tenis',
  soccer: 'Fútbol',
  basketball: 'Básquet',
  volleyball: 'Vóley',
  hockey: 'Hockey',
  pickleball: 'Pickleball',
};

export interface SportCount {
  sport: string;
  /** Null when the payload does not carry court counts. */
  courtCount: number | null;
}

export interface HubComplex {
  slug: string;
  name: string;
  address: string;
  /** The complex's city, or empty when the payload does not carry it. */
  city: string;
  /** The cover picture, an http(s) URL, or null. */
  coverUrl: string | null;
  /** Sport labels, in the API's order. */
  sports: string[];
  /** The active courts of the complex, when the payload carries them. */
  courtCount: number | null;
  /** Courts per sport, when the payload carries them. */
  courtsBySport: SportCount[] | null;
}

export interface HubOther {
  slug: string;
  name: string;
  province: string;
  complexCount: number;
}

/** The part of GET /api/v1/public/hubs/{city}/data that the page reads. */
export interface HubData {
  hub: {
    slug: string;
    name: string;
    /** Empty when the payload does not carry it. */
    province: string;
    courtCount: number | null;
    /** The hub's sports with court counts; null when the payload does not carry the list. */
    sports: SportCount[] | null;
  };
  complexes: HubComplex[];
  otherHubs: HubOther[];
}

export interface HubComplexView {
  slug: string;
  name: string;
  address: string;
  /** Relative link to the complex page, served by the same origin. */
  href: string;
  coverUrl: string | null;
  /** Alt text of the cover: "{Club} en {City}", or the club alone when no city is known. */
  coverAlt: string;
  /** Spanish labels, in the API's order. */
  sports: string[];
  /** "2 canchas de tenis · 1 de pádel", or null when the payload has no courts per sport. */
  courtsLine: string | null;
}

export interface HubOtherView {
  name: string;
  href: string;
  label: string;
}

/** A city with other cities that have complexes, as the empty hub page links them. */
export interface NearbyHubView {
  href: string;
  /** "Canchas en {City}". */
  label: string;
  /** "2 complejos". */
  countLabel: string;
  /** In the same province as the empty hub; provinces compare without accents or case. */
  sameProvince: boolean;
}

/** What a hub with no complexes shows in place of its complex list. */
export interface EmptyHubView {
  owner: { heading: string; body: string; cta: string; href: string };
  /** Same province first, then by complex count. Cities without complexes are left out. */
  nearby: NearbyHubView[];
  /** "Canchas cerca" with a same-province city, "Otras ciudades con canchas" otherwise; null with none. */
  nearbyHeading: string | null;
}

export interface FaqItem {
  question: string;
  answer: string;
}

export interface HubView {
  slug: string;
  city: string;
  province: string;
  title: string;
  description: string;
  h1: string;
  /** The intro built from the data, or the lead of a hub with no complexes. */
  intro: string;
  canonical: string;
  robots: string;
  /** The first complex's cover, for og:image, or null. */
  ogImage: string | null;
  complexes: HubComplexView[];
  otherHubs: HubOtherView[];
  faq: FaqItem[];
  /** Serialized JSON-LD: the CollectionPage, the BreadcrumbList and, with a FAQ, the FAQPage. */
  jsonLd: string[];
  /** Set only for a hub with no complexes. The page then shows the owner block and the nearby cities. */
  empty: EmptyHubView | null;
}

export interface Crumb {
  name: string;
  url: string;
}

export class InvalidHubData extends Error {
  constructor(reason: string) {
    super(`invalid hub payload: ${reason}`);
    this.name = 'InvalidHubData';
  }
}

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/** A text field as a clean string: spaces normalized, or empty when it is not text. */
export function textField(value: unknown): string {
  return typeof value === 'string' ? normalizeText(value) : '';
}

/** A count field, or null when it is not a finite number. */
export function countField(value: unknown): number | null {
  return typeof value === 'number' && Number.isFinite(value) ? value : null;
}

/** A list of text labels, cleaned, without the empty ones. */
export function labelList(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  return value
    .filter((item): item is string => typeof item === 'string')
    .map(normalizeText)
    .filter((item) => item !== '');
}

/** Only http(s) URLs may reach an image attribute or og:image. */
function imageUrl(value: unknown): string | null {
  if (typeof value !== 'string' || value === '') return null;
  try {
    const url = new URL(value);
    return url.protocol === 'https:' || url.protocol === 'http:' ? value : null;
  } catch {
    return null;
  }
}

function sportCounts(value: unknown[]): SportCount[] {
  const counts: SportCount[] = [];
  for (const item of value) {
    if (isRecord(item) && typeof item.sport === 'string' && normalizeText(item.sport) !== '') {
      counts.push({ sport: normalizeText(item.sport), courtCount: countField(item.court_count) });
    }
  }
  return counts;
}

function parseComplex(input: unknown): HubComplex {
  if (!isRecord(input) || typeof input.slug !== 'string' || typeof input.name !== 'string') {
    throw new InvalidHubData('a complex needs a slug and a name');
  }
  return {
    slug: input.slug,
    name: normalizeText(input.name),
    // A missing address should not turn a live hub into a 503; the page shows no line.
    address: textField(input.address),
    city: textField(input.city),
    coverUrl: imageUrl(input.cover_url),
    sports: labelList(input.sports),
    courtCount: countField(input.court_count),
    courtsBySport: Array.isArray(input.courts_by_sport) ? sportCounts(input.courts_by_sport) : null,
  };
}

/** Another city. One without a slug or a name is skipped: it must not fail this page. */
function parseOtherHub(input: unknown): HubOther | null {
  if (!isRecord(input) || typeof input.slug !== 'string' || typeof input.name !== 'string') return null;
  return {
    slug: input.slug,
    name: normalizeText(input.name),
    province: textField(input.province),
    complexCount: countField(input.complex_count) ?? 0,
  };
}

/** Validates the fields the page reads and ignores the rest of the payload. */
export function parseHubData(input: unknown): HubData {
  if (!isRecord(input)) throw new InvalidHubData('the body is not an object');
  const { hub, complexes, other_hubs: otherHubs } = input;
  if (!isRecord(hub) || typeof hub.slug !== 'string' || typeof hub.name !== 'string') {
    throw new InvalidHubData('the hub needs a slug and a name');
  }
  if (!Array.isArray(complexes)) throw new InvalidHubData('complexes is not a list');
  return {
    hub: {
      slug: hub.slug,
      name: normalizeText(hub.name),
      province: textField(hub.province),
      courtCount: countField(hub.court_count),
      sports: Array.isArray(hub.sports) ? sportCounts(hub.sports) : null,
    },
    complexes: complexes.map(parseComplex),
    otherHubs: Array.isArray(otherHubs)
      ? otherHubs.map(parseOtherHub).filter((entry): entry is HubOther => entry !== null)
      : [],
  };
}

/** Spanish label for a sport key. A label, or a key this table does not know, is shown as it is. */
export function sportLabel(sport: string): string {
  return Object.hasOwn(SPORT_LABELS, sport) ? SPORT_LABELS[sport] : sport;
}

/**
 * The hub's sports, most courts first, at most three, as labels. The hub's own list is
 * used when the payload has one; otherwise the sports of its complexes, without counts.
 */
export function topSports(data: HubData): string[] {
  const counts = data.hub.sports ?? unionOfComplexSports(data.complexes);
  return [...counts]
    .sort((a, b) => (b.courtCount ?? 0) - (a.courtCount ?? 0))
    .slice(0, SPORTS_IN_COPY)
    .map((entry) => sportLabel(entry.sport));
}

function unionOfComplexSports(complexes: readonly HubComplex[]): SportCount[] {
  const seen: string[] = [];
  for (const complex of complexes) {
    for (const sport of complex.sports) {
      const label = sportLabel(sport);
      if (!seen.includes(label)) seen.push(label);
    }
  }
  return seen.map((sport) => ({ sport, courtCount: null }));
}

/** The sports as the copy writes them: lowercase, joined "a, b y c", at most three. */
function sportsPhrase(sports: readonly string[]): string {
  return joinSpanish(sports.slice(0, SPORTS_IN_COPY).map((sport) => sport.toLowerCase()));
}

function plural(count: number, one: string, many: string): string {
  return `${count} ${count === 1 ? one : many}`;
}

function runeLength(text: string): number {
  return Array.from(text).length;
}

/** Complex or city names: up to three, then "y N más". */
export function namesList(names: readonly string[]): string {
  if (names.length > VISIBLE_NAMES) {
    return `${names.slice(0, VISIBLE_NAMES).join(', ')} y ${names.length - VISIBLE_NAMES} más`;
  }
  return joinSpanish(names);
}

/**
 * "Canchas de {sports} en {City}, {Province} | Vibe". Without the province when that is
 * over 60 characters; with the top sport alone when even that is over 60. With no sports,
 * "Canchas en {City} | Vibe".
 */
export function hubTitle(city: string, province: string, sports: readonly string[]): string {
  const name = normalizeText(city);
  const region = normalizeText(province);
  if (sports.length === 0) return `Canchas en ${name} | Vibe`;

  const phrase = sportsPhrase(sports);
  const withProvince = region ? `Canchas de ${phrase} en ${name}, ${region} | Vibe` : '';
  if (withProvince && runeLength(withProvince) <= TITLE_MAX) return withProvince;

  const plain = `Canchas de ${phrase} en ${name} | Vibe`;
  if (runeLength(plain) <= TITLE_MAX) return plain;

  return `Canchas de ${sportsPhrase(sports.slice(0, 1))} en ${name} | Vibe`;
}

/** "Canchas de {sports} en {City}", or "Canchas en {City}" with no sports. */
export function hubH1(city: string, sports: readonly string[]): string {
  const name = normalizeText(city);
  return sports.length === 0 ? `Canchas en ${name}` : `Canchas de ${sportsPhrase(sports)} en ${name}`;
}

/**
 * "Reservá canchas de {sports} en {City}, {Province}. {N} complejo(s): {names}. Mirá los
 * turnos libres y reservá online." Cut at a word boundary to HUB_DESCRIPTION_MAX.
 */
export function hubDescription(
  city: string,
  province: string,
  sports: readonly string[],
  names: readonly string[],
): string {
  const name = normalizeText(city);
  const region = normalizeText(province);
  const place = region ? `${name}, ${region}` : name;
  const lead =
    sports.length === 0 ? `Reservá canchas en ${place}` : `Reservá canchas de ${sportsPhrase(sports)} en ${place}`;
  const list =
    names.length === 0
      ? ''
      : ` ${plural(names.length, 'complejo', 'complejos')}: ${namesList(names.map(normalizeText))}.`;
  return cutAtWord(`${lead}.${list} Mirá los turnos libres y reservá online.`, HUB_DESCRIPTION_MAX);
}

/**
 * The intro built from the payload: "En {City} hay N complejos con M canchas de {sports}
 * que podés reservar online en Vibe." The court part is left out when the count is unknown.
 */
export function hubIntro(
  city: string,
  complexCount: number,
  courtCount: number | null,
  sports: readonly string[],
): string {
  const name = normalizeText(city);
  const courts = courtCount === null ? '' : plural(courtCount, 'cancha', 'canchas');
  let clause = '';
  if (sports.length === 0) {
    clause = courts ? ` con ${courts}` : '';
  } else {
    clause = courts ? ` con ${courts} de ${sportsPhrase(sports)}` : ` con canchas de ${sportsPhrase(sports)}`;
  }
  const count = plural(complexCount, 'complejo', 'complejos');
  return `En ${name} hay ${count}${clause} que podés reservar online en Vibe. Elegí un complejo, mirá los turnos libres y reservá tu turno.`;
}

/**
 * The FAQ, built only from data: where each of the top sports is played (from the
 * complexes that offer it), how to book, and how many complexes take bookings.
 */
export function hubFaq(
  city: string,
  sports: readonly string[],
  complexes: ReadonlyArray<{ name: string; sports: readonly string[] }>,
): FaqItem[] {
  if (complexes.length === 0) return [];
  const name = normalizeText(city);
  const faq: FaqItem[] = [];

  for (const sport of sports.slice(0, SPORTS_IN_COPY)) {
    const names = complexes.filter((complex) => complex.sports.includes(sport)).map((complex) => normalizeText(complex.name));
    if (names.length === 0) continue;
    const lower = sport.toLowerCase();
    faq.push({
      question: `¿Dónde hay canchas de ${lower} en ${name}?`,
      answer: `En ${name} podés reservar canchas de ${lower} en ${joinSpanish(names)}.`,
    });
  }

  faq.push({
    question: `¿Cómo reservo una cancha en ${name}?`,
    answer: 'Elegí un complejo de la lista, mirá los turnos libres y confirmá tu reserva online.',
  });

  const all = complexes.map((complex) => normalizeText(complex.name));
  faq.push({
    question: `¿Cuántos complejos de ${name} reciben reservas en Vibe?`,
    answer: `${plural(all.length, 'complejo', 'complejos')}: ${joinSpanish(all)}.`,
  });
  return faq;
}

/**
 * The robots directive of a hub answer. A hub under MIN_INDEXABLE_HUB_COMPLEXES is
 * followed but not indexed. Error answers keep the directive of robotsDirective.
 */
export function hubRobots(complexCount: number, status = 200): string {
  const directive = robotsDirective(status);
  return directive === 'index, follow' && complexCount < MIN_INDEXABLE_HUB_COMPLEXES ? 'noindex, follow' : directive;
}

/** Relative path of a city hub. */
export function cityPath(slug: string): string {
  return `/canchas/${encodeURIComponent(slug)}`;
}

/** Relative path of a complex page. Hub links use it; JSON-LD adds the origin. */
export function complexPath(slug: string): string {
  return `/c/${encodeURIComponent(slug)}`;
}

const CITY_SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

/** The city slug of a public complex payload, when it is a plain slug; otherwise null. */
export function citySlugOf(payload: unknown): string | null {
  if (!isRecord(payload) || !isRecord(payload.complex)) return null;
  const slug = payload.complex.city_slug;
  return typeof slug === 'string' && CITY_SLUG.test(slug) ? slug : null;
}

/** A BreadcrumbList for the trail, serialized for a script element. */
export function breadcrumbJsonLd(trail: readonly Crumb[]): string {
  return jsonLdScript({
    '@context': 'https://schema.org',
    '@type': 'BreadcrumbList',
    itemListElement: trail.map((crumb, index) => ({
      '@type': 'ListItem',
      position: index + 1,
      name: crumb.name,
      item: crumb.url,
    })),
  });
}

/** "2 canchas de tenis · 1 de pádel": the first part in full, the rest short. */
function courtsLine(entries: readonly SportCount[] | null): string | null {
  const parts = (entries ?? []).flatMap((entry) =>
    entry.courtCount !== null && entry.courtCount > 0
      ? [{ sport: sportLabel(entry.sport).toLowerCase(), courtCount: entry.courtCount }]
      : [],
  );
  if (parts.length === 0) return null;
  return parts
    .map((part, index) =>
      index === 0 ? `${plural(part.courtCount, 'cancha', 'canchas')} de ${part.sport}` : `${part.courtCount} de ${part.sport}`,
    )
    .join(' · ');
}

function complexView(complex: HubComplex): HubComplexView {
  const name = normalizeText(complex.name);
  return {
    slug: complex.slug,
    name,
    address: complex.address,
    href: complexPath(complex.slug),
    coverUrl: complex.coverUrl,
    coverAlt: complex.city ? `${name} en ${complex.city}` : name,
    sports: complex.sports.map(sportLabel),
    courtsLine: courtsLine(complex.courtsBySport),
  };
}

/** Where a complex owner signs up. The site's other CTAs link to the same URL. */
const OWNER_SIGNUP_URL = 'https://app.vibe.com.ar/register';

/**
 * The title of a hub with no complexes: "Canchas en {City}, {Province} | Vibe", without the
 * province when that is over 60 characters. A sport is not named: the city has none to show.
 */
export function emptyHubTitle(city: string, province: string): string {
  const name = normalizeText(city);
  const region = normalizeText(province);
  const withProvince = region ? `Canchas en ${name}, ${region} | Vibe` : '';
  if (withProvince && runeLength(withProvince) <= TITLE_MAX) return withProvince;
  return `Canchas en ${name} | Vibe`;
}

/** The lead of a hub with no complexes. It says there are no courts to book yet, and nothing more. */
export function emptyHubLead(city: string): string {
  return `Todavía no hay canchas para reservar en ${normalizeText(city)} con Vibe.`;
}

/** The meta description of a hub with no complexes: the lead with the province, and a line for owners. */
export function emptyHubDescription(city: string, province: string): string {
  const name = normalizeText(city);
  const region = normalizeText(province);
  const place = region ? `${name}, ${region}` : name;
  return cutAtWord(
    `Todavía no hay canchas para reservar en ${place} con Vibe. Si tenés un complejo, podés sumarlo gratis.`,
    HUB_DESCRIPTION_MAX,
  );
}

/** Province key for comparing two provinces: without accents, case or extra spaces. */
function provinceKey(province: string): string {
  return normalizeText(province).normalize('NFD').replace(/\p{Diacritic}/gu, '').toLowerCase();
}

/**
 * The other cities for a hub with no complexes: the same province first, each group by
 * complex count, most first. A city without complexes is left out, since its page is empty too.
 */
function nearbyHubsOf(province: string, otherHubs: readonly HubOther[]): NearbyHubView[] {
  const region = provinceKey(province);
  return otherHubs
    .filter((hub) => hub.complexCount > 0)
    .map((hub) => ({ hub, sameProvince: region !== '' && provinceKey(hub.province) === region }))
    .sort((a, b) => Number(b.sameProvince) - Number(a.sameProvince) || b.hub.complexCount - a.hub.complexCount)
    .map(({ hub, sameProvince }) => ({
      href: cityPath(hub.slug),
      label: `Canchas en ${hub.name}`,
      countLabel: plural(hub.complexCount, 'complejo', 'complejos'),
      sameProvince,
    }));
}

/** What a hub with no complexes shows: the owner block and the nearby cities. */
function emptyHubView(city: string, province: string, otherHubs: readonly HubOther[]): EmptyHubView {
  const nearby = nearbyHubsOf(province, otherHubs);
  let nearbyHeading: string | null = null;
  if (nearby.length > 0) {
    nearbyHeading = nearby.some((hub) => hub.sameProvince) ? 'Canchas cerca' : 'Otras ciudades con canchas';
  }
  return {
    owner: {
      heading: `¿Tenés un complejo en ${normalizeText(city)}?`,
      body: 'Vibe es gratis para gestionar tu complejo y tomar reservas online.',
      cta: 'Sumar mi complejo gratis',
      href: OWNER_SIGNUP_URL,
    },
    nearby,
    nearbyHeading,
  };
}

/** Maps the API payload to what the page renders, with the SEO copy and the JSON-LD. */
export function toHubView(data: HubData, origin: string = STOREFRONT_ORIGIN): HubView {
  const slug = data.hub.slug;
  const city = normalizeText(data.hub.name);
  const province = normalizeText(data.hub.province);
  const sports = topSports(data);
  const complexes = data.complexes.map(complexView);
  const names = complexes.map((complex) => complex.name);
  // A hub with no complexes is "Canchas en {City}" whatever sports its payload lists.
  const empty = complexes.length === 0;
  const h1 = hubH1(city, empty ? [] : sports);
  const canonical = `${origin}${cityPath(slug)}`;
  const faq = hubFaq(city, sports, complexes);

  const collectionJsonLd = jsonLdScript({
    '@context': 'https://schema.org',
    '@type': 'CollectionPage',
    name: h1,
    url: canonical,
    about: {
      '@type': 'Place',
      name: city,
      address: {
        '@type': 'PostalAddress',
        addressLocality: city,
        ...(province ? { addressRegion: province } : {}),
        addressCountry: 'AR',
      },
    },
    mainEntity: {
      '@type': 'ItemList',
      name: h1,
      itemListElement: complexes.map((complex, index) => ({
        '@type': 'ListItem',
        position: index + 1,
        url: `${origin}${complex.href}`,
        name: complex.name,
      })),
    },
  });

  const breadcrumb = breadcrumbJsonLd([
    { name: 'Inicio', url: `${origin}/` },
    { name: 'Canchas', url: `${origin}/canchas` },
    { name: city, url: canonical },
  ]);
  // A CollectionPage would list complexes, and a hub with none has nothing to list.
  const jsonLd = empty ? [breadcrumb] : [collectionJsonLd, breadcrumb];
  if (faq.length > 0) {
    jsonLd.push(
      jsonLdScript({
        '@context': 'https://schema.org',
        '@type': 'FAQPage',
        mainEntity: faq.map((item) => ({
          '@type': 'Question',
          name: item.question,
          acceptedAnswer: { '@type': 'Answer', text: item.answer },
        })),
      }),
    );
  }

  return {
    slug,
    city,
    province,
    title: empty ? emptyHubTitle(city, province) : hubTitle(city, province, sports),
    description: empty ? emptyHubDescription(city, province) : hubDescription(city, province, sports, names),
    h1,
    intro: empty ? emptyHubLead(city) : hubIntro(city, complexes.length, data.hub.courtCount, sports),
    canonical,
    robots: hubRobots(complexes.length),
    ogImage: complexes.find((complex) => complex.coverUrl !== null)?.coverUrl ?? null,
    complexes,
    otherHubs: data.otherHubs.map((hub) => ({
      name: hub.name,
      href: cityPath(hub.slug),
      label: `Canchas en ${hub.name}`,
    })),
    faq,
    jsonLd,
    empty: empty ? emptyHubView(city, province, data.otherHubs) : null,
  };
}
