import { z } from 'zod';
import type {
  CashSession,
  CashMovement,
  CashMovementTotal,
  CashBookingPaymentTotal,
  CashSessionSummary,
  CashSessionsListResponse,
  CashSessionCurrentResponse,
  CashSessionDetailResponse,
  CashSessionOpenResponse,
  CashMovementCreateResponse,
} from '@/shared/types/api.types';
import { paginationMetadataSchema } from './envelope.schema';
import { exact } from '@/shared/lib/apiParse';

// ─── Cash (pos-cashbox T3) ───

// The five counter methods, closed: `CashMovement.method`/`CashMovementTotal.method`
// are a `TEXT` + `CHECK` column in a brand-new table (db/migrations/003_cashbox.sql),
// not a payments row written under a since-retired value — unlike `Payment.method`
// (see `payment.schema.ts`), there is no history to keep parsing, so this stays a
// hard failure on an unknown value instead of falling open.
const cashCounterMethodSchema = z.enum(['cash', 'transfer', 'debit_card', 'credit_card', 'qr_wallet']);
const cashMovementKindSchema = z.enum(['income', 'expense']);

export const cashSessionSchema = exact<CashSession>(
  z
    .object({
      id: z.string(),
      complex_id: z.string(),
      opened_at: z.string(),
      opened_by: z.string(),
      opening_cash: z.number(),
      closed_at: z.string().nullable().optional(),
      closed_by: z.string().nullable().optional(),
      counted_cash: z.number().nullable().optional(),
      expected_cash: z.number().nullable().optional(),
      difference: z.number().nullable().optional(),
      opening_note: z.string().nullable().optional(),
      closing_note: z.string().nullable().optional(),
      created_at: z.string(),
      updated_at: z.string(),
    })
    .loose(),
);

export const cashMovementSchema = exact<CashMovement>(
  z
    .object({
      id: z.string(),
      complex_id: z.string(),
      session_id: z.string(),
      kind: cashMovementKindSchema,
      category: z.enum([
        'other_income',
        'supplies',
        'salaries',
        'services',
        'maintenance',
        'cleaning',
        'withdrawal',
        'other_expense',
      ]),
      method: cashCounterMethodSchema,
      amount: z.number(),
      note: z.string().nullable().optional(),
      voids_movement_id: z.string().nullable().optional(),
      created_at: z.string(),
      created_by: z.string(),
    })
    .loose(),
);

/** One (method, kind, category) bucket's total within a session — only consumed by `cashSessionSummarySchema` below, so not exported on its own. */
const cashMovementTotalSchema = exact<CashMovementTotal>(
  z
    .object({
      method: cashCounterMethodSchema,
      kind: cashMovementKindSchema,
      // Left open (not the same 8-value enum as `CashMovement.category`
      // above): the backend's own schema declares it a bare string, the same
      // way an aggregated bucket is free to widen ahead of the client.
      category: z.string(),
      total: z.number(),
      count: z.number(),
    })
    .loose(),
);

/** One payment method's booking-payment total within the session's window — `mercadopago` included; only consumed by `cashSessionSummarySchema` below. */
const cashBookingPaymentTotalSchema = exact<CashBookingPaymentTotal>(
  z
    .object({
      method: z.enum(['mercadopago', 'cash', 'transfer', 'debit_card', 'credit_card', 'qr_wallet']),
      count: z.number(),
      amount: z.number(),
      service_fee: z.number(),
      refunded: z.number(),
    })
    .loose(),
);

export const cashSessionSummarySchema = exact<CashSessionSummary>(
  z
    .object({
      opening_cash: z.number(),
      expected_cash: z.number(),
      counted_cash: z.number().nullable().optional(),
      difference: z.number().nullable().optional(),
      movement_totals: z.array(cashMovementTotalSchema),
      booking_payments: z.array(cashBookingPaymentTotalSchema),
    })
    .loose(),
);

export const cashSessionsListResponseSchema = z
  .object({
    cash_sessions: z.array(cashSessionSchema),
    metadata: paginationMetadataSchema,
  })
  .loose() satisfies z.ZodType<CashSessionsListResponse>;

export const cashSessionCurrentResponseSchema = z
  .object({
    cash_session: cashSessionSchema,
    summary: cashSessionSummarySchema,
  })
  .loose() satisfies z.ZodType<CashSessionCurrentResponse>;

export const cashSessionDetailResponseSchema = z
  .object({
    cash_session: cashSessionSchema,
    summary: cashSessionSummarySchema,
    movements: z.array(cashMovementSchema),
  })
  .loose() satisfies z.ZodType<CashSessionDetailResponse>;

/**
 * `{ cash_session: CashSession }` — shared by `cashSessionsOpen` and
 * `cashSessionsClose`, whose response bodies are structurally identical.
 */
export const cashSessionEnvelopeSchema = z
  .object({
    cash_session: cashSessionSchema,
  })
  .loose() satisfies z.ZodType<CashSessionOpenResponse>;

/**
 * `{ cash_movement: CashMovement }` — shared by `cashMovementsCreate` and
 * `cashMovementsVoid`, whose response bodies are structurally identical.
 */
export const cashMovementEnvelopeSchema = z
  .object({
    cash_movement: cashMovementSchema,
  })
  .loose() satisfies z.ZodType<CashMovementCreateResponse>;
