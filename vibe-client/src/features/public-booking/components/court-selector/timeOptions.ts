import type { AvailabilitySlot, CourtAvailability } from '@/shared/types/api.types';
import { getTimeGroup, type TimeGroup } from './slotMath';

/** One court that is free at a given time, with the slot that says so. */
export interface CourtAtTime {
  court: CourtAvailability;
  slot: AvailabilitySlot;
}

export interface TimeOption {
  startTime: string;
  /** Window position — see `AvailabilitySlot.start_min`. Orders and buckets the hour. */
  startMin: number;
  endTime: string;
  /** Only the courts actually free at this time, in the order the API sent them. */
  courts: CourtAtTime[];
  /** The cheapest of those courts. Equal to every price when they agree. */
  minPrice: number;
  /**
   * Whether those courts disagree on price. `court_prices` hangs off
   * `court_id`, so two courts at the same hour on the same day may legitimately
   * cost different amounts — a venue can charge more for the covered court.
   * Data where they always match is one venue's pricing, not a rule, so the
   * price label is chosen from this rather than assumed.
   */
  priceVaries: boolean;
  /**
   * The court type, when every court still free at this hour shares one — and
   * `null` when they do not.
   *
   * This is what an hour has to say for itself. Tapping 20:00 and only then
   * discovering that the one court left is uncovered is a label promising
   * something the next screen takes away. Saying it on the button means you
   * know before you tap, without anyone having answered a question about court
   * types they may not care about.
   *
   * Same threshold as "Última cancha" and the "desde" prefix: shown only when
   * it changes the decision. A venue whose courts are all one type has nothing
   * to warn about, so the caller drops it there.
   */
  soleType: string | null;
}

/**
 * Turns the API's court-major availability into time-major options.
 *
 * The page asked "which court?" first and then "which hour?", so the same
 * hour appeared once per court: twelve courts turned forty-five distinct
 * times into hundreds of buttons, and the duration picker was drawn twelve
 * times over. Nobody arrives wanting court 7 — they arrive wanting Saturday
 * at 20:00 — so the hour is the question, and the court is what is chosen
 * once an hour is picked.
 *
 * Unavailable slots are dropped rather than rendered as disabled buttons: a
 * time with no free court is not an option, and drawing it as one that cannot
 * be pressed is the page arguing with itself.
 */
export function buildTimeOptions(courts: CourtAvailability[]): TimeOption[] {
  const byTime = new Map<string, CourtAtTime[]>();

  for (const court of courts) {
    for (const slot of court.slots) {
      if (!slot.available) continue;
      const existing = byTime.get(slot.start_time);
      if (existing) {
        existing.push({ court, slot });
      } else {
        byTime.set(slot.start_time, [{ court, slot }]);
      }
    }
  }

  const options: TimeOption[] = [];
  for (const [startTime, entries] of byTime) {
    const first = entries[0];
    if (!first) continue; // type-level only: a key exists because something was pushed
    let min = first.slot.price;
    let max = first.slot.price;
    let oneType: string | null = first.court.court_type;
    for (const entry of entries) {
      if (entry.slot.price < min) min = entry.slot.price;
      if (entry.slot.price > max) max = entry.slot.price;
      if (entry.court.court_type !== oneType) oneType = null;
    }
    options.push({
      startTime,
      startMin: first.slot.start_min,
      // Every court at one start time shares an end time: the server derives
      // it from the requested duration, not from the court.
      endTime: first.slot.end_time,
      courts: entries,
      minPrice: min,
      priceVaries: min !== max,
      soleType: oneType,
    });
  }

  // By window position, never by the clock face. A venue open 20:00-02:00
  // publishes its last hours as "00:00" and "00:30", which a string sort puts
  // at the top of the night rather than the end of it.
  return options.sort((a, b) => a.startMin - b.startMin);
}

/**
 * Buckets time options into morning / afternoon / evening, order preserved.
 *
 * Bucketed on window position too, for the same reason it is sorted on it: a
 * club's 00:30 slot belongs to the night it closes, and reading the clock
 * face would file it under morning — at the top of the page, above the
 * evening it ends.
 */
export function groupTimeOptions(options: TimeOption[]): Map<TimeGroup, TimeOption[]> {
  const groups = new Map<TimeGroup, TimeOption[]>();
  for (const option of options) {
    const key = getTimeGroup(option.startMin);
    const existing = groups.get(key);
    if (existing) {
      existing.push(option);
    } else {
      groups.set(key, [option]);
    }
  }
  return groups;
}
