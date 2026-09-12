import { z } from 'zod';
import type { Payment } from '@/shared/types/api.types';
import { exact } from '@/shared/lib/apiParse';

// ─── Payment ───

/**
 * `method` and `status` stay `z.string()` even though `openapi.yaml` declares
 * both as closed vocabularies, and `Payment` reopens them with `Open<>` to
 * match. A payments row is history: one written before the server retired or
 * added a member still has to render, and rejecting it here would fail the
 * whole booking-detail response over a label. `BookingInfoSection`'s
 * `methodLabel` already falls back to the raw value.
 */
export const paymentSchema = exact<Payment>(
  z
    .object({
      id: z.string(),
      booking_id: z.string(),
      complex_id: z.string(),
      amount: z.number(),
      service_fee: z.number(),
      method: z.string(),
      status: z.string(),
      mp_payment_id: z.string().nullable().optional(),
      mp_preference_id: z.string().nullable().optional(),
      refund_amount: z.number(),
      status_detail: z.string().nullable().optional(),
      created_at: z.string(),
      updated_at: z.string(),
    })
    .loose(),
);
