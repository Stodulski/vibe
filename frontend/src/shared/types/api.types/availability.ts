import type { Open, Spec } from './spec';

// ─── Availability ───

/**
 * `start_min` is where the slot sits in its trading window, in minutes from
 * that window's own midnight, never wrapped: the 00:30 slot of a 20:00-02:00
 * Thursday is 1470, not 30. Sort and bucket on it, never on `start_time` — a
 * venue trading past midnight renders its closing hours as "00:00", "00:30",
 * which a string comparison places before "20:00", putting the end of the
 * night at the top of the list and labelling it as morning.
 */
export type AvailabilitySlot = Spec<'AvailabilitySlot'>;

/**
 * `sport` and `court_type` are read through {@link Open}. This is the public
 * slot grid: a venue that adds a court in a sport this build predates must
 * cost that one court its label, not the whole day's availability.
 */
export type CourtAvailability = Omit<Spec<'AvailabilityCourt'>, 'sport' | 'court_type'> & {
  sport: Open<Spec<'AvailabilityCourt'>['sport']>;
  court_type: Open<Spec<'AvailabilityCourt'>['court_type']>;
};

export type AvailabilityData = Omit<Spec<'Availability'>, 'day' | 'courts'> & {
  day: Open<Spec<'Availability'>['day']>;
  courts: CourtAvailability[];
};
