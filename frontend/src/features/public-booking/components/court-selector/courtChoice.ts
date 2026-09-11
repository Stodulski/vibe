import type { CourtAtTime } from './timeOptions';

/**
 * Whether to ask which court, once an hour is chosen.
 *
 * False when there is nothing to decide, and the caller assigns a court
 * without a question. That is the common case: most venues' courts are
 * interchangeable at a given hour, and asking anyway is a step that exists
 * only because the data has more than one row.
 *
 * True when the free courts differ in type or price. The picker then lists
 * every court as a card with its type, sport, description and price, so the
 * player compares them directly instead of reading group headings.
 */
export function needsCourtChoice(entries: CourtAtTime[]): boolean {
  if (entries.length <= 1) return false;

  const types = new Set(entries.map((e) => e.court.court_type));
  const prices = new Set(entries.map((e) => e.slot.price));
  return types.size > 1 || prices.size > 1;
}
