import { z } from 'zod';
import { ES_AR } from '@/shared/i18n/es_AR';
import { COUNTER_PAYMENT_METHODS } from '@/shared/lib/paymentMethods';
import { ADJUSTMENT_REASONS } from '../lib/adjustmentReasons';
import { MAX_PRODUCT_PRICE_PESOS, MAX_RESTOCK_COST_PESOS, MAX_STOCK_QUANTITY } from '../lib/money';

const t = ES_AR;

const optionalNoteSchema = z.string().max(500, t.validation.maxChars500).optional().or(z.literal(''));

// Whole pesos only, may be 0 (owner decision: "precio en pesos ... may be
// 0") — same NaN-from-an-empty-number-input handling as `cash.schema.ts`'s
// `sessionCashSchema`.
const priceSchema = z
  .number({ message: t.validation.amountRequired })
  .min(0, t.validation.amountNonNegative)
  .max(MAX_PRODUCT_PRICE_PESOS, t.validation.movementAmountTooLarge)
  .int(t.validation.amountMustBeWhole);

/**
 * The create/edit form. `thresholdLocked` is not a static property of the
 * shape — it depends on whether the product being edited already carries a
 * `low_stock_threshold` (the API can replace that value but never clear it
 * back to unset, `productsUpdate`'s own doc comment) — so this is a factory,
 * not a fixed schema constant. A brand-new product is never locked.
 */
export function productFormSchema(thresholdLocked: boolean) {
  return z
    .object({
      name: z.string().trim().min(1, t.validation.nameRequired).max(120, t.validation.maxChars120),
      category: z.string().max(60, t.validation.maxChars60).optional().or(z.literal('')),
      price: priceSchema,
      tracks_stock: z.boolean(),
      // Same NaN-from-a-blank-input concern as `priceSchema` above, but this
      // field is optional rather than required: `QuantityField` already turns
      // a cleared input into `undefined` on its own change handler, but this
      // is the defensive backstop at the schema boundary — without it, a
      // stray NaN reaching `z.number()` fails with Zod's own English
      // "invalid_type" default instead of either Spanish outcome below.
      // `z.union([z.number(), z.nan()])` (not `z.preprocess`) so the field
      // keeps a concrete `number | undefined` type on both sides of the
      // pipe — a `preprocess` widens its input to `unknown`, which breaks
      // `zodResolver`'s type against `ProductFormDto` under
      // `exactOptionalPropertyTypes`. Cleared and unset are the same case: no
      // threshold. Cleared while `thresholdLocked` (the `.refine` below)
      // still reports the Spanish "no se puede borrar" error, since the
      // value is `undefined` either way.
      low_stock_threshold: z
        .union([z.number(), z.nan()])
        .optional()
        .transform((value) => (value === undefined || Number.isNaN(value) ? undefined : value))
        .pipe(z.number().min(0, t.validation.amountNonNegative).int(t.validation.amountMustBeWhole).optional()),
    })
    .refine((data) => !thresholdLocked || data.low_stock_threshold !== undefined, {
      message: t.products.thresholdRequiredOnceSet,
      path: ['low_stock_threshold'],
    });
}

export type ProductFormDto = z.infer<ReturnType<typeof productFormSchema>>;

export const restockSchema = z.object({
  quantity: z
    .number({ message: t.validation.quantityRequired })
    .positive(t.validation.quantityPositive)
    .max(MAX_STOCK_QUANTITY, t.validation.quantityTooLarge)
    .int(t.validation.amountMustBeWhole),
  total_cost: z
    .number({ message: t.validation.amountRequired })
    .positive(t.validation.amountPositive)
    .max(MAX_RESTOCK_COST_PESOS, t.validation.movementAmountTooLarge)
    .int(t.validation.amountMustBeWhole),
  method: z.enum(COUNTER_PAYMENT_METHODS, { message: t.validation.selectPaymentMethod }),
  note: optionalNoteSchema,
});

export type RestockDto = z.infer<typeof restockSchema>;

/**
 * The owner types what they counted, not the signed delta the API wants —
 * `AdjustDialog` computes `quantity` (`counted - currentStock`) itself and
 * disables submit at 0, since a real zero-difference is not a valid
 * adjustment (the API's own `quantity` is "signed, non-zero").
 *
 * A factory, not a fixed schema constant, the same reason as
 * `productFormSchema`: capping `counted` on its own can't keep the signed
 * difference the API actually receives (`counted - currentStock`) inside its
 * own ±{@link MAX_STOCK_QUANTITY} range without knowing `currentStock`.
 */
export function adjustSchema(currentStock: number) {
  return z
    .object({
      counted: z
        .number({ message: t.validation.quantityRequired })
        .min(0, t.validation.quantityNonNegative)
        .int(t.validation.countedMustBeWhole),
      reason: z.enum(ADJUSTMENT_REASONS, { message: t.validation.selectReason }),
      note: optionalNoteSchema,
    })
    .refine((data) => Math.abs(data.counted - currentStock) <= MAX_STOCK_QUANTITY, {
      message: t.validation.quantityTooLarge,
      path: ['counted'],
    });
}

export type AdjustDto = z.infer<ReturnType<typeof adjustSchema>>;
