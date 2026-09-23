import { Banknote } from 'lucide-react';
import { Input } from '@/shared/components/ui/input';
import { FormField } from './FormField';

interface MoneyPesosFieldProps {
  id: string;
  label: string;
  value: number | undefined;
  onChange: (pesos: number | undefined) => void;
  error?: string | undefined;
  placeholder?: string | undefined;
}

/**
 * A pesos money input, shared by every cashbox form (open/close/movement) and
 * the product catalog's own forms (price, restock total cost).
 *
 * Moved here from `features/cash/components/` (pos-products-screen T5a):
 * `features/products` had duplicated this verbatim on the "features never
 * import from one another" reasoning, but the repo's actual rule for that
 * case is to move the shared piece to `shared/` instead — same move as
 * `shared/lib/paymentMethods.ts` and `shared/hooks/useCashSession.ts`. Two
 * copies of a money input drift; caps and labels stay feature-specific and
 * are passed in by the caller (`../schemas/*.schema`), not hardcoded here.
 *
 * Controlled via `value`/`onChange` (react-hook-form's `setValue`), not
 * `register` — same shape as `DepositAmountInput`/`ManualPriceField` in
 * `features/bookings`: a bare number input round-trips a blank field as `''`
 * under `register`'s default coercion, while `valueAsNumber` here reports it
 * as `NaN`, turned into `undefined` so a Zod schema reports "ingresá un
 * monto" instead of a value silently reset to 0.
 *
 * `h-12` (taller than the shared `Input`'s default `h-10`) and `text-base`:
 * the counter workflow this feature exists for is used on a phone, one-handed,
 * mid-transaction — a bigger touch target and a bigger digit size than the
 * desktop-first forms elsewhere in the app.
 *
 * Whole pesos only, no `step` attribute — same convention as
 * `DepositAmountInput`/`ManualPriceField`, which take no decimals either.
 * `step="0.01"` was tried first (to allow centavos) and had to be dropped: a
 * browser's native number-input step validation compares the typed value
 * against `step` in floating point, and `0.01` cannot be represented
 * exactly in IEEE754 — an ordinary value like `1500.5` failed that check as a
 * false "step mismatch". Omitting `step` falls back to its default of `1`,
 * which every typed integer satisfies, but a genuinely fractional amount
 * (`1500.5`) is still a real step mismatch under that default — and every
 * form using this field sets `noValidate` precisely so that never silently
 * blocks the `<form>` submit before React ever runs: the caller's own Zod
 * schema rejects a non-integer amount itself, which is the message the
 * person actually sees.
 */
export function MoneyPesosField({ id, label, value, onChange, error, placeholder }: MoneyPesosFieldProps) {
  return (
    <FormField label={label} htmlFor={id} error={error}>
      <div className="relative">
        <Banknote className="text-text-tertiary absolute top-1/2 left-3 size-4 -translate-y-1/2" aria-hidden="true" />
        <Input
          id={id}
          type="number"
          inputMode="numeric"
          min={0}
          className="h-12 pl-9 text-base"
          // `NaN` (an untouched required field, see a schema's `Number.NaN`
          // default) renders as the literal string "NaN" if handed straight
          // to a controlled input's `value` — guarded the same as the
          // `undefined`/absent case.
          value={value === undefined || Number.isNaN(value) ? '' : value}
          placeholder={placeholder}
          onChange={(e) => {
            const raw = e.target.valueAsNumber;
            onChange(Number.isNaN(raw) ? undefined : raw);
          }}
        />
      </div>
    </FormField>
  );
}
