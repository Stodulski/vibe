import { ChevronDown } from 'lucide-react';
import { cn } from '@/shared/lib/utils';

function generateTimeOptions(): string[] {
  const options: string[] = [];
  for (let h = 0; h < 24; h++) {
    for (let m = 0; m < 60; m += 30) {
      options.push(`${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}`);
    }
  }
  return options;
}

const TIME_OPTIONS = generateTimeOptions();

/**
 * One end of a time range: an opening hour, a closing hour, a price band's edge.
 *
 * It lives in `shared/` rather than beside the schedule form that first needed
 * it because the court price editor asks the same question — "which half-hour?"
 * — and two controls for one question is how an app ends up rendering "08:00"
 * on one tab and "8:00 AM" on the next.
 *
 * A native `<select>`, wearing the app's input styles. The schedule screen tried
 * the three alternatives and each lost something the browser gives away:
 *
 * - The app's Radix Select matched visually, but fourteen of them mount on
 *   this tab — seven days, open and close — and entering it froze for about a
 *   second. The browser renders 48 options for nothing; Radix builds fourteen
 *   roots, contexts and portals.
 * - `<input type="time">` cost nothing, but shows 12- or 24-hour according to
 *   the VIEWER's system locale, which no attribute or stylesheet overrides. An
 *   es-AR app rendering "08:00 AM" is wrong, and wrong differently per machine.
 * - Two hand-rolled digit fields fixed both, then had to re-implement snapping,
 *   carrying and keyboard stepping — behaviour a list of valid values never has
 *   to get right, because no invalid value exists in it.
 *
 * So: native, styled rather than replaced. `appearance-none` drops the system
 * arrow and the classes are the ones `Input` uses, so it sits among the app's
 * controls without being one of its components. 40px tall, where the original
 * was 32 and under the minimum target size.
 *
 * The open panel is still the operating system's. That is the trade, and it is
 * the cheap half: the closed control is what is on screen all the time.
 */
export function TimeSelect({
  value,
  onChange,
  disabled,
  className,
  compact,
  'aria-label': ariaLabel,
}: {
  value: string;
  onChange: (v: string) => void;
  disabled?: boolean;
  /** Overrides the default width pair; see the comment on the wrapper below. */
  className?: string;
  /**
   * Tighter padding, smaller text and a smaller chevron below `sm`, restoring
   * the exact default chrome at `sm:` and up — opt-in, so the schedule tab's
   * own selects (one per row, room to spare) render exactly as before. Only
   * the price editor's differentiated-row select, packed onto one line with
   * another select, a price field and a delete button down to 320px, sets it.
   */
  compact?: boolean;
  'aria-label'?: string;
}) {
  return (
    // Fills the line on a phone, fixed from sm. The row has the width to give
    // there and nothing else to spend it on, and two boxes stretched to the
    // edge read as the pair they are; at 108px they sat in the left third with
    // the rest of the row blank.
    //
    // `className` overrides that pair of widths where the row is tighter than a
    // schedule row — the price editor's bands sit inside a dialog column that
    // also carries a price field and a delete button, and cannot give a select
    // the whole line.
    <div className={cn('relative min-w-0 flex-1 sm:w-[6.75rem] sm:flex-none', className)}>
      <select
        value={value}
        onChange={(e) => {
          onChange(e.target.value);
        }}
        disabled={disabled}
        aria-label={ariaLabel}
        className={cn(
          'border-border-interactive bg-bg-base/60 h-10 w-full appearance-none rounded-xl border',
          'text-text-primary transition-input pr-8 pl-3.5 text-sm tabular-nums shadow-xs outline-none',
          'hover:border-border-interactive-hover hover:bg-bg-base/80',
          'focus-visible:border-primary-500/60 focus-visible:bg-bg-base focus-visible:ring-primary-500/15 focus-visible:ring-[3px]',
          'disabled:cursor-not-allowed disabled:opacity-50',
          compact && 'pr-5 pl-2 text-xs sm:pr-8 sm:pl-3.5 sm:text-sm',
        )}
      >
        {TIME_OPTIONS.map((time) => (
          <option key={time} value={time}>
            {time}
          </option>
        ))}
      </select>
      <ChevronDown
        aria-hidden="true"
        className={cn(
          'text-text-tertiary pointer-events-none absolute top-1/2 right-3 size-4 -translate-y-1/2',
          compact && 'right-1.5 size-3 sm:right-3 sm:size-4',
        )}
      />
    </div>
  );
}
