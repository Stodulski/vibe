import { useLayoutEffect, useRef } from 'react';
import {
  caretPositionForDigitCount,
  countDigits,
  extractMoneyDigits,
  formatMoneyValue,
  parseMoneyDigits,
} from '@/shared/lib/money';

interface UseMoneyInputArgs {
  value: number | undefined;
  onChange: (pesos: number | undefined) => void;
}

interface UseMoneyInputResult {
  /** The value to hand a `type="text"` input's `value` prop: "150000" -> "150.000". */
  displayValue: string;
  /** Attach to the input so the caret can be repositioned after a reformat. */
  inputRef: React.RefObject<HTMLInputElement | null>;
  /** The input's `onChange` — parses what was typed/pasted and calls the caller's `onChange`. */
  handleChange: (event: React.ChangeEvent<HTMLInputElement>) => void;
}

/**
 * The formatting/parsing behind every money input in the app — a plain
 * `value`/`onChange` pesos API (unchanged from before this hook existed) over
 * a `type="text"` input that displays es-AR thousands separators as the
 * person types: "150000" renders "150.000".
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
 * (not characters — the '.' separators do not count) preceded the caret
 * right after the keystroke lands, stores that count, then — once the
 * reformatted `displayValue` has actually reached the DOM — places the caret
 * after that same count of digits in the NEW string. Separators inserted or
 * removed around the edit point never move where the person is actually
 * looking.
 *
 * Parsing (`extractMoneyDigits`) drops everything from the first ',' onward
 * before stripping non-digits, so a pasted "1.500,50" becomes "1500", never
 * "150050" from treating the decimal comma as just more digits — this app's
 * money fields are whole pesos only, the same convention `MoneyPesosField`
 * documents for the native `type="number"` inputs this hook replaces.
 */
export function useMoneyInput({ value, onChange }: UseMoneyInputArgs): UseMoneyInputResult {
  const inputRef = useRef<HTMLInputElement>(null);
  const pendingCaretDigits = useRef<number | null>(null);
  const displayValue = formatMoneyValue(value);

  useLayoutEffect(() => {
    const digitsBeforeCaret = pendingCaretDigits.current;
    if (digitsBeforeCaret === null) return;
    pendingCaretDigits.current = null;
    const input = inputRef.current;
    if (!input) return;
    const pos = caretPositionForDigitCount(displayValue, digitsBeforeCaret);
    input.setSelectionRange(pos, pos);
  }, [displayValue]);

  const handleChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const raw = event.target.value;
    const caretPos = event.target.selectionStart ?? raw.length;
    const commaIndex = raw.indexOf(',');
    const kept = commaIndex === -1 ? raw : raw.slice(0, commaIndex);
    const clampedCaret = Math.min(caretPos, kept.length);
    const digits = extractMoneyDigits(raw);
    const digitsBeforeCaret = Math.min(countDigits(kept.slice(0, clampedCaret)), digits.length);

    pendingCaretDigits.current = digitsBeforeCaret;
    onChange(parseMoneyDigits(digits));
  };

  return { displayValue, inputRef, handleChange };
}
