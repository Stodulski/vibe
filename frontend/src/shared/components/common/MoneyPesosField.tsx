import { FormField } from './FormField';
import { MoneyInput } from './MoneyInput';

interface MoneyPesosFieldProps {
  id: string;
  label: string;
  value: number | undefined;
  onChange: (pesos: number | undefined) => void;
  error?: string | undefined;
  placeholder?: string | undefined;
}

/**
 * A labelled pesos money field (FormField + `MoneyInput`), shared by every cashbox form (open/close/movement) and
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
 * `type="text"`, not `type="number"` — `useMoneyInput` formats the amount
 * with es-AR thousands separators as the person types ("150000" renders
 * "150.000"), which a number input cannot render at all. This is also what
 * dropped the `step` concern this comment used to document: a native
 * number-input's step validation used to compare a typed value against
 * `step` in floating point, and `0.01` (tried first, to allow centavos)
 * cannot be represented exactly in IEEE754 — an ordinary value like `1500.5`
 * failed that check as a false "step mismatch". A `type="text"` input has no
 * step at all; a typed/pasted decimal comma ("1500,50") is caught by
 * `useMoneyInput` itself, which shows it back grouped ("1.500,50") and
 * reports the real fractional pesos number — centavos are an accepted amount
 * everywhere in the app now (money-centavos change), converted with
 * `pesosToCentavos` by the caller, and the schema's own "hasta 2 decimales"
 * rule is only the backstop for a value that reaches the form another way.
 * Every form using this field still sets `noValidate` for the same reason as
 * always: the caller's own Zod schema is what reports a validation problem,
 * never a browser-native block before React runs.
 */
export function MoneyPesosField({ id, label, value, onChange, error, placeholder }: MoneyPesosFieldProps) {
  return (
    <FormField label={label} htmlFor={id} error={error}>
      <MoneyInput id={id} value={value} onChange={onChange} placeholder={placeholder} />
    </FormField>
  );
}
