import { useLayoutEffect, useRef, useState } from 'react';
import {
  analyzeMoneyInput,
  caretPositionForDigitCount,
  composeMoneyDisplay,
  countDigits,
  formatMoneyValue,
  fractionalMoneyValue,
  parseMoneyDigits,
} from '@/shared/lib/money';

interface UseMoneyInputArgs {
  value: number | undefined;
  onChange: (pesos: number | undefined) => void;
}

interface UseMoneyInputResult {
  /** The value to hand a `type="text"` input's `value` prop: "150000" -> "150.000", "1500.5" -> "1.500,50". */
  displayValue: string;
  /** Attach to the input so the caret can be repositioned after a reformat. */
  inputRef: React.RefObject<HTMLInputElement | null>;
  /** The input's `onChange` — parses what was typed/pasted and calls the caller's `onChange`. */
  handleChange: (event: React.ChangeEvent<HTMLInputElement>) => void;
  /** The input's `onBlur` — resolves a still-pending trailing comma, a "no centavos" zero decimal, or a single decimal digit that still needs its trailing zero (see `analyzeMoneyInput`). */
  handleBlur: () => void;
}

/**
 * The formatting/parsing behind every money input in the app — a plain
 * `value`/`onChange` pesos API over a `type="text"` input that displays
 * es-AR thousands separators, and up to 2 decimal digits after a comma, as
 * the person types: "150000" renders "150.000", "1500,5" renders "1.500,5".
 *
 * `type="text"`, not `type="number"`: a number input cannot render "150.000"
 * at all (a browser parses the '.' as a decimal point, if it accepts the
 * string as a number in the first place), which is the whole reason this
 * hook exists rather than a `step`/formatting trick on the native input.
 *
 * The caret is the hard part. Reformatting the string on every keystroke —
 * inserting or removing a '.' as a digit crosses a thousands boundary —
 * otherwise jumps a controlled input's caret to the end on every render,
 * which makes editing a digit in the middle of "150.000" impossible (typing
 * there always lands back after the last 0). The fix counts how many DIGITS
 * (not characters — the '.' separators and the decimal ',' do not count)
 * preceded the caret right after the keystroke lands, stores that count, then
 * — once the reformatted `displayValue` has actually reached the DOM —
 * places the caret after that same count of digits in the NEW string.
 * Separators inserted or removed around the edit point never move where the
 * person is actually looking.
 *
 * A decimal comma (',') is the other hard part, and the one two real bugs
 * used to live in (see the `money-centavos` change): typing "1.500,50" one
 * keystroke at a time used to reformat away the comma the instant it landed,
 * so the very next digit ("5") read as another thousands digit of the
 * INTEGER part instead of a centavos digit — "1.500,50" silently became
 * 150050 pesos, a 100x amount, with no error anywhere. The same thing then
 * happened one digit later for an EXPLICIT zero decimal: collapsing ",0" or
 * ",00" immediately (they read as unambiguous — "no centavos" — so it seemed
 * safe) still dropped the comma from the DOM, and a still-in-progress
 * "1500,00" typed one keystroke at a time silently became 15000 the moment
 * the second "0" landed.
 *
 * `analyzeMoneyInput` is what prevents both, and now that centavos are a
 * real, accepted amount (not an error) rather than a bug to avoid, the fix
 * generalizes into the ordinary live-formatting path: whenever the raw input
 * has a comma, `composeMoneyDisplay` builds the displayed string directly
 * from the split integer/decimal digits (grouped integer + ',' + up to 2
 * decimal digits) instead of using the raw DOM text verbatim — the comma
 * itself, and any zeros after it, are never dropped mid-edit. A 3rd decimal
 * digit is capped away by `analyzeMoneyInput` itself and never reaches the
 * display or the reported value; `input.value` is also corrected directly
 * (not only via the next React render) so a keystroke that types past the
 * cap never lingers in the DOM even when the computed string does not
 * otherwise change.
 *
 * `handleBlur` only has to clear the in-progress override: once cleared, the
 * display falls back to `formatMoneyValue(value)`, which already knows how
 * to render the exact reported number — an integer value (from a pending
 * comma, or an all-zero decimal) loses its comma entirely ("1500," / "1500,00"
 * -> "1.500"), and a real fractional value gets its trailing zero padded in
 * ("1500,5" -> value 1500.5 -> "1.500,50") — without `handleBlur` needing any
 * of that logic itself.
 */
