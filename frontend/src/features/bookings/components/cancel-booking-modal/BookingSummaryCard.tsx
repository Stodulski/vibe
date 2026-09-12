import { ES_AR } from '@/shared/i18n/es_AR';
import { formatDate, formatHourRange } from '@/shared/lib/utils';

const t = ES_AR;

export function BookingSummaryCard({
  booking,
}: {
  booking: {
    court_name: string;
    date: string;
    /** RFC3339, Argentina offset — rendered together as one range. */
    starts_at: string;
    ends_at: string;
    client_name: string;
  };
}) {
  return (
    <div className="border-border-subtle bg-bg-subtle rounded-xl border p-3 text-sm">
      <div className="space-y-1.5">
        <div className="flex justify-between gap-3">
          <span className="text-text-tertiary shrink-0">{t.bookings.court}</span>
          <span className="text-text-primary truncate font-medium">{booking.court_name}</span>
        </div>
        <div className="flex justify-between gap-3">
          <span className="text-text-tertiary shrink-0">{t.bookings.date}</span>
          <span className="text-text-primary font-medium">{formatDate(booking.date)}</span>
        </div>
        <div className="flex justify-between gap-3">
          <span className="text-text-tertiary shrink-0">{t.bookings.time}</span>
          <span className="score-text text-text-primary font-medium">
            {formatHourRange(booking.starts_at, booking.ends_at)}
          </span>
        </div>
        <div className="flex justify-between gap-3">
          <span className="text-text-tertiary shrink-0">{t.bookings.client}</span>
          <span className="text-text-primary truncate font-medium">{booking.client_name}</span>
        </div>
      </div>
    </div>
  );
}
