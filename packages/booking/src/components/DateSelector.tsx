import { useMemo, useState } from 'react';
import { format } from 'date-fns/format';
import { addDays } from 'date-fns/addDays';
import { differenceInCalendarDays } from 'date-fns/differenceInCalendarDays';
import { es } from 'date-fns/locale/es';
import { BOOKING_MESSAGES } from '../messages';
import type { Schedule } from '../api/types';
import { useLoadMoreOnScroll } from './date-selector/useLoadMoreOnScroll';
import { useAutoScrollToSelectedDate } from './date-selector/useAutoScrollToSelectedDate';
import { useDateStripKeyboardNav } from './date-selector/useDateStripKeyboardNav';
import { DateButton } from './date-selector/DateButton';
import { todayInVenueTz } from '../lib/venueDate';

const t = BOOKING_MESSAGES;

interface DateSelectorProps {
  selectedDate: Date;
  onDateSelect: (date: Date) => void;
  schedules: Schedule[];
}

export function DateSelector({ selectedDate, onDateSelect, schedules }: DateSelectorProps) {
  // The venue's today, not the browser's: the server and the hydrating client must agree.
  const today = useMemo(() => todayInVenueTz(), []);
  const [daysCount, setDaysCount] = useState(14);
  // A day chosen from elsewhere (a fully booked day's "next free day") can lie
  // past the strip's current end. The strip grows far enough to show it, so it
  // is on screen and the auto-scroll below can reach it.
  const selectedOffset = differenceInCalendarDays(selectedDate, today) + 1;
  const visibleCount = Math.max(daysCount, selectedOffset);
  const days = useMemo(() => Array.from({ length: visibleCount }, (_, i) => addDays(today, i)), [today, visibleCount]);

  const scrollRef = useLoadMoreOnScroll(() => {
    setDaysCount((prev) => prev + 14);
  });
  useAutoScrollToSelectedDate(scrollRef, days, selectedDate);
  const { buttonRefs, handleKeyDown } = useDateStripKeyboardNav(days, schedules, onDateSelect);

  return (
    <div className="w-full">
      <h2 className="text-text-primary mb-4 text-sm font-semibold first-letter:uppercase">
        {format(selectedDate, "EEE d 'de' MMM, yyyy", { locale: es })}
      </h2>
      <div
        ref={scrollRef}
        role="radiogroup"
        aria-label={t.publicBooking.dateStripLabel}
        className="scrollbar-none flex gap-3 overflow-x-auto px-1 pt-1 pb-3"
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
