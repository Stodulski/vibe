import { DateNavControls } from './date-nav-strip/DateNavControls';
import { WeekStrip } from './date-nav-strip/WeekStrip';

interface DateNavStripProps {
  selectedDate: string;
  dateLabel: string;
  isToday: boolean;
  calendarOpen: boolean;
  onCalendarOpenChange: (open: boolean) => void;
  onDateSelect: (date: string) => void;
  onPrevDay: () => void;
  onNextDay: () => void;
  onGoToToday: () => void;
}

export function DateNavStrip({
  selectedDate,
  dateLabel,
  isToday,
  calendarOpen,
  onCalendarOpenChange,
  onDateSelect,
  onPrevDay,
  onNextDay,
  onGoToToday,
}: DateNavStripProps) {
  return (
    <div className="space-y-3">
      {/* Date controls float above the card, centred. */}
      <div className="flex items-center justify-center">
        <DateNavControls
          selectedDate={selectedDate}
          dateLabel={dateLabel}
          isToday={isToday}
          calendarOpen={calendarOpen}
          onCalendarOpenChange={onCalendarOpenChange}
          onDateSelect={onDateSelect}
          onPrevDay={onPrevDay}
          onNextDay={onNextDay}
          onGoToToday={onGoToToday}
        />
      </div>

      {/* No card: each day tile is already its own hit target, and a box
          around the row only drew a second border around borders. */}
      <WeekStrip selectedDate={selectedDate} onDateSelect={onDateSelect} />
    </div>
  );
}
