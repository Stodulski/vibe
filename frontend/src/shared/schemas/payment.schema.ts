import { z } from 'zod';
import type { Payment } from '@/shared/types/api.types';
import { exact } from '@/shared/lib/apiParse';

// ─── Payment ───

/**
 * `method` and `status` are the closed vocabularies `openapi.yaml` declares.
 * They used to be `z.string()` here because `payment.ts` typed them that way
 * by hand; now that `Payment` is derived from the document, an unexpected
 * value is a response-schema failure instead of a string nothing can render.
 * `status` is the payments table's own, wider vocabulary — distinct from a
 * booking's `collection_status`/`refund_status`, which were split off it.
 */
export const paymentSchema = exact<Payment>(
  z
    .object({
      id: z.string(),
      booking_id: z.string(),
      complex_id: z.string(),
      amount: z.number(),
      service_fee: z.number(),
      method: z.enum(['mercadopago', 'cash', 'transfer']),
      status: z.enum(['unpaid', 'deposit_paid', 'fully_paid', 'refunded', 'refund_pending', 'partial_refund']),
      mp_payment_id: z.string().nullable().optional(),
      mp_preference_id: z.string().nullable().optional(),
      refund_amount: z.number(),
      status_detail: z.string().nullable().optional(),
      created_at: z.string(),
      updated_at: z.string(),
    })
    .loose(),
);
