import '../lib/zodConfig';
import { z } from 'zod';
import type {
  PublicPairCancelResponse,
  PublicPairCancellation,
  PublicPairRegistration,
  PublicPairSignUp,
  PublicTournament,
  PublicTournamentCategory,
  PublicTournamentDetail,
  PublicTournamentGroup,
  PublicTournamentListResponse,
  PublicTournamentMatch,
  PublicTournamentPair,
  PublicTournamentPlayer,
  PublicTournamentSchedule,
  PublicTournamentSet,
  PublicTournamentSource,
  PublicTournamentStanding,
  PublicTournamentSummary,
} from '../api/types';
import { exact } from '../lib/apiParse';
import { refundEnvelopeSchema } from './publicBooking.schema';

/** A Go nil slice marshals to null: read it as an empty list. */
function list<T extends z.ZodType>(item: T) {
  return z
    .array(item)
    .nullable()
    .transform((v) => v ?? []);
}

const tournamentObject = z
  .object({
    id: z.string(),
    name: z.string(),
    // Open vocabularies: a sport or status this build predates still renders.
    sport: z.string(),
    start_date: z.string(),
    end_date: z.string(),
    status: z.string(),
    cancel_reason: z.string().nullable(),
    cancelled_at: z.string().nullable(),
    created_at: z.string(),
  })
  .loose();

export const publicTournamentSchema = exact<PublicTournament>(tournamentObject);

export const publicTournamentCategorySchema = exact<PublicTournamentCategory>(
  z
    .object({
      id: z.string(),
      tournament_id: z.string(),
      name: z.string(),
      format: z.string(),
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
      status: z.string(),
      draw_generated: z.boolean(),
    })
    .loose(),
);

const summarySchema = exact<PublicTournamentSummary>(
  tournamentObject.extend({ categories: list(publicTournamentCategorySchema) }),
);

export const publicTournamentListResponseSchema = z
  .object({ tournaments: list(summarySchema) })
  .loose() satisfies z.ZodType<PublicTournamentListResponse>;

const playerSchema = exact<PublicTournamentPlayer>(z.object({ first_name: z.string(), last_name: z.string() }).loose());

const pairSchema = exact<PublicTournamentPair>(
  z
    .object({
      id: z.string(),
      category_id: z.string(),
      players: list(playerSchema),
      seed: z.number().nullable(),
      group_id: z.string().nullable(),
      status: z.string(),
    })
    .loose(),
);

const standingSchema = exact<PublicTournamentStanding>(
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

const groupSchema = exact<PublicTournamentGroup>(
  z
    .object({
      id: z.string(),
      category_id: z.string(),
      name: z.string(),
      position: z.number(),
      standings: list(standingSchema),
    })
    .loose(),
);

const setSchema = exact<PublicTournamentSet>(
  z.object({ a: z.number(), b: z.number(), super: z.boolean().optional() }).loose(),
);

const sourceSchema = exact<PublicTournamentSource>(z.object({ group_id: z.string(), rank: z.number() }).loose());

const scheduleSchema = exact<PublicTournamentSchedule>(
  z
    .object({
      court_id: z.string(),
      court_name: z.string(),
      date: z.string(),
      start_time: z.string(),
      end_time: z.string(),
      duration_minutes: z.number(),
    })
    .loose(),
);

const matchSchema = exact<PublicTournamentMatch>(
  z
    .object({
      id: z.string(),
      category_id: z.string(),
      stage: z.string(),
      group_id: z.string().nullable(),
      round: z.number(),
      position: z.number(),
      pair1_id: z.string().nullable(),
      pair2_id: z.string().nullable(),
      source1: sourceSchema.nullable(),
      source2: sourceSchema.nullable(),
      bye: z.boolean(),
      status: z.string(),
      winner_pair_id: z.string().nullable(),
      sets: list(setSchema),
      walkover_loser: z.union([z.literal(0), z.literal(1), z.literal(2)]),
      schedule: scheduleSchema.nullable(),
    })
    .loose(),
);

export const publicTournamentDetailSchema = exact<PublicTournamentDetail>(
  z
    .object({
      tournament: publicTournamentSchema,
      categories: list(publicTournamentCategorySchema),
      pairs: list(pairSchema),
      groups: list(groupSchema),
      matches: list(matchSchema),
      complex_name: z.string(),
      cancellation_hours: z.number(),
    })
    .loose(),
);

export const publicPairSignUpSchema = exact<PublicPairSignUp>(
  z
    .object({
      registration: z.object({ status: z.literal('held'), held_until: z.string() }).loose(),
      token: z.string().min(1),
      tournament: publicTournamentSchema,
      category: publicTournamentCategorySchema,
      mp_init_point: z.string(),
      mp_preference_id: z.string(),
      service_fee: z.number(),
      total_price: z.number(),
    })
    .loose(),
);

const cancellationSchema = exact<PublicPairCancellation>(
  z
    .object({
      can_cancel: z.boolean(),
      within_window: z.boolean(),
      deadline: z.string(),
      walkover: z.boolean(),
    })
    .loose(),
);

export const publicPairRegistrationSchema = exact<PublicPairRegistration>(
  z
    .object({
      status: z.string(),
      pair_status: z.string(),
      held_until: z.string(),
      payer: playerSchema,
      partner: playerSchema,
      price: z.number(),
      service_fee: z.number(),
      tournament: publicTournamentSchema,
      category: publicTournamentCategorySchema,
      complex_name: z.string(),
      complex_address: z.string(),
      cancellation: cancellationSchema,
    })
    .loose(),
);

export const publicPairCancelResponseSchema = z
  .object({
    registration: publicPairRegistrationSchema,
    refunded: z.boolean(),
    refund: refundEnvelopeSchema,
  })
  .loose() satisfies z.ZodType<PublicPairCancelResponse>;
