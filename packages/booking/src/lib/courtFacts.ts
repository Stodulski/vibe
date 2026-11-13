import { BOOKING_MESSAGES } from '../messages';

const t = BOOKING_MESSAGES.courts;

/** The facts of a court a player compares, as the API sends them. Loose strings: a value this build predates still renders. */
export interface CourtFactsInput {
  court_type: string;
  surface?: string | null | undefined;
  has_lighting: boolean;
}

/**
 * The parts of a court's facts line, in reading order: the roof type always,
 * the surface only when the owner set one, the lighting only when there is some.
 * A court type or surface this build has no label for is shown as sent.
 */
export function courtFactParts(court: CourtFactsInput): string[] {
  const roof = (t.courtTypes as Record<string, string>)[court.court_type] ?? court.court_type;
  const parts = [roof];
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
export function courtFeatureNames(courts: readonly (CourtFactsInput & { is_active: boolean })[]): string[] {
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
