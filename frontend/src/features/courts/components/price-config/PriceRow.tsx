import type { ReactNode } from 'react';
import { useId } from 'react';
import { MoneyInput } from '@/shared/components/common/MoneyInput';
import { cn } from '@/shared/lib/utils';

/**
 * One line of the rate table: a label on the left, a price field on the right.
 *
 * Label beside the field rather than above it, which a form of heterogeneous
 * questions should not do — there the eye zig-zags and the gap between each
 * label and its field changes with the label's length. Neither applies here.
 * This is a table of one repeated question, and the fields are pinned to the
 * right edge of their column, so they line up perfectly however long the day's
 * name is. That is what buys back half the height of seven rows.
 */
export function PriceRow({
  label,
  shortLabel,
  children,
}: {
  label: string;
  /** Shown below `sm`, where the full weekday name does not fit the column. */
  shortLabel?: string;
  children: ReactNode;
}) {
  return (
    <div className="hover:bg-bg-highlight/40 flex items-center justify-between gap-3 rounded-lg px-2 py-1.5 transition-colors">
      <div className="min-w-0">
        <span className="text-text-secondary text-xs font-medium">
          {shortLabel === undefined ? (
            label
          ) : (
            <>
              <span className="sm:hidden">{shortLabel}</span>
              <span className="hidden sm:inline">{label}</span>
            </>
          )}
        </span>
      </div>
      {children}
    </div>
  );
}

/**
 * The price field itself: the shared `MoneyInput` (same icon and height as every money field), and a width sized for the five or six
 * digits that go in it rather than for whatever the row has left over.
 *
 * `error`, when present, renders below the field rather than above it —
 * this is a dense seven-row table, not a single-field form, so FormField's
 * label-above-input chrome (`src/shared/components/common/FormField.tsx`,
 * which puts the message above for its own good reason) does not fit here.
 * `role="alert"` still gets the message to a screen reader.
 *
 * It renders IN FLOW, not `position: absolute` — a row whose error used to
 * float over the row below it without pushing anything down is the bug this
 * replaced (an absolutely positioned message takes no space, so nothing
 * after it moves). The `$`/input pair lives in its own inner `relative`
 * wrapper now specifically so the error can sit in normal flow in the OUTER
 * one below it: `$` still centers on the input's own height (its `top-1/2`
 * resolves against that inner wrapper, not one that has grown to include the
 * error text). The message wraps within the field's own width rather than
 * running on one `nowrap` line past it — it belongs to this field alone, not
 * the row, so it stays exactly as wide as the field does.
 *
 * Width is fixed, not fluid, because `MoneyInput`'s icon and padding leave only
 * about 52px of the box unusable for digits: "1.500.000" needs ~65px of text.
 * `w-32` (128px) for the day-level field; `compact`, set only by `BandRow`,
 * uses `w-[7.5rem]` (120px) below `sm` so the two time selects keep the ~60px
 * they need on a 320px line, and `sm:w-32` with `sm:ml-auto` above, so the
 * row's spare width collects as a gap before it, next to the delete button.
 */
export function PriceField({
  error,
  compact,
  value,
  onChange,
  placeholder,
  'aria-label': ariaLabel,
}: {
  value: number | undefined;
  onChange: (pesos: number | undefined) => void;
  placeholder?: string;
  'aria-label'?: string;
  error?: string | undefined;
  compact?: boolean;
}) {
  const errorId = useId();
  return (
    <div className={cn(compact ? 'w-[7.5rem] shrink-0 sm:ml-auto sm:w-32' : 'w-32 shrink-0')}>
      <MoneyInput
        value={value}
        onChange={onChange}
        placeholder={placeholder}
        aria-label={ariaLabel}
        aria-invalid={!!error}
        aria-describedby={error ? errorId : undefined}
      />
      {error && (
        <p id={errorId} role="alert" className="text-error-text mt-0.5 text-[10px] break-words">
          {error}
        </p>
      )}
    </div>
  );
}
