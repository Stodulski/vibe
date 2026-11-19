/** What a visitor asks to be told about when a place frees. */
export type WaitlistTarget =
  | { kind: 'open_play'; event_id: string }
  | { kind: 'tournament_category'; category_id: string }
  | { kind: 'slot'; court_id: string; date: string; start_time: string; duration_minutes: number }
  /** Every court taken at that hour: whichever frees first sends the notice. */
  | { kind: 'slot'; court_ids: string[]; date: string; start_time: string; duration_minutes: number };

/** POST /public/complexes/{slug}/waitlist/places. Signing up twice for the same place is not an error. */
export type PublicWaitlistRequest = WaitlistTarget & {
  email: string;
  turnstile_token?: string | undefined;
};
