import { forwardRef } from 'react';
import { format } from 'date-fns/format';
import { isSameDay } from 'date-fns/isSameDay';
import { es } from 'date-fns/locale/es';
import { BOOKING_MESSAGES } from '../../messages';
import { cn } from '@vibe/ui';
import type { Schedule } from '../../api/types';
import { isClosedDay } from './dayUtils';

const t = BOOKING_MESSAGES;

interface DateButtonProps {
  date: Date;
  today: Date;
  selectedDate: Date;
  schedules: Schedule[];
  onDateSelect: (date: Date) => void;
  onKeyDown?: (event: React.KeyboardEvent<HTMLButtonElement>) => void;
}

export const DateButton = forwardRef<HTMLButtonElement, DateButtonProps>(function DateButton(
  { date, today, selectedDate, schedules, onDateSelect, onKeyDown },
  ref,
) {
  const isToday = isSameDay(date, today);
  const isSelected = isSameDay(date, selectedDate);
  const closed = isClosedDay(date, schedules);
  // The selected fill is solid brand green, so the labels take the inverse ink at 80%
  // (about 5.7:1 on #1db954). Unselected days keep the quieter grey.
  const labelTone = isSelected ? 'text-text-inverse' : 'text-text-tertiary';

  return (
    <button
      ref={ref}
      key={date.toISOString()}
      role="radio"
      aria-checked={isSelected}
      aria-current={isToday ? 'date' : undefined}
      // Roving tabindex: only the selected date sits in the Tab order.
      tabIndex={isSelected ? 0 : -1}
      onClick={() => {
        if (!closed) onDateSelect(date);
      }}
      onKeyDown={onKeyDown}
      disabled={closed}
      className={cn(
        // Idle gets a real edge (`border-border-interactive`, ~3:1 against
        // every surface tier — see `scripts/contrast-report.mjs`) instead of
        // the old `border-border-subtle` decorative divider, and selected
        // gets a stronger fill than idle so the two states don't collapse
        // into one another (odd/tasks/app-dark-contrast.md T4).
        //
        // `opacity-self`: opts out of the global `:disabled` 50% opacity
        // (globals.css) — closed already reads as unavailable through color
        // alone, and that opacity would wash the "Cerrado" label back out.
        'opacity-self press-scale relative flex min-w-[72px] shrink-0 flex-col items-center rounded-2xl border px-3.5 py-2.5 text-center transition-colors duration-200',
        isSelected
          ? 'border-primary-500 bg-primary-500 text-text-inverse shadow-brand'
          : closed
            ? // No `opacity-30` on the whole button — that also washed out
              // the "Cerrado" text itself down to ~1.6:1. A closed day reads
              // as unavailable through real, still-legible color: a quiet
              // edge, no fill, dimmed (not faded) numerals.
              'border-border-subtle text-text-tertiary cursor-not-allowed bg-transparent'
            : 'border-border-interactive bg-bg-subtle text-text-secondary hover:border-border-interactive-hover hover:bg-bg-highlight',
      )}
    >
      {/* Today is a dot in the top corner, out of the flow so every day keeps
          the same height. Screen readers get `aria-current="date"` instead. */}
      {isToday && (
        <span
          aria-hidden="true"
          className={cn(
            'absolute top-3 right-3 size-1.5 rounded-full',
            isSelected ? 'bg-text-inverse' : 'bg-primary-400',
          )}
        />
      )}
      <span className={cn(labelTone, 'text-xs capitalize')}>{format(date, 'EEE', { locale: es })}</span>
      <span
        className={cn(
          'text-lg font-bold',
          isSelected ? 'text-text-inverse' : closed ? 'text-text-tertiary' : 'text-text-primary',
        )}
      >
        {format(date, 'd')}
      </span>
      <span className={cn(labelTone, 'text-xs capitalize')}>{format(date, 'MMM', { locale: es })}</span>
      {closed && <span className="text-error-text mt-0.5 text-xs font-medium">{t.publicBooking.closed}</span>}
    </button>
  );
});
