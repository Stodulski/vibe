/**
 * Pesos <-> centavos conversion, shared by the cashbox and the product
 * catalog — every money-carrying API field is integer centavos, and every
 * form collects pesos (whole or with up to 2 decimal places — see the
 * `money-centavos` change: every money field now accepts centavos).
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
 * The formatting/parsing helpers below it are the other half of the
 * centavos-everywhere rule: they back `useMoneyInput`, the hook every money
 * field in the app (`MoneyPesosField` and the booking/court price fields)
 * uses to display es-AR thousands separators — and now up to 2 decimal
 * places, comma-separated — as the person types. Kept dependency-free from
 * React so the digit-extraction and caret-mapping rules are unit-testable on
 * their own.
 */

/** Pesos (possibly with cents) to integer centavos — same rounding as `cleanBookingPayload`. */
export function pesosToCentavos(pesos: number): number {
  return Math.round(pesos * 100);
}

/** Centavos to pesos, for seeding a form field from a value the API returned. */
export function centavosToPesos(centavos: number): number {
  return centavos / 100;
}

/**
 * True when `value` carries at most 2 decimal digits — the shared Zod
 * refinement every money-amount field applies (cash, products, bookings,
 * courts). `useMoneyInput` never lets a person TYPE a third decimal digit in
 * the first place (see `analyzeMoneyInput`'s `MAX_DECIMAL_DIGITS` cap), so
 * this exists as the schema-level backstop for any value that reaches a form
 * another way (a prefilled value from the API, a programmatic `setValue`).
 *
 * Compares against a small epsilon rather than `Number.isInteger(value*100)`
 * directly — floating point means `1500.1 * 100` is `150009.99999999999`,
 * not `150010`, so a strict integer check would reject a perfectly valid
 * amount.
 */
export function hasAtMostTwoDecimals(value: number): boolean {
  const scaled = value * 100;
  return Math.abs(scaled - Math.round(scaled)) < 1e-6;
}

/** Hard cap on how many integer digits a money field accepts, well past any real amount. */
export const MAX_MONEY_DIGITS = 12;

/** Hard cap on how many decimal digits a money field accepts — centavos, never more. */
const MAX_DECIMAL_DIGITS = 2;

/**
 * Extracts the digits that make up a typed or pasted amount that has NO
 * decimal comma at all.
 *
 * Every non-digit character (the '.' thousands separator this same field
 * renders, a leading "$", spaces) is stripped, and the result is capped at
 * `MAX_MONEY_DIGITS` digits. A raw string containing a comma is handled by
 * `analyzeMoneyInput` instead, which splits the integer and decimal parts
 * before stripping each separately — blindly stripping a comma here would
 * concatenate the two parts (a pasted "1.500,50" becoming "150050", a 100x
 * amount).
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

/**
 * A controlled field's numeric `value` prop, formatted for display:
 * es-AR-grouped pesos, plus a comma and exactly 2 decimal digits whenever the
 * amount actually carries centavos — "1500" -> "1.500", "1500.5" ->
 * "1.500,50". A whole amount never shows a decimal part at all.
 *
 * Rounds to the nearest centavo first (via integer centavos, not a decimal
 * string operation) so a value like `1500.1` — `150010` centavos exactly —
 * never renders as "1.500,09" from a stray floating-point remainder.
 */
