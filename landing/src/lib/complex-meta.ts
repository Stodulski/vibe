/**
 * Head copy of the complex page. It follows the backend prerender
 * (backend/internal/publicsite/copy.go), which is the copy crawlers read today, so the
 * title and description do not change at the cutover. Kept free of package imports so
 * node:test can run it: the page passes in the labels it looked up.
 */
import { DEFAULT_SHARE_IMAGE, STOREFRONT_ORIGIN } from './storefront.ts';

const APP_NAME = 'Vibe';
/** Caps how many sports and services the description names, as the backend does. */
const MAX_DESCRIPTION_SPORTS = 3;
const MAX_DESCRIPTION_SERVICES = 3;

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

/** "a", "a y b", "a, b y c". */
export function joinSpanish(items: readonly string[]): string {
  if (items.length <= 1) return items.join('');
  return `${items.slice(0, -1).join(', ')} y ${items[items.length - 1]}`;
}

export function complexHead(input: ComplexHeadInput): ComplexHead {
  const place = input.city ? `${input.name} en ${input.city}` : input.name;
  const title = `${place} - Reservá tu cancha | ${APP_NAME}`;

  const sports = input.sports.slice(0, MAX_DESCRIPTION_SPORTS).map((sport) => sport.toLowerCase());
  const subject = sports.length > 0 ? `Reservá canchas de ${joinSpanish(sports)} en ` : 'Reservá tu cancha en ';
  const where = input.city ? `${input.name}, ${input.city}` : input.name;
  const channel = input.online ? ' online' : ' por WhatsApp';
  const services = input.services.slice(0, MAX_DESCRIPTION_SERVICES);
  const detail = services.length > 0 ? `${services.join(', ')}.` : 'Horarios y ubicación.';

  return {
    title,
    description: `${subject}${where}${channel}. ${detail}`,
    canonical: complexCanonical(input.slug),
    image: input.ownImage || DEFAULT_SHARE_IMAGE,
    locale: 'es_AR',
  };
}

/**
 * JSON for a `<script type="application/ld+json">` body. The payload carries venue
 * names and addresses typed by owners, so `<`, `>`, `&` and the two line separators are
 * written as escapes: a name containing `</script>` cannot close the element.
 */
export function jsonLdScript(data: unknown): string {
  return JSON.stringify(data)
    .replace(/</g, '\\u003c')
    .replace(/>/g, '\\u003e')
    .replace(/&/g, '\\u0026')
    .replace(/\u2028/g, '\\u2028')
    .replace(/\u2029/g, '\\u2029');
}
