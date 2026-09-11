import { format, parseISO, startOfDay } from 'date-fns';
import { es } from 'date-fns/locale';
import { CalendarIcon } from 'lucide-react';
import { Textarea } from '@/shared/components/ui/textarea';
import { Calendar } from '@/shared/components/ui/calendar';
import { Popover, PopoverContent, PopoverTrigger } from '@/shared/components/ui/popover';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { FormField } from '@/shared/components/common/FormField';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';
import type { CourtWithPrices } from '@/shared/types/api.types';

const t = ES_AR;

interface DateFieldProps {
  date: string;
  error?: string | undefined;
  calendarOpen: boolean;
  setCalendarOpen: (open: boolean) => void;
  onSelectDate: (date: string) => void;
}

export function DateField({ date, error, calendarOpen, setCalendarOpen, onSelectDate }: DateFieldProps) {
  return (
    <FormField label={t.courts.blockDate} htmlFor="block-date" error={error}>
      <Popover open={calendarOpen} onOpenChange={setCalendarOpen}>
        <PopoverTrigger asChild>
          <button
            id="block-date"
            type="button"
            className={cn(
              'flex h-10 w-full items-center gap-2.5 rounded-xl border border-border-subtle bg-bg-base/60 px-3.5 text-sm shadow-xs outline-none transition-input',
              'hover:border-border-default hover:bg-bg-base/80',
              'focus-visible:border-primary-500/60 focus-visible:bg-bg-base focus-visible:ring-[3px] focus-visible:ring-primary-500/15',
              date ? 'text-text-primary' : 'text-text-tertiary/70',
            )}
          >
            <CalendarIcon className="size-4 shrink-0 text-text-tertiary" />
            <span className="truncate first-letter:uppercase">
              {date ? format(parseISO(date), "EEE d 'de' MMM yyyy", { locale: es }) : t.bookings.selectDate}
            </span>
          </button>
        </PopoverTrigger>
        <PopoverContent className="w-auto max-w-[calc(100vw-2rem)] p-0" align="start">
          <Calendar
            mode="single"
            selected={date ? parseISO(date) : undefined}
            onSelect={(day) => {
              if (day) {
                onSelectDate(format(day, 'yyyy-MM-dd'));
                setCalendarOpen(false);
              }
            }}
            disabled={{ before: startOfDay(new Date()) }}
            locale={es}
          />
        </PopoverContent>
      </Popover>
    </FormField>
  );
}

interface CourtFieldProps {
  courtId: string;
  error?: string | undefined;
  setCourtId: (id: string) => void;
  activeCourts: CourtWithPrices[];
}

export function CourtField({ courtId, error, setCourtId, activeCourts }: CourtFieldProps) {
  return (
    <FormField label={t.bookings.court} htmlFor="block-court" error={error}>
      <Select value={courtId} onValueChange={setCourtId}>
        <SelectTrigger id="block-court">
          <SelectValue placeholder={t.bookings.selectCourt} />
        </SelectTrigger>
        <SelectContent>
          {activeCourts.map((court) => (
            <SelectItem key={court.id} value={court.id}>
              {court.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </FormField>
  );
}

interface TimeRangeFieldsProps {
  date: string;
  startTime: string;
  endTime: string;
  startTimeError?: string | undefined;
  endTimeError?: string | undefined;
  startTimeOptions: string[];
  endTimeOptions: string[];
  setStartTime: (time: string) => void;
  setEndTime: (time: string) => void;
}

export function TimeRangeFields({
  date,
  startTime,
  endTime,
  startTimeError,
  endTimeError,
  startTimeOptions,
  endTimeOptions,
  setStartTime,
  setEndTime,
}: TimeRangeFieldsProps) {
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      <FormField label={t.courts.blockStartTime} htmlFor="block-start-time" error={startTimeError}>
        <Select value={startTime} onValueChange={setStartTime} disabled={!date}>
          <SelectTrigger id="block-start-time">
            <SelectValue placeholder="--:--" />
          </SelectTrigger>
          <SelectContent>
            {startTimeOptions.map((time) => (
              <SelectItem key={time} value={time}>
                {time}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </FormField>
      <FormField label={t.courts.blockEndTime} htmlFor="block-end-time" error={endTimeError}>
        <Select value={endTime} onValueChange={setEndTime} disabled={!startTime}>
          <SelectTrigger id="block-end-time">
            <SelectValue placeholder="--:--" />
          </SelectTrigger>
          <SelectContent>
            {endTimeOptions.map((time) => (
              <SelectItem key={time} value={time}>
                {time}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </FormField>
    </div>
  );
}

interface ReasonFieldProps {
  reason: string;
  setReason: (reason: string) => void;
}

export function ReasonField({ reason, setReason }: ReasonFieldProps) {
  return (
    <FormField label={t.courts.blockReason} htmlFor="block-reason">
      <Textarea
        id="block-reason"
        value={reason}
        onChange={(e) => {
          setReason(e.target.value);
        }}
        placeholder={t.placeholders.blockReason}
        maxLength={500}
        className="min-h-[40px] resize-none"
      />
    </FormField>
  );
}
