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
    <div className="flex items-center justify-between gap-3 rounded-lg px-2 py-1.5 transition-colors hover:bg-bg-highlight/40">
      <div className="min-w-0">
        <span className="text-xs font-medium text-text-secondary">
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
 * `error`, when present, renders below the field rather than pushing the
 * whole row's label out of line — this is a dense seven-row table, not a
 * single-field form, so FormField's label-above-input chrome (`src/shared/
 * components/common/FormField.tsx`) does not fit here. `role="alert"` still
 * gets the message to a screen reader without the extra layout.
 */
export function PriceField({
  className,
  error,
  'aria-label': ariaLabel,
  ...props
}: React.ComponentProps<typeof Input> & { error?: string | undefined }) {
  const errorId = useId();
  return (
    <div className="relative w-24 shrink-0">
      <span className="absolute left-2.5 top-1/2 -translate-y-1/2 text-xs font-medium text-text-tertiary">$</span>
      <Input
        type="number"
        inputMode="decimal"
        min={0}
        step="any"
        aria-invalid={!!error}
        aria-label={ariaLabel}
        aria-describedby={error ? errorId : undefined}
        className={cn('h-8 pl-5 text-sm font-semibold', className)}
        {...props}
      />
      {error && (
        <p
          id={errorId}
          role="alert"
          className="absolute top-full right-0 mt-0.5 whitespace-nowrap text-[10px] text-error-text"
        >
          {error}
        </p>
      )}
    </div>
  );
}
