import type { Body, Ok, Open, Spec } from './spec';
import type { EventBlock } from './events';

// ─── Tournaments (torneos), the owner side ───

/** A tournament: its days and state. The categories come with it (`TournamentSummary`, `TournamentDetail`). */
export type Tournament = Spec<'Tournament'>;

/** A tournament in the list, with its categories. */
export type TournamentSummary = Spec<'TournamentSummary'>;

/** One category: its own capacity (pairs), price per pair, format and group settings. */
export type TournamentCategory = Spec<'TournamentCategory'>;

/** A pair with both players' contacts, its spot and the payment behind it. */
export type TournamentOwnerPair = Spec<'TournamentOwnerPair'>;

/** One pair's line of a group table. */
export type TournamentStanding = Spec<'TournamentStanding'>;

export type TournamentGroup = Spec<'TournamentGroup'>;

/** One set (games each side), or the match tiebreak that is the third set (`super`). */
export type TournamentSet = Spec<'TournamentSet'>;

export type TournamentMatch = Spec<'TournamentMatch'>;

/** Everything under a tournament, as every owner read and draw/result change answers. */
export type TournamentDetailResponse = Ok<'tournamentsGet'>;

export type TournamentsListResponse = Ok<'tournamentsList'>;

export type CreateTournamentRequest = Body<'tournamentsCreate'>;

export type CancelTournamentResponse = Ok<'tournamentsCancel'>;

export type CancelTournamentCategoryResponse = Ok<'tournamentsCategoryCancel'>;

export type DrawGenerateRequest = Body<'tournamentsDrawGenerate'>;

export type DrawMoveRequest = Body<'tournamentsDrawMove'>;

export type ScheduleMatchRequest = Body<'tournamentsMatchSchedule'>;

export type MatchResultRequest = Body<'tournamentsMatchResult'>;

/** The match `schedule`/`unschedule` answer with. */
export type TournamentMatchResponse = Ok<'tournamentsMatchSchedule'>;

/** The hours a scheduled match holds on one court, as the owner's grid draws them. */
export type MatchBlock = Spec<'MatchBlock'>;

/** What the grid draws as "taken by something that is not a booking and has its own page": an open-play event or a tournament match. */
export type CourtEventBlock = EventBlock | MatchBlock;

// ─── Public side of the tournaments, as the booking package reads them ───

/**
 * Vocabularies the server can grow are read through {@link Open}, as the
 * public events do: a state or sport this build predates must cost one label,
 * not the whole page.
 */
export type PublicTournament = Omit<Tournament, 'sport' | 'status'> & {
  sport: Open<Tournament['sport']>;
  status: Open<Tournament['status']>;
};

/** A category as a visitor sees it, with the match format and the sign-up counts. */
export type PublicTournamentCategory = Omit<TournamentCategory, 'format' | 'status'> & {
  format: Open<TournamentCategory['format']>;
  status: Open<TournamentCategory['status']>;
};

export type PublicTournamentSummary = PublicTournament & { categories: PublicTournamentCategory[] };

export interface PublicTournamentListResponse {
  tournaments: PublicTournamentSummary[];
}

/** Names only: the public page never shows a phone, an email or a payment. */
export type PublicTournamentPlayer = Spec<'TournamentPlayer'>;

export type PublicTournamentPair = Omit<Spec<'TournamentPair'>, 'status'> & {
  status: Open<Spec<'TournamentPair'>['status']>;
};

export type PublicTournamentStanding = TournamentStanding;

export type PublicTournamentGroup = TournamentGroup;

/**
 * The backend leaves `super` out of a plain set (`omitempty`) although the
 * document lists it as always present with a default, so a visitor's read
 * keeps it optional.
 */
export type PublicTournamentSet = Omit<TournamentSet, 'super'> & { super?: boolean };

export type PublicTournamentSource = NonNullable<Spec<'TournamentSource'>>;

export type PublicTournamentSchedule = NonNullable<TournamentMatch['schedule']>;

export type PublicTournamentMatch = Omit<TournamentMatch, 'stage' | 'status' | 'sets'> & {
  sets: PublicTournamentSet[];
  stage: Open<TournamentMatch['stage']>;
  status: Open<TournamentMatch['status']>;
};

export type PublicTournamentDetail = Omit<Spec<'PublicTournamentDetail'>, 'tournament' | 'categories' | 'pairs' | 'matches'> & {
  tournament: PublicTournament;
  categories: PublicTournamentCategory[];
  pairs: PublicTournamentPair[];
  matches: PublicTournamentMatch[];
};

export type PublicPairSignUpRequest = Body<'tournamentsPublicSignUp'>;

/** `POST .../pairs`: the held spot and the checkout that pays for it. */
export type PublicPairSignUp = Omit<Spec<'PublicPairSignUp'>, 'tournament' | 'category'> & {
  tournament: PublicTournament;
  category: PublicTournamentCategory;
};

/** Whether and how a pair can still cancel its own spot. */
export type PublicPairCancellation = Spec<'PublicPairRegistration'>['cancellation'];

/** A pair's own spot, read through the token the checkout returns. */
export type PublicPairRegistration = Omit<
  Spec<'PublicPairRegistration'>,
  'tournament' | 'category' | 'status' | 'pair_status'
> & {
  tournament: PublicTournament;
  category: PublicTournamentCategory;
  status: Open<Spec<'PublicPairRegistration'>['status']>;
  pair_status: Open<Spec<'PublicPairRegistration'>['pair_status']>;
};

export type PublicPairCancelRequest = Body<'tournamentsPublicRegistrationCancel'>;

export type PublicPairCancelResponse = Omit<Ok<'tournamentsPublicRegistrationCancel'>, 'registration'> & {
  registration: PublicPairRegistration;
};
