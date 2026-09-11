import { useMemo, useState } from 'react';
import { format, addDays } from 'date-fns';
import { es } from 'date-fns/locale';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Schedule } from '@/shared/types/api.types';
import { useLoadMoreOnScroll } from './date-selector/useLoadMoreOnScroll';
import { useAutoScrollToSelectedDate } from './date-selector/useAutoScrollToSelectedDate';
import { useDateStripKeyboardNav } from './date-selector/useDateStripKeyboardNav';
import { DateButton } from './date-selector/DateButton';

const t = ES_AR;

interface DateSelectorProps {
  selectedDate: Date;
  onDateSelect: (date: Date) => void;
  schedules: Schedule[];
}

export function DateSelector({ selectedDate, onDateSelect, schedules }: DateSelectorProps) {
  const today = useMemo(() => new Date(), []);
  const [daysCount, setDaysCount] = useState(14);
  const days = useMemo(() => Array.from({ length: daysCount }, (_, i) => addDays(today, i)), [today, daysCount]);

  const scrollRef = useLoadMoreOnScroll(() => {
    setDaysCount((prev) => prev + 14);
  });
  useAutoScrollToSelectedDate(scrollRef, days, selectedDate);
  const { buttonRefs, handleKeyDown } = useDateStripKeyboardNav(days, schedules, onDateSelect);

  return (
    <div className="w-full">
      <h2 className="mb-4 text-sm font-semibold first-letter:uppercase text-text-primary">
        {format(selectedDate, "EEE d 'de' MMM, yyyy", { locale: es })}
      </h2>
      <div
        ref={scrollRef}
        role="radiogroup"
        aria-label={t.publicBooking.dateStripLabel}
        className="flex gap-3 overflow-x-auto px-1 pb-3 pt-1 scrollbar-none"
      >
        {days.map((date, index) => (
          <DateButton
            key={date.toISOString()}
            ref={(el) => {
              buttonRefs.current[index] = el;
            }}
            date={date}
            today={today}
            selectedDate={selectedDate}
            schedules={schedules}
            onDateSelect={onDateSelect}
            onKeyDown={(event) => {
              handleKeyDown(event, index);
            }}
          />
        ))}
      </div>
    </div>
  );
}
