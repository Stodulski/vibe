import { forwardRef } from 'react';
import { format } from 'date-fns/format';
import { isSameDay } from 'date-fns/isSameDay';
import { es } from 'date-fns/locale/es';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';
import type { Schedule } from '@/shared/types/api.types';
import { isClosedDay } from './dayUtils';

const t = ES_AR;

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

  return (
    <button
      ref={ref}
      key={date.toISOString()}
      role="radio"
      aria-checked={isSelected}
      // Roving tabindex: only the selected date sits in the Tab order.
      tabIndex={isSelected ? 0 : -1}
      onClick={() => {
        if (!closed) onDateSelect(date);
      }}
      onKeyDown={onKeyDown}
      disabled={closed}
      className={cn(
        'flex shrink-0 flex-col items-center rounded-2xl border px-3.5 py-2.5 text-center transition-colors duration-200 min-w-[72px] press-scale',
        isSelected
          ? 'border-primary-500 bg-primary-500/10 text-primary-400 shadow-brand'
          : 'border-border-subtle bg-bg-subtle text-text-secondary hover:border-border-default hover:bg-bg-overlay',
        closed && 'cursor-not-allowed opacity-30 hover:border-border-subtle hover:bg-bg-subtle',
      )}
    >
      {isToday && (
        <span className="mb-0.5 text-xs font-bold uppercase tracking-wider text-primary-400">
          {t.publicBooking.today}
        </span>
      )}
      <span className="text-xs capitalize text-text-tertiary">{format(date, 'EEE', { locale: es })}</span>
      <span className={cn('text-lg font-bold', isSelected ? 'text-primary-400' : 'text-text-primary')}>
        {format(date, 'd')}
      </span>
      <span className="text-xs capitalize text-text-tertiary">{format(date, 'MMM', { locale: es })}</span>
      {closed && <span className="mt-0.5 text-xs font-medium text-error-text">{t.publicBooking.closed}</span>}
    </button>
  );
});
