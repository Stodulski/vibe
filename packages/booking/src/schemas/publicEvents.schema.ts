import '../lib/zodConfig';
import { z } from 'zod';
import type {
  PublicEventCancelResponse,
  PublicEventCancellation,
  PublicEventListResponse,
  PublicEventRegistration,
  PublicEventResponse,
  PublicEventSignUp,
  PublicOpenPlayEvent,
} from '../api/types';
import { exact } from '../lib/apiParse';
import { refundEnvelopeSchema } from './publicBooking.schema';

export const publicOpenPlayEventSchema = exact<PublicOpenPlayEvent>(
  z
    .object({
      id: z.string(),
      name: z.string(),
      // Open vocabularies: a sport or status this build predates still renders.
      sport: z.string(),
      date: z.string(),
      start_time: z.string(),
      end_time: z.string(),
      duration_minutes: z.number(),
      capacity: z.number(),
      spots_left: z.number(),
      price: z.number(),
      service_fee: z.number(),
      total_price: z.number(),
      status: z.string(),
      // A Go nil slice marshals to null.
      court_names: z
        .array(z.string())
        .nullable()
        .transform((v) => v ?? []),
      complex_name: z.string().optional(),
      cancellation_hours: z.number().optional(),
    })
    .loose(),
);

export const publicEventListResponseSchema = z
  .object({
    events: z
      .array(publicOpenPlayEventSchema)
      .nullable()
      .transform((v) => v ?? []),
  })
  .loose() satisfies z.ZodType<PublicEventListResponse>;

export const publicEventResponseSchema = z
  .object({ event: publicOpenPlayEventSchema })
  .loose() satisfies z.ZodType<PublicEventResponse>;

export const publicEventSignUpSchema = exact<PublicEventSignUp>(
  z
    .object({
      registration: z.object({ status: z.literal('held'), held_until: z.string() }).loose(),
      token: z.string().min(1),
      event: publicOpenPlayEventSchema,
      mp_init_point: z.string().optional(),
      mp_preference_id: z.string().optional(),
      service_fee: z.number(),
      total_price: z.number(),
    })
    .loose(),
);

const cancellationSchema = exact<PublicEventCancellation>(
  z
    .object({
      can_cancel: z.boolean(),
      within_window: z.boolean(),
      deadline: z.string(),
    })
    .loose(),
);

export const publicEventRegistrationSchema = exact<PublicEventRegistration>(
  z
    .object({
      status: z.string(),
      first_name: z.string(),
      last_name: z.string(),
      price: z.number(),
      service_fee: z.number(),
      held_until: z.string().optional(),
      event: publicOpenPlayEventSchema,
      complex_name: z.string().optional(),
      complex_address: z.string().optional(),
      cancellation: cancellationSchema,
    })
    .loose(),
);

export const publicEventCancelResponseSchema = z
  .object({
    registration: publicEventRegistrationSchema,
    refunded: z.boolean(),
    refund: refundEnvelopeSchema,
  })
  .loose() satisfies z.ZodType<PublicEventCancelResponse>;
