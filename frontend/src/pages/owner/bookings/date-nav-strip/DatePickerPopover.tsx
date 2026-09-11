import { format } from 'date-fns/format';
import { parseISO } from 'date-fns/parseISO';
import { es } from 'date-fns/locale/es';
import { CalendarIcon } from 'lucide-react';
import { Popover, PopoverContent, PopoverTrigger } from '@/shared/components/ui/popover';
import { Calendar } from '@/shared/components/ui/calendar';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function DatePickerPopover({
  selectedDate,
  dateLabel,
  isToday,
  calendarOpen,
  onCalendarOpenChange,
  onDateSelect,
}: {
  selectedDate: string;
  dateLabel: string;
  isToday: boolean;
  calendarOpen: boolean;
  onCalendarOpenChange: (open: boolean) => void;
  onDateSelect: (date: string) => void;
}) {
  return (
    <Popover open={calendarOpen} onOpenChange={onCalendarOpenChange}>
      <PopoverTrigger asChild>
        <button
          className="flex min-h-0 min-w-0 items-center gap-2 rounded-lg px-2 py-1.5 text-left transition-colors hover:bg-bg-elevated/50"
          aria-label={`Fecha: ${dateLabel}`}
        >
          <CalendarIcon className="hidden size-4 shrink-0 text-primary-400 sm:block" />
          <span className="truncate whitespace-nowrap text-sm font-semibold first-letter:uppercase text-text-primary sm:text-base">
            {dateLabel}
          </span>
          {/* Plain text, like the "Ir a hoy" control it shares this slot with:
              the two swap places, so they should read the same weight. */}
          {isToday && (
            <span className="-mr-1.5 ml-2 hidden shrink-0 text-xs font-medium text-primary-400 sm:inline">
              {t.bookings.today}
            </span>
          )}
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-auto p-0" align="start">
        <Calendar
          mode="single"
          selected={parseISO(selectedDate)}
          onSelect={(day) => {
            if (day) {
              onDateSelect(format(day, 'yyyy-MM-dd'));
              onCalendarOpenChange(false);
            }
          }}
          locale={es}
        />
      </PopoverContent>
    </Popover>
  );
}