export function useMoneyInput({ value, onChange }: UseMoneyInputArgs): UseMoneyInputResult {
  const inputRef = useRef<HTMLInputElement>(null);
  // The absolute caret position to restore in the NEXT rendered
  // `displayValue`, computed once in `handleChange` (see there for why a
  // decimal comma needs its own adjustment on top of the digit count).
  const pendingCaretPos = useRef<number | null>(null);
  // Non-null while the field shows something the numeric `value` prop alone
  // cannot represent as typed: a trailing "1500," with nothing after the
  // comma yet, an in-progress "1500,0"/"1500,00", or a decimal with only 1
  // digit so far ("1500,5", value 1500.5, not yet padded to "1500,50").
  // Reformatting any of these away immediately is exactly the bug this hook
  // exists to prevent (see the doc comment above) — this is what keeps the
  // composed text on screen instead.
  const [rawOverride, setRawOverride] = useState<string | null>(null);
  const displayValue = rawOverride ?? formatMoneyValue(value);

  useLayoutEffect(() => {
    const pos = pendingCaretPos.current;
    if (pos === null) return;
    pendingCaretPos.current = null;
    const input = inputRef.current;
    if (!input) return;
    input.setSelectionRange(pos, pos);
  }, [displayValue]);

  const handleChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const input = event.target;
    const raw = input.value;
    const analysis = analyzeMoneyInput(raw);
    const nextDisplay = composeMoneyDisplay(analysis);

    const caretPos = input.selectionStart ?? raw.length;
    const rawBeforeCaret = raw.slice(0, caretPos);
    const totalDigitsKept = analysis.integerDigits.length + analysis.decimalDigits.length;
    const digitsBeforeCaret = Math.min(countDigits(rawBeforeCaret), totalDigitsKept);
    let targetPos = caretPositionForDigitCount(nextDisplay, digitsBeforeCaret);
    // `caretPositionForDigitCount` only anchors on digits, which is not
    // enough the instant the comma itself is the last thing typed: with 0
    // decimal digits after it, the digit-count anchor lands right BEFORE the
    // comma (there is no (N+1)-th digit yet to anchor past it), so a caret
    // that in the raw text already sat after the comma would otherwise be
    // pulled back in front of it — the very next keystroke would then land
    // inside the INTEGER part instead of starting the decimal one (typing
    // "1500," then "5" would insert into "1500" itself, not read as a
    // centavos digit). Once the raw text the person is looking at has the
    // comma before their caret, the target position is pinned to at least
    // right after the comma in the new display too.
    if (analysis.hasComma && rawBeforeCaret.includes(',')) {
      const commaIndex = nextDisplay.indexOf(',');
      if (commaIndex !== -1) targetPos = Math.max(targetPos, commaIndex + 1);
    }
    pendingCaretPos.current = targetPos;

    // Corrects the DOM directly, not only via the next React render: when the
    // computed `nextDisplay` is identical to what was already there (e.g. a
    // keystroke that types a 3rd decimal digit past the cap, which
    // `analyzeMoneyInput` drops entirely), React sees an unchanged state
    // value and bails out of re-rendering, which would otherwise leave the
    // just-typed extra character sitting in the DOM even though the reported
    // value never moved.
    if (input.value !== nextDisplay) {
      input.value = nextDisplay;
      input.setSelectionRange(targetPos, targetPos);
    }

    if (!analysis.hasComma) {
      setRawOverride(null);
      onChange(parseMoneyDigits(analysis.integerDigits));
      return;
    }

    setRawOverride(nextDisplay);
    onChange(fractionalMoneyValue(analysis));
  };

  const handleBlur = () => {
    if (rawOverride === null) return;
    setRawOverride(null);
  };

  return { displayValue, inputRef, handleChange, handleBlur };
}
