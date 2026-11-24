import { BOOKING_MESSAGES } from '../messages';

const t = BOOKING_MESSAGES.courts;

/**
 * The facts of a court a player compares, as the API sends them. Loose strings: a value this
 * build predates still renders. Each one is optional so a payload that lacks it (an older
 * server, a cached copy) shows what it has, and the panel can ask for the details without the roof.
 */
export interface CourtFactsInput {
  court_type?: string | null | undefined;
  surface?: string | null | undefined;
  has_lighting?: boolean | null | undefined;
}

/**
 * The parts of a court's facts line, in reading order: the roof type when known,
 * the surface only when the owner set one, the lighting only when there is some.
 * A court type or surface this build has no label for is shown as sent.
 * This is the one renderer of court facts: the storefront, the panel and the landing all use it.
 */
export function courtFactParts(court: CourtFactsInput): string[] {
  const parts: string[] = [];
  if (court.court_type) parts.push((t.courtTypes as Record<string, string>)[court.court_type] ?? court.court_type);
  if (court.surface) parts.push((t.surfaces as Record<string, string>)[court.surface] ?? court.surface);
  if (court.has_lighting) parts.push(t.withLighting);
  return parts;
}

/** "Techada · Césped sintético · Con iluminación". */
export function courtFactsLine(court: CourtFactsInput): string {
  return courtFactParts(court).join(' · ');
}

/**
 * What the active courts of a complex have, as feature names for structured data: one per
 * roof type ("Cancha techada"), one per surface ("Césped sintético"), and "Iluminación"
 * when any court has lights. Unique, in that order; a value this build has no label for is left out.
 */
export function courtFeatureNames(
  courts: readonly (CourtFactsInput & { court_type: string; is_active: boolean })[],
): string[] {
  const active = courts.filter((court) => court.is_active);
  const roofs = new Set<string>();
  const surfaces = new Set<string>();
  for (const court of active) {
    const roof = (t.courtTypes as Record<string, string | undefined>)[court.court_type];
    if (roof) roofs.add(`Cancha ${roof.toLowerCase()}`);
    const surface = court.surface ? (t.surfaces as Record<string, string | undefined>)[court.surface] : undefined;
    if (surface) surfaces.add(surface);
  }
  return [...roofs, ...surfaces, ...(active.some((court) => court.has_lighting) ? [t.lighting] : [])];
}
