import { z } from 'zod';
import type {
  Sale,
  SaleItem,
  SaleBookingPayment,
  SaleStockWarning,
  BalanceDue,
  SalesListResponse,
  SaleCreateResponse,
  SaleEnvelopeResponse,
  BalancesDueResponse,
} from '@/shared/types/api.types';
import { paginationMetadataSchema } from './envelope.schema';
import { bookingStatusSchema } from './booking.schema';
import { exact } from '@/shared/lib/apiParse';

// ─── Sales (pos-cashbox T5b) ───

// Same reasoning as `cash.schema.ts`'s `cashCounterMethodSchema`: a brand-new
// table (005_sales.sql), no history of a retired value to keep parsing, so an
// unknown method is a hard failure here rather than falling open.
const saleCounterMethodSchema = z.enum(['cash', 'transfer', 'debit_card', 'credit_card', 'qr_wallet']);

export const saleItemSchema = exact<SaleItem>(
  z
    .object({
      id: z.string(),
      complex_id: z.string(),
      sale_id: z.string(),
      product_id: z.string(),
      product_name: z.string(),
      unit_price: z.number(),
      quantity: z.number(),
      line_total: z.number(),
    })
    .loose(),
);

/** A booking balance collected in a checkout, with the summary a receipt needs. */
export const saleBookingPaymentSchema = exact<SaleBookingPayment>(
  z
    .object({
      payment_id: z.string(),
      booking_id: z.string(),
      amount: z.number(),
      court_name: z.string(),
      date: z.string(),
      start_time: z.string(),
      end_time: z.string(),
      client_name: z.string(),
    })
    .loose(),
);

export const saleSchema = exact<Sale>(
  z
    .object({
      id: z.string(),
      complex_id: z.string(),
      session_id: z.string(),
      method: saleCounterMethodSchema,
      total: z.number(),
      cash_movement_id: z.string(),
      voided_at: z.string().nullable().optional(),
      voided_by: z.string().nullable().optional(),
      void_cash_movement_id: z.string().nullable().optional(),
      items: z.array(saleItemSchema),
      booking_payments: z.array(saleBookingPaymentSchema),
      created_at: z.string(),
      created_by: z.string(),
    })
    .loose(),
);

/** One booking of today that still owes money, for the sell page's "Turnos de hoy". */
export const balanceDueSchema = exact<BalanceDue>(
  z
    .object({
      booking_id: z.string(),
      court_name: z.string(),
      date: z.string(),
      start_time: z.string(),
      end_time: z.string(),
      client_name: z.string(),
      status: bookingStatusSchema,
      total: z.number(),
      paid: z.number(),
      remaining: z.number(),
    })
    .loose(),
);

export const balancesDueResponseSchema = z
  .object({
    date: z.string(),
    bookings: z.array(balanceDueSchema),
  })
  .loose() satisfies z.ZodType<BalancesDueResponse>;

export const saleStockWarningSchema = exact<SaleStockWarning>(
  z
    .object({
      product_id: z.string(),
      product_name: z.string(),
      stock_on_hand: z.number(),
    })
    .loose(),
);

export const salesListResponseSchema = z
  .object({
    sales: z.array(saleSchema),
    metadata: paginationMetadataSchema,
  })
  .loose() satisfies z.ZodType<SalesListResponse>;

/** `sale` is null for a checkout that only collected booking balances. */
export const saleCreateResponseSchema = z
  .object({
    sale: saleSchema.nullable(),
    stock_warnings: z.array(saleStockWarningSchema),
    booking_payments: z.array(saleBookingPaymentSchema),
  })
  .loose() satisfies z.ZodType<SaleCreateResponse>;

/** `{ sale: Sale }` — shared by `salesGet` and `salesVoid`, whose response bodies are structurally identical. */
export const saleEnvelopeSchema = z
  .object({
    sale: saleSchema,
  })
  .loose() satisfies z.ZodType<SaleEnvelopeResponse>;
