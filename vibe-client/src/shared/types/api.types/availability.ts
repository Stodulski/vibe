// ─── Availability ───

export interface AvailabilitySlot {
  start_time: string;
  end_time: string;
  /**
   * Where the slot sits in its trading window, in minutes from that window's
   * own midnight, never wrapped: the 00:30 slot of a 20:00-02:00 Thursday is
   * 1470, not 30.
   *
   * Sort and bucket on this, never on `start_time`. A venue trading past
   * midnight renders its closing hours as "00:00", "00:30" — which a string
   * comparison places before "20:00", putting the end of the night at the top
   * of the list and labelling it as morning.
   */
  start_min: number;
  duration_minutes: number;
  price: number;
  available: boolean;
}

export interface CourtAvailability {
  court_id: string;
  court_name: string;
  sport: string;
  court_type: string;
  /**
   * What the court is like, when the owner has written it. The server omits
   * the key entirely for a court that has none, so there is no empty string
   * to guard against — only `undefined`.
   */
  description?: string;
  /** Not sent by the server; the duration lives on each slot. */
  duration_minutes?: number;
  slots: AvailabilitySlot[];
}

export interface AvailabilityData {
  date: string;
  day: string;
  is_open: boolean;
  courts: CourtAvailability[];
}
