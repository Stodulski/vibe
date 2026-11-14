import type { Body, Ok, Spec } from './spec';
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
