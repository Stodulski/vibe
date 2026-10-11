import type { CourtWithPrices, Sport } from '../api/types';

/** One sport's courts at a complex, as the public payload shows them. */
export interface SportSummary {
  sport: Sport;
  /** Active courts of this sport. */
  courts: number;
  /** The cheapest priced hour across those courts, in centavos. Null when no band is priced. */
  minPrice: number | null;
}

/**
 * The prices of a court's bands, in centavos per hour. A band at zero or below
 * is not a price: it is left out, so a sport never reads as "desde $0".
 */
function pricedHours(court: CourtWithPrices): number[] {
  return court.prices.map((band) => band.price).filter((price) => price > 0);
}

/** The active courts grouped by sport, in the order each sport first appears in the payload. */
export function sportSummaries(courts: readonly CourtWithPrices[]): SportSummary[] {
  const bySport = new Map<Sport, { courts: number; prices: number[] }>();
  for (const court of courts) {
    if (!court.is_active) continue;
    const group = bySport.get(court.sport) ?? { courts: 0, prices: [] };
    group.courts += 1;
    group.prices.push(...pricedHours(court));
    bySport.set(court.sport, group);
  }
  return [...bySport].map(([sport, group]) => ({
    sport,
    courts: group.courts,
    minPrice: group.prices.length > 0 ? Math.min(...group.prices) : null,
  }));
}

/** The cheapest and the dearest priced hour across the active courts, in centavos. Null when none is priced. */
export function priceRange(courts: readonly CourtWithPrices[]): { min: number; max: number } | null {
  const prices = courts.filter((court) => court.is_active).flatMap(pricedHours);
  if (prices.length === 0) return null;
  return { min: Math.min(...prices), max: Math.max(...prices) };
}
