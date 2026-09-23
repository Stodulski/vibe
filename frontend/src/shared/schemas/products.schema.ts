import { z } from 'zod';
import type {
  Product,
  StockMovement,
  ProductsListResponse,
  ProductEnvelopeResponse,
  ProductStockMovementsResponse,
  ProductStockWriteResponse,
} from '@/shared/types/api.types';
import { paginationMetadataSchema } from './envelope.schema';
import { exact } from '@/shared/lib/apiParse';

// ─── Products (pos-cashbox T5a) ───

export const productSchema = exact<Product>(
  z
    .object({
      id: z.string(),
      complex_id: z.string(),
      name: z.string(),
      category: z.string().nullable().optional(),
      price: z.number(),
      tracks_stock: z.boolean(),
      stock_on_hand: z.number(),
      low_stock_threshold: z.number().nullable().optional(),
      active: z.boolean(),
      low_stock: z.boolean(),
      needs_stock_review: z.boolean(),
      created_at: z.string(),
      updated_at: z.string(),
      version: z.number().optional(),
    })
    .loose(),
);

export const stockMovementSchema = exact<StockMovement>(
  z
    .object({
      id: z.string(),
      complex_id: z.string(),
      product_id: z.string(),
      kind: z.enum(['sale', 'restock', 'adjustment', 'sale_void']),
      quantity: z.number(),
      reason: z.enum(['breakage', 'expired', 'own_consumption', 'count_correction', 'other']).nullable().optional(),
      note: z.string().nullable().optional(),
      cash_movement_id: z.string().nullable().optional(),
      sale_id: z.string().nullable().optional(),
      created_at: z.string(),
      created_by: z.string(),
    })
    .loose(),
);

export const productsListResponseSchema = z
  .object({
    products: z.array(productSchema),
  })
  .loose() satisfies z.ZodType<ProductsListResponse>;

/**
 * `{ product: Product }` — shared by `productsGet`, `productsCreate` and
 * `productsUpdate`, whose response bodies are structurally identical (see
 * `ProductEnvelopeResponse`).
 */
export const productEnvelopeSchema = z
  .object({
    product: productSchema,
  })
  .loose() satisfies z.ZodType<ProductEnvelopeResponse>;

export const productStockMovementsResponseSchema = z
  .object({
    stock_movements: z.array(stockMovementSchema),
    metadata: paginationMetadataSchema,
  })
  .loose() satisfies z.ZodType<ProductStockMovementsResponse>;

/**
 * `{ product, stock_movement }` — shared by `productsRestock` and
 * `productsAdjust`, whose response bodies are structurally identical.
 */
export const productStockWriteResponseSchema = z
  .object({
    product: productSchema,
    stock_movement: stockMovementSchema,
  })
  .loose() satisfies z.ZodType<ProductStockWriteResponse>;
