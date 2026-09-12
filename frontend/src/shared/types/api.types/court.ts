import type { Body, Spec } from './spec';

// ─── Court ───

export type Sport = Spec<'Court'>['sport'];
export type CourtType = Spec<'Court'>['court_type'];
export type DayType = Spec<'CourtPrice'>['day_type'];

/**
 * TODO(openapi): not in spec. `openapi.yaml` types every `duration_minutes`
 * as a bare integer and says only "Must be one of the permitted slot
 * durations" — it never lists them. The three are what the slot picker
 * offers and what the price bands are keyed on, so they stay written out
 * here until the document enumerates them.
 */
export type DurationMinutes = 60 | 90 | 120;

export type Court = Spec<'Court'>;

/**
 * `from_min`/`to_min` are the band as minutes from its weekday's own
 * midnight — `to_min` exceeds 1440 for a band running into the next day, and
 * comparing the two clock strings instead puts such a band's end before its
 * start, so a match written that way covers nothing at all. Derived by the
 * database (the `span_min` generated column), which is why every price row
 * carries them even though `openapi.yaml` leaves both out of `required`;
 * the price lookup (`create-booking-modal/pricing.ts`) reads them unguarded.
 */
export type CourtPrice = Spec<'CourtPrice'> & Required<Pick<Spec<'CourtPrice'>, 'from_min' | 'to_min'>>;

export type CourtWithPrices = Omit<Spec<'CourtWithPrices'>, 'prices'> & { prices: CourtPrice[] };

export type CreateCourtRequest = Body<'courtsCreate'>;

export type UpdateCourtRequest = Body<'courtsUpdate'>;

export type UpdatePricesRequest = Body<'courtsUpdatePrices'>;