export function formatMoneyValue(value: number | undefined): string {
  if (value === undefined || Number.isNaN(value)) return '';
  const centavos = Math.round(value * 100);
  const pesos = Math.trunc(centavos / 100);
  const cents = Math.abs(centavos % 100);
  const groupedPesos = formatMoneyDigits(String(Math.abs(pesos)));
  const sign = pesos < 0 || (pesos === 0 && centavos < 0) ? '-' : '';
  if (cents === 0) return `${sign}${groupedPesos}`;
  return `${sign}${groupedPesos},${String(cents).padStart(2, '0')}`;
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
 * Works the same way across a decimal comma and its digits: a comma is not a
 * digit character either, so it is skipped exactly like a thousands '.' is.
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

/**
 * What a raw money-field input means, split around its (possible) decimal
 * comma — `useMoneyInput` reads this on every keystroke to decide what to
 * display and what to report.
 *
 * - `'none'`    — no comma typed at all; the plain whole-pesos path applies
 *                 (see `extractMoneyDigits`).
 * - `'pending'` — a trailing comma with nothing after it yet ("1500,"). Kept
 *                 on screen (never silently dropped): dropping it while the
 *                 person is still typing is the 100x bug this type exists to
 *                 prevent — the very next digit would otherwise read as
 *                 another thousands digit of the integer part instead of the
 *                 first centavos digit ("1.500,50" typed one keystroke at a
 *                 time must never end up "150.050"). Resolved on blur.
 * - `'zero'`    — a comma followed only by zeros so far ("1500,0", "1500,00"):
 *                 an explicit, unambiguous "no centavos" ONCE THE PERSON IS
 *                 DONE TYPING — but NOT safe to collapse immediately while
 *                 they might still be typing (a further digit right after
 *                 would land as an extra thousands digit the same way).
 *                 Resolved on blur, same as `'pending'`.
 * - `'decimal'` — a comma followed by 1 or 2 non-all-zero digits: a real
 *                 centavos amount, accepted as-is (e.g. "1500,5" -> 1500.5).
 *                 A 3rd decimal digit is never captured here at all — see
 *                 `decimalDigits`'s own doc.
 */
export interface MoneyInputAnalysis {
  hasComma: boolean;
  /** Digits before the first comma (or every digit, when there is no comma), capped at `MAX_MONEY_DIGITS`. */
  integerDigits: string;
  /**
   * Digits after the first comma, capped at `MAX_DECIMAL_DIGITS` (2); `''`
   * when there is no comma. A 3rd (or later) typed decimal digit is dropped
   * here and never appears anywhere — not in the reported value, not in the
   * displayed string — rather than silently rounding or truncating a value
   * the person can still see and correct.
   */
  decimalDigits: string;
  kind: 'none' | 'pending' | 'zero' | 'decimal';
}

/** Classifies a raw money-field input around its (possible) decimal comma. See `MoneyInputAnalysis`. */
export function analyzeMoneyInput(raw: string): MoneyInputAnalysis {
  const commaIndex = raw.indexOf(',');
  if (commaIndex === -1) {
    return { hasComma: false, integerDigits: extractMoneyDigits(raw), decimalDigits: '', kind: 'none' };
  }
  const integerDigits = raw.slice(0, commaIndex).replace(/\D/g, '').slice(0, MAX_MONEY_DIGITS);
  const decimalDigits = raw
    .slice(commaIndex + 1)
    .replace(/\D/g, '')
    .slice(0, MAX_DECIMAL_DIGITS);
  if (decimalDigits === '') return { hasComma: true, integerDigits, decimalDigits, kind: 'pending' };
  if (/^0+$/.test(decimalDigits)) return { hasComma: true, integerDigits, decimalDigits, kind: 'zero' };
  return { hasComma: true, integerDigits, decimalDigits, kind: 'decimal' };
}

/**
 * The grouped display string an analysis produces, WHILE the field is still
 * being edited (comma present): the integer part grouped with '.', a comma,
 * and the (already capped-at-2) decimal digits exactly as typed —
 * "1500,5" -> "1.500,5", "1500," -> "1.500,", "1500,00" -> "1.500,00".
 */
export function composeMoneyDisplay(analysis: MoneyInputAnalysis): string {
  if (!analysis.hasComma) return formatMoneyDigits(analysis.integerDigits);
  return `${formatMoneyDigits(analysis.integerDigits)},${analysis.decimalDigits}`;
}

/**
 * The pesos number an analysis reports, whatever its `kind` — e.g.
 * `{ integerDigits: '1500', decimalDigits: '5' }` -> `1500.5`,
 * `{ integerDigits: '1500', decimalDigits: '' }` -> `1500`. Never truncated
 * or multiplied away: this is exactly what the person typed, in pesos, so a
 * caller's own `pesosToCentavos` (and its Zod rule) sees the real amount.
 * An analysis with no digits at all (a lone "," typed into an empty field)
 * reports `undefined`, the same "empty" signal `parseMoneyDigits('')` gives.
 */
export function fractionalMoneyValue(analysis: MoneyInputAnalysis): number | undefined {
  if (analysis.integerDigits === '' && analysis.decimalDigits === '') return undefined;
  const intPart = analysis.integerDigits === '' ? '0' : analysis.integerDigits;
  return analysis.decimalDigits === '' ? Number(intPart) : Number(`${intPart}.${analysis.decimalDigits}`);
}
