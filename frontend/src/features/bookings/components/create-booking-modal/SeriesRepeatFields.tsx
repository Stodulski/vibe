import { useState } from 'react';
import { parseISO } from 'date-fns/parseISO';
import { format } from 'date-fns/format';
import { es } from 'date-fns/locale/es';
import { FormField } from '@/shared/components/common/FormField';
import { Calendar } from '@/shared/components/ui/calendar';
import { Input } from '@/shared/components/ui/input';
import { Label } from '@/shared/components/ui/label';
import { Popover, PopoverContent, PopoverTrigger } from '@/shared/components/ui/popover';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { Switch } from '@/shared/components/ui/switch';
import { ES_AR } from '@/shared/i18n/es_AR';
import { DateFieldTrigger } from './DateField';
import type { SeriesEndMode, SeriesOptions } from './useSeriesOptions';

const t = ES_AR;

const END_MODES: { value: SeriesEndMode; label: string }[] = [
  { value: 'never', label: t.bookings.series.endNever },
  { value: 'until', label: t.bookings.series.endUntil },
  { value: 'weeks', label: t.bookings.series.endWeeks },
];

function UntilPicker({ firstDate, series }: { firstDate: string; series: SeriesOptions }) {
  const [open, setOpen] = useState(false);
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <DateFieldTrigger id="series-until" date={series.until || undefined} hasError={!!series.error} />
      </PopoverTrigger>
      <PopoverContent className="w-auto max-w-[calc(100vw-2rem)] p-0" align="start">
        <Calendar
          mode="single"
          selected={series.until ? parseISO(series.until) : undefined}
          onSelect={(day) => {
            if (day) {
              series.setUntil(format(day, 'yyyy-MM-dd'));
              setOpen(false);
            }
          }}
          disabled={firstDate ? { before: parseISO(firstDate) } : undefined}
          locale={es}
        />
      </PopoverContent>
    </Popover>
  );
}

function EndChoice({ firstDate, series }: { firstDate: string; series: SeriesOptions }) {
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      <FormField label={t.bookings.series.endLabel} htmlFor="series-end-mode">
        <Select
          value={series.endMode}
          onValueChange={(v) => {
            series.setEndMode(v as SeriesEndMode);
          }}
        >
          <SelectTrigger id="series-end-mode">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {END_MODES.map((mode) => (
              <SelectItem key={mode.value} value={mode.value}>
                {mode.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </FormField>
      {series.endMode === 'until' && (
        <FormField label={t.bookings.series.endUntil} htmlFor="series-until" error={series.error}>
          <UntilPicker firstDate={firstDate} series={series} />
        </FormField>
      )}
      {series.endMode === 'weeks' && (
        <FormField label={t.bookings.series.weeksLabel} htmlFor="series-weeks" error={series.error}>
          <Input
            id="series-weeks"
            type="number"
            inputMode="numeric"
            min={2}
            max={52}
            value={series.weeks}
            onChange={(e) => {
              series.setWeeks(e.target.value);
            }}
          />
        </FormField>
      )}
    </div>
  );
}

/** The "repeat every week" switch of step 1 and, once on, how the series ends. */
export function SeriesRepeatFields({ firstDate, series }: { firstDate: string; series: SeriesOptions }) {
  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between gap-3">
        <Label htmlFor="series-repeat">{t.bookings.series.repeatWeekly}</Label>
        <Switch id="series-repeat" checked={series.repeat} onCheckedChange={series.setRepeat} />
      </div>
      {series.repeat && <EndChoice firstDate={firstDate} series={series} />}
    </div>
  );
}
