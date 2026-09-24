/**
 * Pesos <-> centavos conversion, shared by the cashbox and the product
 * catalog — every money-carrying API field is integer centavos, and every
 * form collects whole pesos.
 *
 * Moved here from `features/cash/lib/money.ts` (pos-products-screen T5a):
 * `features/products` had duplicated these two functions verbatim in its own
 * `lib/money.ts` on the "features never import from one another" reasoning —
 * the repo's rule for that case is to move the shared piece to `shared/`
 * instead, same move as `shared/lib/paymentMethods.ts`. Each feature's own
 * `lib/money.ts` keeps its feature-specific caps (session cash / movement
 * amount for cash; product price / restock cost / stock quantity for
 * products), which are not shared.
 *
 * The formatting/parsing helpers below it are the other half of "every form
 * collects whole pesos": they back `useMoneyInput`, the hook every money
 * field in the app (`MoneyPesosField` and the booking/court price fields)
 * uses to display es-AR thousands separators as the person types. Kept
 * dependency-free from React so the digit-extraction and caret-mapping rules
 * are unit-testable on their own. There is no decimal point anywhere below
 * on purpose — whole pesos only, same as the conversion pair above.
 */

/** Pesos (possibly with cents) to integer centavos — same rounding as `cleanBookingPayload`. */
export function pesosToCentavos(pesos: number): number {
  return Math.round(pesos * 100);
}

/** Centavos to pesos, for seeding a form field from a value the API returned. */
export function centavosToPesos(centavos: number): number {
  return centavos / 100;
}

/** Hard cap on how many digits a money field accepts, well past any real amount. */
export const MAX_MONEY_DIGITS = 12;

/**
 * Extracts the digits that make up a typed or pasted amount.
 *
 * Everything from the first ',' onward is dropped BEFORE stripping other
 * characters — a pasted "1.500,50" (thousands '.', decimal ',') must become
 * "1500", never "150050" from blindly stripping the comma too. Every other
 * non-digit character (the '.' thousands separator this same field renders,
 * a leading "$", spaces) is then stripped, and the result is capped at
 * `MAX_MONEY_DIGITS` digits.
 */
export function extractMoneyDigits(raw: string): string {
  const commaIndex = raw.indexOf(',');
  const kept = commaIndex === -1 ? raw : raw.slice(0, commaIndex);
  return kept.replace(/\D/g, '').slice(0, MAX_MONEY_DIGITS);
}

/** A digit string ('' for an untouched/cleared field) as the pesos number the field reports. */
export function parseMoneyDigits(digits: string): number | undefined {
  return digits === '' ? undefined : Number.parseInt(digits, 10);
}

/** A digit string, grouped with es-AR thousands separators ('.'): "150000" -> "150.000". */
export function formatMoneyDigits(digits: string): string {
  return digits === '' ? '' : Number.parseInt(digits, 10).toLocaleString('es-AR');
}

/** A controlled field's numeric `value` prop, formatted for display. */
export function formatMoneyValue(value: number | undefined): string {
  if (value === undefined || Number.isNaN(value)) return '';
  return formatMoneyDigits(String(Math.trunc(value)));
}

/** How many `0`-`9` characters appear in `str`. */
export function countDigits(str: string): number {
  let count = 0;
  for (const ch of str) {
    if (ch >= '0' && ch <= '9') count++;
  }
  return count;
}

/**
 * Where the caret belongs in `formatted` so it sits right after the
 * `digitCount`-th digit — the anchor a reformat (inserting/removing a
 * thousands '.' as a digit crosses a grouping boundary) has to preserve so
 * editing in the middle of "150.000" does not jump the caret to the end.
 */
export function caretPositionForDigitCount(formatted: string, digitCount: number): number {
  if (digitCount <= 0) return 0;
  let seen = 0;
  for (let i = 0; i < formatted.length; i++) {
    const ch = formatted[i];
    if (ch !== undefined && ch >= '0' && ch <= '9') {
      seen++;
      if (seen >= digitCount) return i + 1;
    }
  }
  return formatted.length;
}
