import { Banknote } from 'lucide-react';
import { Input } from '@/shared/components/ui/input';
import { useMoneyInput } from '@/shared/hooks/useMoneyInput';
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
 * under `register`'s default coercion, while this field's own parsing (see
 * `useMoneyInput`) reports a blank field as `undefined` so a Zod schema
 * reports "ingresá un monto" instead of a value silently reset to 0.
 *
 * `h-12` (taller than the shared `Input`'s default `h-10`) and `text-base`:
 * the counter workflow this feature exists for is used on a phone, one-handed,
 * mid-transaction — a bigger touch target and a bigger digit size than the
 * desktop-first forms elsewhere in the app.
 *
 * `type="text"`, not `type="number"` — `useMoneyInput` formats the amount
 * with es-AR thousands separators as the person types ("150000" renders
 * "150.000"), which a number input cannot render at all. This is also what
 * dropped the `step` concern this comment used to document: a native
 * number-input's step validation used to compare a typed value against
 * `step` in floating point, and `0.01` (tried first, to allow centavos)
 * cannot be represented exactly in IEEE754 — an ordinary value like `1500.5`
 * failed that check as a false "step mismatch". A `type="text"` input has no
 * step at all, and `useMoneyInput`'s own digit-only parsing makes a decimal
 * amount unreachable by typing or pasting in the first place — whole pesos
 * only, same convention as `DepositAmountInput`/`ManualPriceField`. Every
 * form using this field still sets `noValidate` for the same reason as
 * always: the caller's own Zod schema is what reports a validation problem,
 * never a browser-native block before React runs.
 */
export function MoneyPesosField({ id, label, value, onChange, error, placeholder }: MoneyPesosFieldProps) {
  const { displayValue, inputRef, handleChange } = useMoneyInput({ value, onChange });

  return (
    <FormField label={label} htmlFor={id} error={error}>
      <div className="relative">
        <Banknote className="text-text-tertiary absolute top-1/2 left-3 size-4 -translate-y-1/2" aria-hidden="true" />
        <Input
          id={id}
          type="text"
          inputMode="numeric"
          autoComplete="off"
          className="h-12 pl-9 text-base"
          ref={inputRef}
          value={displayValue}
          placeholder={placeholder}
          onChange={handleChange}
        />
      </div>
    </FormField>
  );
}
