import type { DeclaredGender, Modality, Open, RefundEnvelope, Sport } from './types';

// ─── Tournaments (public reads and pair sign-up) ───

export type TournamentStatus = 'active' | 'cancelled';

export type TournamentFormat = 'elimination' | 'groups';

/** `open` takes sign-ups while spots are left, `closed` once the draw exists. */
export type TournamentCategoryStatus = 'open' | 'closed' | 'cancelled';

export interface PublicTournament {
  id: string;
  name: string;
  sport: Open<Sport>;
  /** The first and last local day (`YYYY-MM-DD`). */
  start_date: string;
  end_date: string;
  status: Open<TournamentStatus>;
  /** Why it was cancelled, `null` while active. */
  cancel_reason: string | null;
  cancelled_at: string | null;
  created_at: string;
}

export interface PublicTournamentCategory {
  id: string;
  tournament_id: string;
  /** Composed by the server from the traits; a category created before them keeps the name it was given. */
  name: string;
  /** Caballeros, Damas or Mixto; `null` on a category created before the traits. */
  modality: Open<Modality> | null;
  /** 1ra (1) to 9na (9); `null` for a Suma or a category created before the traits. */
  level: number | null;
  /** The two players' levels added together (2 to 18); `null` for a level category. */
  suma: number | null;
  prizes: string | null;
  /** The day the category starts (`YYYY-MM-DD`); `null` on a category created before it had one. */
  starts_on: string | null;
  /** The time the category starts (`HH:MM`); `null` on a category created before it had one. */
  starts_at: string | null;
  format: Open<TournamentFormat>;
  /** Pairs. */
  capacity: number;
  /** Centavos ARS per pair. */
  price: number;
  /** Centavos ARS the payer pays on top. */
  service_fee?: number;
  /** Centavos ARS: `price` plus `service_fee`. */
  total_price?: number;
  spots_left: number;
  /** Pairs that paid. */
  confirmed_count: number;
  /** Pairs holding a spot while they pay. */
  held_count: number;
  group_size: number;
  qualifiers_per_group: number;
  /** `1` is a single set, `2` is best of three. */
  sets_to_win: 1 | 2;
  /** Games that make a set (4 to 9, 6 is standard). */
  games_per_set: number;
  /** The third set is a match tiebreak to 10 points; only meaningful when `sets_to_win` is 2. */
  super_tiebreak_decider: boolean;
  /** Court time a match gets when scheduled without a duration. */
  match_duration_minutes: number;
  status: Open<TournamentCategoryStatus>;
  draw_generated: boolean;
}

/** A tournament in the complex page's list, with its categories. */
export interface PublicTournamentSummary extends PublicTournament {
  categories: PublicTournamentCategory[];
}

export interface PublicTournamentListResponse {
  tournaments: PublicTournamentSummary[];
}

/** Names only: the public page never shows a phone, an email or a payment. */
export interface PublicTournamentPlayer {
  first_name: string;
  last_name: string;
}

export interface PublicTournamentPair {
  id: string;
  category_id: string;
  /** The payer, then the partner. */
  players: PublicTournamentPlayer[];
  seed: number | null;
  group_id: string | null;
  /** `withdrawn` for a pair that left after the draw. */
  status: Open<'active' | 'withdrawn'>;
}

export interface PublicTournamentStanding {
  pair_id: string;
  /** `1` is the group winner. */
  rank: number;
  played: number;
  won: number;
  lost: number;
  sets_for: number;
  sets_against: number;
  games_for: number;
  games_against: number;
}

export interface PublicTournamentGroup {
  id: string;
  category_id: string;
  name: string;
  position: number;
  standings: PublicTournamentStanding[];
}

/** One set (`a` is the match's first pair), or, with `super`, the points of the match tiebreak. */
export interface PublicTournamentSet {
  a: number;
  b: number;
  super?: boolean;
}

/** Where a bracket slot's pair comes from until the group finishes: the Nth of a group. */
export interface PublicTournamentSource {
  group_id: string;
  rank: number;
}

export interface PublicTournamentSchedule {
  court_id: string;
  court_name: string;
  date: string;
  start_time: string;
  end_time: string;
  duration_minutes: number;
}

/**
 * A group match (`stage` `group`, `round` the matchday) or a bracket match
 * (`stage` `bracket`, `round` 1 the first and the highest the final). A `bye`
 * has one slot that never fills and is already completed.
 */
export interface PublicTournamentMatch {
  id: string;
  category_id: string;
  stage: Open<'group' | 'bracket'>;
  group_id: string | null;
  round: number;
  position: number;
  pair1_id: string | null;
  pair2_id: string | null;
  source1: PublicTournamentSource | null;
  source2: PublicTournamentSource | null;
  bye: boolean;
  status: Open<'pending' | 'completed'>;
  winner_pair_id: string | null;
  sets: PublicTournamentSet[];
  /** `1` or `2` for a walkover, `0` otherwise. */
  walkover_loser: 0 | 1 | 2;
  schedule: PublicTournamentSchedule | null;
}

export interface PublicTournamentDetail {
  tournament: PublicTournament;
  categories: PublicTournamentCategory[];
  /** The pairs that paid. */
  pairs: PublicTournamentPair[];
  groups: PublicTournamentGroup[];
  matches: PublicTournamentMatch[];
  complex_name: string;
  cancellation_hours: number;
}

/**
 * What a player declares. Both are required when the category has a modality
 * and must fit it (`modality_mismatch`, `level_too_strong`); a category without
 * one takes the pair as it is, with neither sent.
 */
export interface PublicTournamentPartnerRequest {
  first_name: string;
  last_name: string;
  phone: string;
  email?: string | undefined;
  gender?: DeclaredGender | undefined;
  /** 1ra (1) to 9na (9). */
  level?: number | undefined;
}

/** One person signs up both players and pays the whole entry. */
export interface PublicPairSignUpRequest {
  first_name: string;
  last_name: string;
  phone: string;
  email?: string | undefined;
  /** The payer's own declaration, the same as the partner's. */
  gender?: DeclaredGender | undefined;
  level?: number | undefined;
  partner: PublicTournamentPartnerRequest;
  /** Cloudflare Turnstile token, sent only when the storefront has a site key. Single use. */
  turnstile_token?: string | undefined;
}

export interface PublicPairSignUp {
  registration: { status: 'held'; held_until: string };
  /** The payer's credential for the status and cancel routes. */
  token: string;
  tournament: PublicTournament;
  category: PublicTournamentCategory;
  mp_init_point: string;
  mp_preference_id: string;
  service_fee: number;
  total_price: number;
}

export type PairRegistrationStatus = 'held' | 'confirmed' | 'cancelled' | 'expired';

export interface PublicPairCancellation {
  can_cancel: boolean;
  /** Whether cancelling now earns the payment back (before the draw only). */
  within_window: boolean;
  deadline: string;
  /** After the draw: leaving forfeits the matches and keeps the payment. */
  walkover: boolean;
}

export interface PublicPairRegistration {
  status: Open<PairRegistrationStatus>;
  pair_status: Open<'active' | 'withdrawn'>;
  held_until: string;
  payer: PublicTournamentPlayer;
  partner: PublicTournamentPlayer;
  price: number;
  service_fee: number;
  tournament: PublicTournament;
  category: PublicTournamentCategory;
  complex_name: string;
  complex_address: string;
  cancellation: PublicPairCancellation;
}

export interface PublicPairCancelRequest {
  token: string;
}

export interface PublicPairCancelResponse {
  registration: PublicPairRegistration;
  refunded: boolean;
  refund: RefundEnvelope;
}
