import { format } from 'date-fns/format';
import { parseISO } from 'date-fns/parseISO';
import { es } from 'date-fns/locale/es';
import { CalendarIcon } from 'lucide-react';
import { FormField } from '@/shared/components/common/FormField';
import { FieldRequirement } from '@/shared/components/common/FieldRequirement';
import { Calendar } from '@/shared/components/ui/calendar';
import { Popover, PopoverContent, PopoverTrigger } from '@/shared/components/ui/popover';
import { cn } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { UseFormSetValue, FieldErrors } from 'react-hook-form';
import type { CreateBookingDto } from '../../schemas/booking.schema';
import { todayInArgentina } from '../../lib/today';

const t = ES_AR;

function DateFieldTrigger({ date, hasError }: { date: string | undefined; hasError: boolean }) {
  return (
    <button
      id="booking-date"
      type="button"
      className={cn(
        'flex h-10 w-full items-center gap-2.5 rounded-xl border border-border-subtle bg-bg-base/60 px-3.5 text-sm shadow-xs outline-none transition-input',
        'hover:border-border-default hover:bg-bg-base/80',
        'focus-visible:border-primary-500/60 focus-visible:bg-bg-base focus-visible:ring-[3px] focus-visible:ring-primary-500/15',
        date ? 'text-text-primary' : 'text-text-tertiary/70',
        hasError && 'border-error-border bg-error-bg/30 ring-1 ring-error-text/10',
      )}
    >
      <CalendarIcon className="size-4 shrink-0 text-text-tertiary" />
      <span className="truncate first-letter:uppercase">
        {date ? format(parseISO(date), "EEE d 'de' MMM yyyy", { locale: es }) : t.bookings.selectDate}
      </span>
    </button>
  );
}

interface DateFieldProps {
  date: string | undefined;
  calendarOpen: boolean;
  setCalendarOpen: (v: boolean) => void;
  setValue: UseFormSetValue<CreateBookingDto>;
  errors: FieldErrors<CreateBookingDto>;
}

export function DateField({ date, calendarOpen, setCalendarOpen, setValue, errors }: DateFieldProps) {
  return (
    <FormField
      label={
        <>
          {t.bookings.date}
          <FieldRequirement required />
        </>
      }
      htmlFor="booking-date"
      error={errors.date?.message}
    >
      <Popover open={calendarOpen} onOpenChange={setCalendarOpen}>
        <PopoverTrigger asChild>
          <DateFieldTrigger date={date} hasError={!!errors.date} />
        </PopoverTrigger>
        <PopoverContent className="w-auto max-w-[calc(100vw-2rem)] p-0" align="start">
          <Calendar
            mode="single"
            selected={date ? parseISO(date) : undefined}
            onSelect={(day) => {
              if (day) {
                setValue('date', format(day, 'yyyy-MM-dd'));
                setValue('start_time', '');
                setCalendarOpen(false);
              }
            }}
            // The venue's "today" (Argentina), not the browser's — see
            // lib/today.ts. `parseISO` on a date-only string is already local
            // midnight, so no extra `startOfDay` is needed.
            disabled={{ before: parseISO(todayInArgentina()) }}
            locale={es}
          />
        </PopoverContent>
      </Popover>
    </FormField>
  );
}
