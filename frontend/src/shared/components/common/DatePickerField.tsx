import { useState, type ReactNode } from 'react';
import { format } from 'date-fns/format';
import { parseISO } from 'date-fns/parseISO';
import { startOfDay } from 'date-fns/startOfDay';
import { es } from 'date-fns/locale/es';
import { CalendarIcon } from 'lucide-react';
import type { Matcher } from 'react-day-picker';
import { Calendar } from '@/shared/components/ui/calendar';
import { Popover, PopoverContent, PopoverTrigger } from '@/shared/components/ui/popover';
import { FormField } from '@/shared/components/common/FormField';
import { cn } from '@/shared/lib/utils';

interface DatePickerFieldProps {
  id: string;
  /** Accepts JSX so a caller can append the "(requerido)" marker. */
  label: ReactNode;
  /** `YYYY-MM-DD`, or empty while nothing is picked. */
  value: string;
  placeholder: string;
  error?: string | undefined;
  onSelect: (date: string) => void;
  /** Days before today are not offered. */
  disablePast?: boolean;
  /** `YYYY-MM-DD` limits: days before `minDate` or after `maxDate` are not offered. */
  minDate?: string;
  maxDate?: string;
  /**
   * Which side of the field the calendar opens on. Inside a dialog the default
   * (below) lands on the footer's buttons: pass 'top' for a field low in a
   * form. Radix still flips it when the chosen side has no room.
   */
  side?: 'top' | 'bottom';
  helpText?: ReactNode;
}

/**
 * A labelled date field that opens a calendar popover and reports the pick as
 * `YYYY-MM-DD`. It owns its open state: callers only hold the value.
 */
export function DatePickerField({
  id,
  label,
  value,
  placeholder,
  error,
  onSelect,
  disablePast = true,
  minDate,
  maxDate,
  side = 'bottom',
  helpText,
}: DatePickerFieldProps) {
  const [open, setOpen] = useState(false);
  const disabled: Matcher[] = [];
  if (disablePast) disabled.push({ before: startOfDay(new Date()) });
  if (minDate) disabled.push({ before: parseISO(minDate) });
  if (maxDate) disabled.push({ after: parseISO(maxDate) });
  const selected = value ? parseISO(value) : undefined;
  const defaultMonth = selected ?? (minDate ? parseISO(minDate) : undefined);

  return (
    <FormField label={label} htmlFor={id} error={error} helpText={helpText}>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <button
            id={id}
            type="button"
            className={cn(
              'border-border-subtle bg-bg-base/60 transition-input flex h-10 w-full items-center gap-2.5 rounded-xl border px-3.5 text-sm shadow-xs outline-none',
              'hover:border-border-default hover:bg-bg-base/80',
              'focus-visible:border-primary-500/60 focus-visible:bg-bg-base focus-visible:ring-primary-500/15 focus-visible:ring-[3px]',
              value ? 'text-text-primary' : 'text-text-tertiary/70',
            )}
          >
            <CalendarIcon className="text-text-tertiary size-4 shrink-0" />
            <span className="truncate first-letter:uppercase">
              {value ? format(parseISO(value), "EEE d 'de' MMM yyyy", { locale: es }) : placeholder}
            </span>
          </button>
        </PopoverTrigger>
        <PopoverContent className="w-auto max-w-[calc(100vw-2rem)] p-0" align="start" side={side} collisionPadding={16}>
          <Calendar
            // 36px days on a phone (seven of them still fit the 288px popover at 320px), 40px from sm.
            className="[--cell-size:--spacing(9)] sm:[--cell-size:--spacing(10)]"
            mode="single"
            selected={selected}
            {...(defaultMonth ? { defaultMonth } : {})}
            onSelect={(day) => {
              if (day) {
                onSelect(format(day, 'yyyy-MM-dd'));
                setOpen(false);
              }
            }}
            {...(disabled.length > 0 ? { disabled } : {})}
            locale={es}
          />
        </PopoverContent>
      </Popover>
    </FormField>
  );
}
