import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatDateOnlyWithYear } from '../../lib/seriesDates';
import type { BookingSeriesConflict } from '@/shared/types/api.types';

const t = ES_AR;

/** Why a date cannot be booked, naming what holds it when the server says. */
function conflictReason(conflict: BookingSeriesConflict): string {
  if (conflict.reason === 'overlap' && conflict.held_by) return t.bookings.series.heldBy[conflict.held_by];
  return t.bookings.series.conflictReasons[conflict.reason];
}

/**
 * What the create-series call refused: the dates with their year, what holds
 * each one, and the two ways out — create the series without them, or go back
 * and change the time. A series with no end only lists the year ahead, which
 * the panel says.
 */
export function SeriesConflictsPanel({
  conflicts,
  endless,
  pending,
  onSkip,
  onBack,
}: {
  conflicts: BookingSeriesConflict[];
  /** The series has no end, so only the year ahead was checked. */
  endless: boolean;
  pending: boolean;
  onSkip: () => void;
  onBack: () => void;
}) {
  return (
    // On a phone the dialog fills the screen and its form keeps the footer at the
    // bottom (see `DialogContent`); this panel is not a form, so it asks for the same.
    <div className="flex flex-1 flex-col gap-4 sm:flex-none">
      <div className="space-y-1">
        <p className="text-text-primary text-sm font-medium">{t.bookings.series.conflictsTitle(conflicts.length)}</p>
        {endless && <p className="text-text-tertiary text-sm">{t.bookings.series.conflictsEndless}</p>}
      </div>
      <ul className="max-h-60 space-y-1.5 overflow-y-auto text-sm">
        {conflicts.map((conflict) => (
          <li key={conflict.date} className="text-text-secondary">
            <span className="text-text-primary font-medium">{formatDateOnlyWithYear(conflict.date)}</span>
            {' — '}
            {conflictReason(conflict)}
          </li>
        ))}
      </ul>
      <SectionFooter
        className="mt-auto sm:mt-0"
        onCancel={onBack}
        cancelLabel={t.bookings.series.conflictsBack}
        cancelDisabled={pending}
        submitLabel={t.bookings.series.skipConflicts}
        pending={pending}
        onSubmit={onSkip}
      />
    </div>
  );
}
