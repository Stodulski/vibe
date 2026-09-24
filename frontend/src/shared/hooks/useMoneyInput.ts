import { useLayoutEffect, useRef, useState } from 'react';
import {
  analyzeMoneyInput,
  caretPositionForDigitCount,
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
  /** The value to hand a `type="text"` input's `value` prop: "150000" -> "150.000". */
  displayValue: string;
  /** Attach to the input so the caret can be repositioned after a reformat. */
  inputRef: React.RefObject<HTMLInputElement | null>;
  /** The input's `onChange` — parses what was typed/pasted and calls the caller's `onChange`. */
  handleChange: (event: React.ChangeEvent<HTMLInputElement>) => void;
  /** The input's `onBlur` — resolves a still-pending trailing comma (see `analyzeMoneyInput`'s `'pending'`). */
  handleBlur: () => void;
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
 * A decimal comma (',') is the other hard part, and the one a real bug lived
 * in: this app's money fields are whole pesos only, but that does NOT mean a
 * typed ',' can simply be dropped. Live, per-keystroke typing of "1.500,50"
 * used to collapse to "1.500" the instant the ',' landed (reformatting drops
 * the comma from the DOM), and the very next digit ("5") then read as just
 * another thousands digit of the INTEGER part instead of a centavos digit —
 * "1.500,50" silently became 150050 pesos, a 100x amount, with no error
 * anywhere. `analyzeMoneyInput` is what prevents that: while a ',' is
 * followed by nothing yet ("pending") or only zeros ("zero"), the field
 * behaves as before; the moment it is followed by a real, non-zero decimal
 * digit ("invalid"), the field STOPS reformatting — it shows exactly what was
 * typed/pasted, verbatim, and reports the real fractional pesos number
 * (`fractionalMoneyValue`) instead of an integer, so the caller's own
 * whole-pesos Zod rule (an `.int()`, where the schema has one) rejects it
 * with a visible message. Wrong or corrupted, never silent.
 */
export function useMoneyInput({ value, onChange }: UseMoneyInputArgs): UseMoneyInputResult {
  const inputRef = useRef<HTMLInputElement>(null);
  const pendingCaretDigits = useRef<number | null>(null);
  // Non-null while the field shows something the numeric `value` prop cannot
  // represent as-is: a trailing "1500," with nothing after the comma yet, or
  // a genuinely fractional "1500,5" a whole-pesos schema is meant to refuse.
  // Reformatting either one immediately is exactly the bug this hook exists
  // to prevent (see the doc comment above) — this is what keeps the raw text
  // on screen instead.
  const [rawOverride, setRawOverride] = useState<string | null>(null);
  const displayValue = rawOverride ?? formatMoneyValue(value);

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
    const analysis = analyzeMoneyInput(raw);

    if (analysis.kind === 'none') {
      setRawOverride(null);
      const caretPos = event.target.selectionStart ?? raw.length;
      const digitsBeforeCaret = Math.min(countDigits(raw.slice(0, caretPos)), analysis.integerDigits.length);
      pendingCaretDigits.current = digitsBeforeCaret;
      onChange(parseMoneyDigits(analysis.integerDigits));
      return;
    }

    if (analysis.kind === 'pending') {
      // A lone trailing comma, nothing after it yet — left exactly as typed
      // (see this hook's own doc comment for why reformatting now would be
      // the bug). `handleBlur` resolves it once the person moves on.
      setRawOverride(raw);
      onChange(parseMoneyDigits(analysis.integerDigits));
      return;
    }

    if (analysis.kind === 'zero') {
      // An explicit, unambiguous "no centavos" (",0", ",00") — unlike
      // `'pending'` there is nothing left to wait for, so this collapses
      // immediately, same as the no-comma path.
      setRawOverride(null);
      pendingCaretDigits.current = analysis.integerDigits.length;
      onChange(parseMoneyDigits(analysis.integerDigits));
      return;
    }

    // 'invalid': a real fractional amount, typed or pasted. Shown verbatim,
    // reported as the real fractional number so a whole-pesos Zod rule can
    // reject it visibly instead of the amount being corrupted in silence.
    setRawOverride(raw);
    onChange(fractionalMoneyValue(analysis));
  };

  const handleBlur = () => {
    if (rawOverride === null) return;
    // Only a still-'pending' trailing comma resolves on blur — an 'invalid'
    // fractional amount stays exactly as typed so its validation error stays
    // visible until the person actually fixes it.
    if (analyzeMoneyInput(rawOverride).kind !== 'pending') return;
    setRawOverride(null);
  };

  return { displayValue, inputRef, handleChange, handleBlur };
}
