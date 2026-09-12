import { ChevronLeft, ChevronRight } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { DatePickerPopover } from './DatePickerPopover';

const t = ES_AR;

interface DateNavControlsProps {
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

export function DateNavControls({
  selectedDate,
  dateLabel,
  isToday,
  calendarOpen,
  onCalendarOpenChange,
  onDateSelect,
  onPrevDay,
  onNextDay,
  onGoToToday,
}: DateNavControlsProps) {
  return (
    <div className="flex min-w-0 items-center justify-center gap-1">
      <Button
        variant="ghost"
        size="icon"
        className="size-9 shrink-0 rounded-lg"
        onClick={onPrevDay}
        aria-label={t.bookings.previousDay}
      >
        <ChevronLeft className="size-4" />
      </Button>

      <DatePickerPopover
        selectedDate={selectedDate}
        dateLabel={dateLabel}
        isToday={isToday}
        calendarOpen={calendarOpen}
        onCalendarOpenChange={onCalendarOpenChange}
        onDateSelect={onDateSelect}
      />

      {/* Takes the slot the "Hoy" badge occupies on today's date, so the
          control never jumps between two places. */}
      {!isToday && (
        <Button
          variant="ghost"
          size="sm"
          className="text-primary-400 hover:text-primary-300 -mx-2 shrink-0 text-xs"
          onClick={onGoToToday}
        >
          {t.bookings.goToToday}
        </Button>
      )}

      <Button
        variant="ghost"
        size="icon"
        className="size-9 shrink-0 rounded-lg"
        onClick={onNextDay}
        aria-label={t.bookings.nextDay}
      >
        <ChevronRight className="size-4" />
      </Button>
    </div>
  );
}
