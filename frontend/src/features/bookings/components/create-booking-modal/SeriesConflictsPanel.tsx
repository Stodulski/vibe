import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatDateOnly } from '../../lib/seriesDates';
import type { BookingSeriesConflict } from '@/shared/types/api.types';

const t = ES_AR;

/**
 * What the create-series call refused: the dates, why each one cannot be
 * booked, and the two ways out — create the series without them, or go back
 * and change the time.
 */
export function SeriesConflictsPanel({
  conflicts,
  pending,
  onSkip,
  onBack,
}: {
  conflicts: BookingSeriesConflict[];
  pending: boolean;
  onSkip: () => void;
  onBack: () => void;
}) {
  return (
    <div className="space-y-4">
      <p className="text-text-primary text-sm font-medium">{t.bookings.series.conflictsTitle}</p>
      <ul className="max-h-60 space-y-1.5 overflow-y-auto text-sm">
        {conflicts.map((conflict) => (
          <li key={conflict.date} className="text-text-secondary">
            <span className="text-text-primary font-medium">{formatDateOnly(conflict.date)}</span>
            {' — '}
            {(t.bookings.series.conflictReasons as Record<string, string>)[conflict.reason] ?? conflict.reason}
          </li>
        ))}
      </ul>
      <SectionFooter
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
