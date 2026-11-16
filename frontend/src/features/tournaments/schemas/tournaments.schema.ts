import { z } from 'zod';
import { exact } from '@/shared/lib/apiParse';
import type {
  CancelTournamentCategoryResponse,
  CancelTournamentResponse,
  Tournament,
  TournamentCourtFee,
  TournamentDetailResponse,
  TournamentGroup,
  TournamentMatch,
  TournamentMatchResponse,
  TournamentOwnerCategory,
  TournamentOwnerMatch,
  TournamentOwnerPair,
  TournamentOwnerSummary,
  TournamentStanding,
  TournamentsListResponse,
} from '@/shared/types/api.types';

const nullableString = z.string().nullable();

const tournamentShape = {
  id: z.string(),
  name: z.string(),
  sport: z.string(),
  start_date: z.string(),
  end_date: z.string(),
  status: z.enum(['active', 'cancelled']),
  cancel_reason: nullableString,
  cancelled_at: nullableString,
  created_at: z.string(),
};

const tournamentSchema = exact<Tournament>(z.object(tournamentShape).loose());

const categorySchema = exact<TournamentOwnerCategory>(
  z
    .object({
      court_fee: z.number(),
      id: z.string(),
      tournament_id: z.string(),
      name: z.string(),
      modality: z.enum(['men', 'women', 'mixed']).nullable(),
      level: z.number().nullable(),
      suma: z.number().nullable(),
      prizes: nullableString,
      starts_on: nullableString,
      starts_at: nullableString,
      format: z.enum(['elimination', 'groups']),
      capacity: z.number(),
      price: z.number(),
      service_fee: z.number().optional(),
      total_price: z.number().optional(),
      spots_left: z.number(),
      confirmed_count: z.number(),
      held_count: z.number(),
      group_size: z.number(),
      qualifiers_per_group: z.number(),
      sets_to_win: z.union([z.literal(1), z.literal(2)]),
      games_per_set: z.number(),
      super_tiebreak_decider: z.boolean(),
      match_duration_minutes: z.number(),
      status: z.enum(['open', 'closed', 'cancelled']),
      draw_generated: z.boolean(),
    })
    .loose(),
);

const playerSchema = z.object({ first_name: z.string(), last_name: z.string() }).loose();
const ownerPlayerSchema = playerSchema.extend({
  phone: z.string(),
  email: nullableString,
  gender: z.enum(['men', 'women']).nullable(),
  level: z.number().nullable(),
});

const pairSchema = exact<TournamentOwnerPair>(
  z
    .object({
      id: z.string(),
      category_id: z.string(),
      players: z.array(playerSchema),
      seed: z.number().nullable(),
      group_id: nullableString,
      status: z.enum(['active', 'withdrawn']),
      payer: ownerPlayerSchema,
      partner: ownerPlayerSchema,
      registration: z
        .object({
          status: z.enum(['held', 'confirmed', 'cancelled', 'expired']),
          price: z.number(),
          service_fee: z.number(),
          held_until: z.string(),
          confirmed_at: nullableString,
          cancelled_by: z.enum(['participant', 'owner']).nullable(),
          signed_up_at: z.string(),
        })
        .loose(),
      payment: z
        .object({
          status: z.string(),
          amount: z.number(),
          service_fee: z.number(),
          refund_amount: z.number(),
        })
        .loose()
        .nullable(),
    })
    .loose(),
);

const standingSchema = exact<TournamentStanding>(
  z
    .object({
      pair_id: z.string(),
      rank: z.number(),
      played: z.number(),
      won: z.number(),
      lost: z.number(),
      sets_for: z.number(),
      sets_against: z.number(),
      games_for: z.number(),
      games_against: z.number(),
    })
    .loose(),
);

const groupSchema = exact<TournamentGroup>(
  z
    .object({
      id: z.string(),
      category_id: z.string(),
      name: z.string(),
      position: z.number(),
      standings: z.array(standingSchema),
    })
    .loose(),
);

const sourceSchema = z.object({ group_id: z.string(), rank: z.number() }).loose().nullable();

const matchShape = {
  id: z.string(),
  category_id: z.string(),
  stage: z.enum(['group', 'bracket']),
  group_id: nullableString,
  round: z.number(),
  position: z.number(),
  pair1_id: nullableString,
  pair2_id: nullableString,
  source1: sourceSchema,
  source2: sourceSchema,
  bye: z.boolean(),
  status: z.enum(['pending', 'completed']),
  winner_pair_id: nullableString,
  sets: z.array(z.object({ a: z.number(), b: z.number(), super: z.boolean() }).loose()),
  walkover_loser: z.union([z.literal(0), z.literal(1), z.literal(2)]),
  schedule: z
    .object({
      court_id: z.string(),
      court_name: z.string(),
      date: z.string(),
      start_time: z.string(),
      end_time: z.string(),
      duration_minutes: z.number(),
    })
    .loose()
    .nullable(),
};

/** What `schedule` and `unschedule` answer with: the match as a visitor would read it. */
const matchSchema = exact<TournamentMatch>(z.object(matchShape).loose());

const courtFeeSchema = exact<TournamentCourtFee>(
  z
    .object({
      side: z.union([z.literal(1), z.literal(2)]),
      chargeable: z.boolean(),
      amount: z.number(),
      charged: z.boolean(),
      method: nullableString,
      charged_at: nullableString,
      charged_by: nullableString,
    })
    .loose(),
);

const ownerMatchSchema = exact<TournamentOwnerMatch>(
  z.object({ ...matchShape, court_fee: z.array(courtFeeSchema) }).loose(),
);

const summarySchema = exact<TournamentOwnerSummary>(
  z.object({ ...tournamentShape, categories: z.array(categorySchema), unscheduled_count: z.number() }).loose(),
);

const refundsSchema = z.array(
  z
    .object({
      registration_id: z.string(),
      refund: z
        .object({
          status: z.enum(['none', 'not_eligible', 'issued', 'already_issued', 'queued', 'manual']),
          message: z.string(),
          amount: z.number().optional(),
          manual_amount: z.number().optional(),
          manual_message: z.string().optional(),
        })
        .loose(),
    })
    .loose(),
);

export const tournamentsListSchema = exact<TournamentsListResponse>(
  z.object({ tournaments: z.array(summarySchema) }).loose(),
);

export const tournamentDetailSchema = exact<TournamentDetailResponse>(
  z
    .object({
      tournament: tournamentSchema,
      categories: z.array(categorySchema),
      pairs: z.array(pairSchema),
      groups: z.array(groupSchema),
      matches: z.array(ownerMatchSchema),
    })
    .loose(),
);

export const matchEnvelopeSchema = exact<TournamentMatchResponse>(z.object({ match: matchSchema }).loose());

export const cancelTournamentSchema = exact<CancelTournamentResponse>(
  z.object({ tournament: tournamentSchema, refunds: refundsSchema }).loose(),
);

export const cancelCategorySchema = exact<CancelTournamentCategoryResponse>(
  z.object({ refunds: refundsSchema }).loose(),
);
