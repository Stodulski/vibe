import type { Spec } from './spec';

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

export type CourtAvailability = Spec<'AvailabilityCourt'>;

export type AvailabilityData = Spec<'Availability'>;
