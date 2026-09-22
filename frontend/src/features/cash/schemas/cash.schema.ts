import { z } from 'zod';
import { ES_AR } from '@/shared/i18n/es_AR';
import { COUNTER_PAYMENT_METHODS } from '@/shared/lib/paymentMethods';
import { EXPENSE_CATEGORIES, INCOME_CATEGORIES, MOVEMENT_CATEGORIES } from '../lib/movementCategories';
import { MAX_SESSION_CASH_PESOS, MAX_MOVEMENT_AMOUNT_PESOS } from '../lib/money';

const t = ES_AR;

// `z.number({message})` (not `.positive()`) is what makes an empty/NaN input
// (react-hook-form's `valueAsNumber` on a blank field) report `amountRequired`
// instead of Zod's generic "invalid type" — see courts.schema.ts's `bandPrice`
// for the same NaN-from-an-empty-number-input concern, though 0 is a
// legitimate opening/counted amount here (an empty drawer) where it is not
// there, so this stays a plain `min(0)` rather than that field's `z.nan()`
// escape hatch.
const sessionCashSchema = z
  .number({ message: t.validation.amountRequired })
  .min(0, t.validation.amountNonNegative)
  .max(MAX_SESSION_CASH_PESOS, t.validation.cashSessionTooLarge)
  .int(t.validation.amountMustBeWhole);

const optionalNoteSchema = z.string().max(500, t.validation.maxChars500).optional().or(z.literal(''));

export const openCashSessionSchema = z.object({
  opening_cash: sessionCashSchema,
  note: optionalNoteSchema,
});

export const closeCashSessionSchema = z.object({
  counted_cash: sessionCashSchema,
  note: optionalNoteSchema,
});

export const cashMovementSchema = z
  .object({
    kind: z.enum(['income', 'expense']),
    category: z.enum(MOVEMENT_CATEGORIES, { message: t.validation.selectCategory }),
    method: z.enum(COUNTER_PAYMENT_METHODS, { message: t.validation.selectPaymentMethod }),
    amount: z
      .number({ message: t.validation.amountRequired })
      .positive(t.validation.amountPositive)
      .max(MAX_MOVEMENT_AMOUNT_PESOS, t.validation.movementAmountTooLarge)
      .int(t.validation.amountMustBeWhole),
    note: optionalNoteSchema,
  })
  // A category belongs to exactly one kind (see `categoriesFor`) — the select
  // only ever offers the matching set, but `kind` can flip (Ingreso/Egreso
  // toggle) without the category selection resetting, which would otherwise
  // let a stale "Insumos" ride along under "Ingreso".
  .refine(
    ({ kind, category }) =>
      kind === 'income'
        ? (INCOME_CATEGORIES as readonly string[]).includes(category)
        : (EXPENSE_CATEGORIES as readonly string[]).includes(category),
    { message: t.validation.selectCategory, path: ['category'] },
  );

export const voidCashMovementSchema = z.object({
  note: optionalNoteSchema,
});

export type OpenCashSessionDto = z.infer<typeof openCashSessionSchema>;
export type CloseCashSessionDto = z.infer<typeof closeCashSessionSchema>;
export type CashMovementDto = z.infer<typeof cashMovementSchema>;
// `VoidMovementDialog` keeps its note in plain `useState` (no form/resolver —
// a single optional field isn't worth react-hook-form's ceremony), so
// `voidCashMovementSchema` has no DTO consumer; not exported.
