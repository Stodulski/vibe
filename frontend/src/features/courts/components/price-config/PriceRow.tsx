import type { ReactNode } from 'react';
import { useId } from 'react';
import { Input } from '@/shared/components/ui/input';
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
 * The price field itself: fixed `$`, and a width sized for the five or six
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
 * `compact` narrows the padding, the `$` and the text size below `sm`,
 * restoring all three exactly at `sm:` and up — for `BandRow`, the one
 * caller that sets this prop; `PriceConfig`'s day-level field (which already
 * fits, unchanged) never does. The WIDTH half of `compact` is two different
 * shapes, not one fill rule at every size:
 *
 *   below `sm`   the box fills whatever share of the row `BandRow` gives it
 *                — `min-w-0 grow-[1.15] shrink basis-0`, capped at `max-w-36`
 *                (144px: a "999999"-sized number plus this field's own
 *                padding, with room to spare) so it cannot stretch past what
 *                a wide dialog would otherwise let it. `grow-[1.15]` rather
 *                than plain `flex-1`: the two time selects beside it claim
 *                `flex-1` each, and this one claims a slightly bigger share,
 *                because its number reads longer than either time does.
 *   `sm:` and up  fixed at `sm:w-24` — the same 96px the day-level field has
 *                always been — and `sm:ml-auto` instead of growing: the row's
 *                spare width collects as a gap BEFORE this field rather than
 *                stretching it, so it sits immediately left of the delete
 *                button rather than in the middle of the row.
 */
export function PriceField({
  className,
  error,
  compact,
  'aria-label': ariaLabel,
  ...props
}: React.ComponentProps<typeof Input> & { error?: string | undefined; compact?: boolean }) {
  const errorId = useId();
  return (
    <div
      className={cn(
        compact
          ? 'max-w-36 min-w-0 shrink grow-[1.15] basis-0 sm:ml-auto sm:w-24 sm:max-w-none sm:flex-none'
          : 'w-24 shrink-0',
      )}
    >
      <div className="relative">
        <span
          className={cn(
            'text-text-tertiary absolute top-1/2 left-2.5 -translate-y-1/2 text-xs font-medium',
            compact && 'left-1.5 sm:left-2.5',
          )}
        >
          $
        </span>
        <Input
          type="number"
          inputMode="decimal"
          min={0}
          step="any"
          aria-invalid={!!error}
          aria-label={ariaLabel}
          aria-describedby={error ? errorId : undefined}
          className={cn(
            // `h-10`, not the `h-8` this used to be: that was a deliberate
            // shrink for a denser row, and it is exactly what made this
            // field visibly shorter than `TimeSelect` beside it — `h-10` is
            // TimeSelect's own height AND the shared `Input` component's own
            // default (see `ui/input.tsx`), so this now matches by using the
            // same shared value rather than a new one picked to fit.
            'h-10 pl-5 text-sm font-semibold',
            compact && 'pr-1 pl-4 text-xs sm:pr-3.5 sm:pl-5 sm:text-sm',
            className,
          )}
          {...props}
        />
      </div>
      {error && (
        <p id={errorId} role="alert" className="text-error-text mt-0.5 text-[10px] break-words">
          {error}
        </p>
      )}
    </div>
  );
}
