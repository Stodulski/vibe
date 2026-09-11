import { z } from 'zod';
import type { Payment } from '@/shared/types/api.types';
import { exact } from '@/shared/lib/apiParse';

// ─── Payment ───

/**
 * `method` and `status` stay `z.string()`, matching the handwritten type: the
 * server's payment-method and payment-status vocabularies aren't modeled as
 * TS unions in `payment.ts`, so narrowing them here would validate against a
 * contract stricter than the one this codebase actually declares.
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
      mp_payment_id: z.string().optional(),
      mp_preference_id: z.string().optional(),
      refund_amount: z.number(),
      created_at: z.string(),
      updated_at: z.string(),
    })
    .loose(),
);
