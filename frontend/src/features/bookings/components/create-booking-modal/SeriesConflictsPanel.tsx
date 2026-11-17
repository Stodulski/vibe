import { useId, useState } from 'react';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import { keepTogether } from '@/shared/lib/keepTogether';
import { formatSeriesDay, weekdayOf } from '../../lib/seriesDates';
import type { BookingSeriesConflict, CreateBookingSeriesRequest } from '@/shared/types/api.types';

const t = ES_AR;
const s = t.bookings.series;

type DateCourt = NonNullable<CreateBookingSeriesRequest['date_courts']>[number];

/** What holds a date that collides, as a short tag: the holder when the server says, else the reason. */
function conflictTag(conflict: BookingSeriesConflict): string {
  if (conflict.reason === 'overlap' && conflict.held_by) return s.conflictTags[conflict.held_by];
  return s.conflictTags[conflict.reason];
}

/** Radix takes no empty value on an item, so "no reservar ese día" is this stand-in value. */
const SKIP_DAY = '__skip_day__';

/** The selector of one date: the app's Select, offering "no reservar ese día" and then the courts free that day. */
function CourtSelect({
  conflict,
  labelledBy,
  value,
  disabled,
  onChange,
}: {
  conflict: BookingSeriesConflict;
  labelledBy: string;
  value: string;
  disabled: boolean;
  onChange: (courtId: string) => void;
}) {
  return (
    <div className="min-w-0 flex-1">
      <Select
        value={value === '' ? SKIP_DAY : value}
        disabled={disabled}
        onValueChange={(courtId) => {
          onChange(courtId === SKIP_DAY ? '' : courtId);
        }}
      >
        <SelectTrigger aria-labelledby={labelledBy}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={SKIP_DAY}>{s.skipDayOption}</SelectItem>
          {(conflict.free_courts ?? []).map((court) => (
            <SelectItem key={court.court_id} value={court.court_id}>
              {keepTogether(court.court_name)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

/**
 * What to do with one occupied date: leave it out (the default) or book that one
 * turn on another court free that day. With no court free it only says so.
 */
function DayChoice({
  conflict,
  value,
  disabled,
  onChange,
}: {
  conflict: BookingSeriesConflict;
  value: string;
  disabled: boolean;
  onChange: (courtId: string) => void;
}) {
  const labelId = useId();
  const dateId = useId();
  return (
    <li className="border-border space-y-2 rounded-lg border p-3">
      <div className="flex flex-wrap items-baseline justify-between gap-x-3">
        <span id={dateId} data-testid="conflict-date" className="text-text-primary text-sm font-medium">
          {formatSeriesDay(conflict.date)}
        </span>
        <span data-testid="conflict-tag" className="text-text-tertiary text-xs">
          {conflictTag(conflict)}
        </span>
      </div>
      {(conflict.free_courts ?? []).length === 0 ? (
        <p className="text-text-tertiary text-sm">{s.noOtherCourtFree}</p>
      ) : (
        <div className="flex flex-col gap-1.5 sm:flex-row sm:items-center sm:gap-3">
          <span id={labelId} className="text-text-secondary shrink-0 text-sm">
            {s.dayChoiceLabel}
          </span>
          <CourtSelect
            conflict={conflict}
            labelledBy={`${labelId} ${dateId}`}
            value={value}
            disabled={disabled}
            onChange={onChange}
          />
        </div>
      )}
    </li>
  );
}

/** The occupied dates of the court, one row each, with the answer picked for each. */
function OccupiedDays({
  court,
  conflicts,
  picked,
  disabled,
  endless,
  onPick,
}: {
  court: string;
  conflicts: BookingSeriesConflict[];
  picked: Record<string, string>;
  disabled: boolean;
  endless: boolean;
  onPick: (date: string, courtId: string) => void;
}) {
  const listId = useId();
  return (
    <section className="space-y-2">
      <h4 id={listId} className="text-text-primary text-sm font-medium">
        {s.occupiedDaysTitle(court)}
      </h4>
      <ul aria-labelledby={listId} className="space-y-2">
        {conflicts.map((conflict) => (
          <DayChoice
            key={conflict.date}
            conflict={conflict}
            value={picked[conflict.date] ?? ''}
            disabled={disabled}
            onChange={(courtId) => {
              onPick(conflict.date, courtId);
            }}
          />
        ))}
      </ul>
      {endless && <p className="text-text-tertiary text-xs">{s.conflictsEndless}</p>}
    </section>
  );
}

/** The problem in a line, with the court and the weekday, and what to do about it. */
function PanelHeading({ court, conflicts }: { court: string; conflicts: BookingSeriesConflict[] }) {
  const firstDate = conflicts[0]?.date;
  const weekday = firstDate ? weekdayOf(firstDate) : 0;
  return (
    <div className="space-y-1">
      <h3 className="text-text-primary text-base font-semibold">
        {s.conflictsTitle(court, conflicts.length, s.weekdaysPlural[weekday] ?? '', s.weekdaysSingular[weekday] ?? '')}
      </h3>
      <p className="text-text-secondary text-sm">{s.conflictsHint}</p>
    </div>
  );
}

/**
 * What the create-series call refused: every date the chosen court cannot hold,
 * each with its own answer. By default a date is left out; where another court
 * of the same sport is free that day, that one turn can be booked there instead.
 * The series itself always stays on the chosen court. A series with no end only
 * lists the year ahead, which the panel says.
 */
export function SeriesConflictsPanel({
  courtName,
  conflicts,
  endless,
  pending,
  onCreate,
  onBack,
}: {
  /** The court the series was refused on. */
  courtName: string;
  conflicts: BookingSeriesConflict[];
  /** The series has no end, so only the year ahead was checked. */
  endless: boolean;
  pending: boolean;
  /** Creates the series, booking the given dates on the courts picked for them. */
  onCreate: (dateCourts: DateCourt[]) => void;
  onBack: () => void;
}) {
  const [picked, setPicked] = useState<Record<string, string>>({});
  const court = keepTogether(courtName);
  const dateCourts = conflicts.flatMap((conflict) => {
    const courtId = picked[conflict.date];
    return courtId ? [{ date: conflict.date, court_id: courtId }] : [];
  });

  return (
    // On a phone the dialog fills the screen and its form keeps the footer at the
    // bottom (see `DialogContent`); this panel is not a form, so it asks for the same.
    <div className="flex flex-1 flex-col gap-5 sm:flex-none">
      <PanelHeading court={court} conflicts={conflicts} />
      <OccupiedDays
        court={court}
        conflicts={conflicts}
        picked={picked}
        disabled={pending}
        endless={endless}
        onPick={(date, courtId) => {
          setPicked((current) => ({ ...current, [date]: courtId }));
        }}
      />
      <p aria-live="polite" className="text-text-secondary text-sm">
        {s.choicesSummary(dateCourts.length, conflicts.length - dateCourts.length)}
      </p>
      <SectionFooter
        className="mt-auto sm:mt-0"
        onCancel={onBack}
        cancelLabel={s.conflictsBack}
        cancelDisabled={pending}
        submitLabel={s.createLabel}
        pending={pending}
        onSubmit={() => {
          onCreate(dateCourts);
        }}
      />
    </div>
  );
}
