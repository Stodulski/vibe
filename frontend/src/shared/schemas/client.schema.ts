import { z } from 'zod';
import type { Client, ClientsListResponse, ClientDetailResponse, Booking } from '@/shared/types/api.types';
import { paginationMetadataSchema } from './envelope.schema';
// `bookingSchema` lives in `booking.schema.ts`, which imports `clientSchema`
// from this file for `BookingDetailResponse` — a real two-file runtime
// cycle. `z.lazy` on both sides (see `booking.schema.ts` too) defers the
// dereference past module load, so it resolves regardless of which file a
// caller happens to import first.
import { bookingSchema } from './booking.schema';
import { exact } from '@/shared/lib/apiParse';

// ─── Client ───

export const clientSchema = exact<Client>(
  z
    .object({
      id: z.string(),
      complex_id: z.string(),
      first_name: z.string(),
      last_name: z.string(),
      phone: z.string(),
      email: z.string().optional(),
      notes: z.string().optional(),
      is_blocked: z.boolean(),
      total_bookings: z.number(),
      no_shows: z.number(),
      created_at: z.string(),
      updated_at: z.string(),
    })
    .loose(),
);

export const clientsListResponseSchema = z
  .object({
    clients: z.array(clientSchema),
    metadata: paginationMetadataSchema,
  })
  .loose() satisfies z.ZodType<ClientsListResponse>;

export const clientDetailResponseSchema = z
  .object({
    client: clientSchema,
    recent_bookings: z.array(z.lazy((): z.ZodType<Booking> => bookingSchema)),
  })
  .loose() satisfies z.ZodType<ClientDetailResponse>;

/** `{ client: Client }` — `clientsApi.update`. */
export const clientEnvelopeSchema = z
  .object({
    client: clientSchema,
  })
  .loose() satisfies z.ZodType<{ client: Client }>;
