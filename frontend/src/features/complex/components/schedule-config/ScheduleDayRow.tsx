import { useCallback } from 'react';
import { Checkbox } from '@/shared/components/ui/checkbox';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';
import { DAYS } from './days';
import { TimeSelect } from './TimeSelect';
import { scheduleField, type ScheduleFormSetValue } from './scheduleField';
import { useScheduleRowState } from './useScheduleRowState';
import type { ScheduleFormControl } from './scheduleField';

const t = ES_AR;

/**
 * One day's opening hours, the same markup at every width.
 *
 * There were two components for this: a table on desktop and a card on a
 * phone. The table put "Lunes" 660px from its own opening-time select — the
 * gap the single-column rule exists to prevent — and the phone version, which
 * kept the day beside its times, was simply the better of the two.
 *
 * The three parts are reordered with flex `order`, NOT by rendering a mobile
 * copy and a desktop copy and hiding one. That is how the old pair worked, and
 * it put two checkboxes with the identical accessible name "Lunes: Cerrado"
 * into the page — a screen reader read every day twice. One control, moved.
 *
 *   phone     day + toggle on one line, times wrapped below
 *   sm and up day, times, toggle on a single line
 *
 * The times disappear when the day is closed: a closing time for a day that
 * never opens is not a setting, it is noise.
 */
export function ScheduleDayRow({
  index,
  control,
  setValue,
}: {
  index: number;
  control: ScheduleFormControl;
  setValue: ScheduleFormSetValue;
}) {
  const { isClosed, openTime, closeTime, isWeekend } = useScheduleRowState(index, control);
  const day = DAYS[index];
  const dayName = day ? t.complex.days[day] : '';

  // Stable identities so a row's re-render does not churn its two inputs.
  // `setValue` is stable across renders and `index` never changes for a row.
  const setOpen = useCallback(
    (v: string) => {
      setValue(scheduleField(index, 'open_time'), v);
    },
    [setValue, index],
  );
  const setClose = useCallback(
    (v: string) => {
      setValue(scheduleField(index, 'close_time'), v);
    },
    [setValue, index],
  );

  return (
    <div
      className={cn(
        'border-border-subtle flex flex-wrap items-center gap-x-4 gap-y-2 border-b py-3 last:border-0',
        isClosed && 'text-text-tertiary',
      )}
    >
      <DayName name={dayName} isClosed={isClosed} isWeekend={isWeekend} />

      <div className="order-3 flex w-full items-center gap-2 sm:order-2 sm:flex-1">
        {isClosed ? (
          <span className="text-text-tertiary text-xs">{t.complex.closedAllDay}</span>
        ) : (
          <OpeningHours
            dayName={dayName}
            openTime={openTime}
            closeTime={closeTime}
            onOpenChange={setOpen}
            onCloseChange={setClose}
          />
        )}
      </div>

      <ClosedToggle
        checked={isClosed}
        dayName={dayName}
        onChange={(v) => {
          setValue(scheduleField(index, 'is_closed'), v);
        }}
      />
    </div>
  );
}

function OpeningHours({
  dayName,
  openTime,
  closeTime,
  onOpenChange,
  onCloseChange,
}: {
  dayName: string;
  openTime: string;
  closeTime: string;
  onOpenChange: (value: string) => void;
  onCloseChange: (value: string) => void;
}) {
  return (
    <>
      <TimeSelect value={openTime} aria-label={`${dayName}: ${t.complex.openTime}`} onChange={onOpenChange} />
      <span className="text-micro text-text-tertiary shrink-0">{t.complex.timeRangeJoiner}</span>
      <TimeSelect value={closeTime} aria-label={`${dayName}: ${t.complex.closeTime}`} onChange={onCloseChange} />
    </>
  );
}

function ClosedToggle({
  checked,
  dayName,
  onChange,
}: {
  checked: boolean;
  dayName: string;
  onChange: (value: boolean) => void;
}) {
  return (
    <label className="order-2 flex shrink-0 cursor-pointer items-center gap-2 sm:order-3">
      {/* Beside the box, on every row. Naming it once in a column header is
          tidier on paper, but the reader scans ROWS here — they land on a day
          and look across it, and by then the header is off the top of their
          attention. */}
      <span className="text-micro text-text-tertiary">{t.complex.closed}</span>
      <Checkbox
        checked={checked}
        onCheckedChange={(value) => {
          onChange(value === true);
        }}
        aria-label={`${dayName}: ${t.complex.closed}`}
      />
    </label>
  );
}

function DayName({ name, isClosed, isWeekend }: { name: string; isClosed: boolean; isWeekend: boolean }) {
  return (
    <div className="order-1 flex min-w-0 flex-1 items-center gap-2 sm:w-28 sm:flex-none">
      <span
        className={cn(
          'truncate text-sm font-medium',
          isClosed ? 'text-text-tertiary' : isWeekend ? 'text-primary-400' : 'text-text-primary',
        )}
      >
        {name}
      </span>
    </div>
  );
}
