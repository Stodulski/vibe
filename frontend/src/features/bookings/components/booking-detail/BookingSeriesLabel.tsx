import { Repeat } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { BookingSeriesLabelSkeleton } from './BookingSeriesLabelSkeleton';
import { SERIES_LABEL_ROW } from './seriesLabelLayout';
import type { BookingSeries } from '@/shared/types/api.types';

const t = ES_AR;

/** "Turno fijo · todos los martes a las 21:00", from the series the booking belongs to. */
export function BookingSeriesLabel({ series, isPending }: { series: BookingSeries | undefined; isPending: boolean }) {
  if (isPending) return <BookingSeriesLabelSkeleton />;
  // A failed fetch drops the line rather than the sheet: the booking itself is intact.
  if (!series) return null;

  const weekday = t.bookings.series.weekdaysPlural[series.weekday] ?? '';
  return (
    <div className={SERIES_LABEL_ROW}>
      <Repeat className="text-text-tertiary size-4 shrink-0" aria-hidden="true" />
      <p className="text-text-secondary min-w-0 truncate text-sm">
        {t.bookings.series.label(weekday, series.start_time.slice(0, 5))}
      </p>
    </div>
  );
}
