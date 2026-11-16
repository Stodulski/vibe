import { Repeat } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { InfoRow } from '@/shared/components/common/InfoRow';
import { BookingSeriesLabelSkeleton } from './BookingSeriesLabelSkeleton';
import type { BookingSeries } from '@/shared/types/api.types';

const t = ES_AR;

/** The weekday and hour a fixed series repeats on, as a fact under its own label: "Turno fijo" over "Todos los martes a las 21:00". */
export function SeriesRepeatRow({ label, weekday, time }: { label: string; weekday: string; time: string }) {
  return (
    <InfoRow label={label}>
      <span className="flex items-center gap-1.5">
        <Repeat className="text-text-tertiary size-4 shrink-0" aria-hidden="true" />
        <span className="min-w-0 break-words">{t.bookings.series.everyWeekdayAt(weekday, time)}</span>
      </span>
    </InfoRow>
  );
}

/** The series a booking belongs to, from its series. */
export function BookingSeriesLabel({ series, isPending }: { series: BookingSeries | undefined; isPending: boolean }) {
  if (isPending) return <BookingSeriesLabelSkeleton />;
  // A failed fetch drops the line rather than the sheet: the booking itself is intact.
  if (!series) return null;

  return (
    <SeriesRepeatRow
      label={t.bookings.fixedBooking}
      weekday={t.bookings.series.weekdaysPlural[series.weekday] ?? ''}
      time={series.start_time.slice(0, 5)}
    />
  );
}
