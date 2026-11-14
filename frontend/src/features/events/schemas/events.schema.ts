import { z } from 'zod';
import { exact } from '@/shared/lib/apiParse';
import type {
  CancelOpenPlayEventResponse,
  OpenPlayEvent,
  OpenPlayEventDetailResponse,
  OpenPlayParticipant,
} from '@/shared/types/api.types';

const SPORTS = ['padel', 'tennis', 'soccer', 'basketball', 'volleyball', 'hockey', 'pickleball'] as const;

const eventSchema = exact<OpenPlayEvent>(
  z
    .object({
      id: z.string(),
      complex_id: z.string(),
      name: z.string(),
      sport: z.enum(SPORTS),
      date: z.string(),
      start_time: z.string(),
      end_time: z.string(),
      duration_minutes: z.number(),
      capacity: z.number(),
      price: z.number(),
      status: z.enum(['active', 'cancelled']),
      cancel_reason: z.string().nullable().optional(),
      cancelled_at: z.string().nullable().optional(),
      courts: z.array(z.object({ id: z.string(), name: z.string() }).loose()),
      confirmed_count: z.number(),
      held_count: z.number(),
      spots_left: z.number(),
      created_at: z.string(),
    })
    .loose(),
);

const participantSchema = exact<OpenPlayParticipant>(
  z
    .object({
      id: z.string(),
      first_name: z.string(),
      last_name: z.string(),
      phone: z.string(),
      email: z.string().nullable().optional(),
      status: z.enum(['held', 'confirmed', 'cancelled']),
      price: z.number(),
      service_fee: z.number(),
      created_at: z.string(),
      confirmed_at: z.string().nullable().optional(),
      cancelled_at: z.string().nullable().optional(),
      cancelled_by: z.enum(['participant', 'owner']).nullable().optional(),
      payment: z
        .object({
          status: z.string(),
          amount: z.number(),
          service_fee: z.number(),
          refund_amount: z.number(),
          method: z.enum(['mercadopago']),
        })
        .loose()
        .nullable()
        .optional(),
    })
    .loose(),
);

export const eventEnvelopeSchema = exact<{ event: OpenPlayEvent }>(z.object({ event: eventSchema }).loose());

export const eventsListEnvelopeSchema = exact<{ events: OpenPlayEvent[] }>(
  z.object({ events: z.array(eventSchema) }).loose(),
);

export const eventDetailEnvelopeSchema = exact<OpenPlayEventDetailResponse>(
  z.object({ event: eventSchema, participants: z.array(participantSchema) }).loose(),
);

export const cancelEventEnvelopeSchema = exact<CancelOpenPlayEventResponse>(
  z
    .object({
      event: eventSchema,
      refunds: z.array(
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
      ),
    })
    .loose(),
);
