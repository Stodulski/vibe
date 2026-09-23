import { z } from 'zod';
import type {
  Sale,
  SaleItem,
  SaleStockWarning,
  SalesListResponse,
  SaleCreateResponse,
  SaleEnvelopeResponse,
} from '@/shared/types/api.types';
import { paginationMetadataSchema } from './envelope.schema';
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
      created_at: z.string(),
      created_by: z.string(),
    })
    .loose(),
);

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

export const saleCreateResponseSchema = z
  .object({
    sale: saleSchema,
    stock_warnings: z.array(saleStockWarningSchema),
  })
  .loose() satisfies z.ZodType<SaleCreateResponse>;

/** `{ sale: Sale }` — shared by `salesGet` and `salesVoid`, whose response bodies are structurally identical. */
export const saleEnvelopeSchema = z
  .object({
    sale: saleSchema,
  })
  .loose() satisfies z.ZodType<SaleEnvelopeResponse>;
