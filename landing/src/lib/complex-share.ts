/**
 * The facts a complex's share image draws, and the URL the complex page points at. The
 * version in the URL is a hash of those facts: WhatsApp and Facebook cache a preview by its
 * URL, so the URL has to change when the picture does and stay the same when it does not.
 *
 * Pure, with no package imports, so node:test can run it.
 */
import { STOREFRONT_ORIGIN } from './storefront.ts';
import { normalizeText } from './copy.ts';

/** The size of the generated card, in pixels. The size WhatsApp, Facebook and LinkedIn ask for. */
export const SHARE_IMAGE_WIDTH = 1200;
export const SHARE_IMAGE_HEIGHT = 630;

export interface ShareFacts {
  name: string;
  city: string;
  province: string;
  /** Spanish labels of the active courts' sports, unique, in the order the card shows them. */
  sports: readonly string[];
}

/** The fields of the public complex payload the card reads. Structural, so it needs no schema import. */
export interface ShareFactsSource {
  complex: { name: string; city: string; province: string };
  courts?: readonly { sport: string; is_active: boolean }[];
}

/**
 * The sport labels of the active courts: each label once, in court order. A sport code the
 * labels do not name is left out, the same rule the complex page applies to its own copy.
 * A payload without courts has no sports.
 */
export function activeSportLabels(
  courts: readonly { sport: string; is_active: boolean }[] = [],
  labels: Readonly<Record<string, string>>,
): string[] {
  const seen = new Set<string>();
  for (const court of courts) {
    if (!court.is_active) continue;
    const label = labels[court.sport];
    if (label) seen.add(label);
  }
  return [...seen];
}

/** The card's facts from the public payload, with the sport labels the booking messages give. */
export function complexShareFacts(source: ShareFactsSource, labels: Readonly<Record<string, string>>): ShareFacts {
  return {
    name: source.complex.name,
    city: source.complex.city,
    province: source.complex.province,
    sports: activeSportLabels(source.courts, labels),
  };
}

/**
 * A short FNV-1a hash of the facts, as 8 hex characters. The facts go in as a JSON array, so
 * no two different sets of facts can join into the same string.
 */
export function shareImageVersion(facts: ShareFacts): string {
  const text = JSON.stringify([
    normalizeText(facts.name),
    normalizeText(facts.city),
    normalizeText(facts.province),
    facts.sports.map(normalizeText),
  ]);
  let hash = 0x811c9dc5;
  for (let i = 0; i < text.length; i += 1) {
    hash ^= text.charCodeAt(i);
    hash = Math.imul(hash, 0x01000193);
  }
  return (hash >>> 0).toString(16).padStart(8, '0');
}

/** The path of a complex's share image, with the version that busts the cache. */
export function shareImagePath(slug: string, facts: ShareFacts): string {
  return `/og/c/${encodeURIComponent(slug)}.png?v=${shareImageVersion(facts)}`;
}

/** The absolute share image URL the complex page writes into og:image. */
export function shareImageUrl(slug: string, facts: ShareFacts): string {
  return `${STOREFRONT_ORIGIN}${shareImagePath(slug, facts)}`;
}
