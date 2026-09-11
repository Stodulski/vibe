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
    <div className="rounded-xl border border-border-subtle bg-bg-subtle p-3 text-sm">
      <div className="space-y-1.5">
        <div className="flex justify-between gap-3">
          <span className="shrink-0 text-text-tertiary">{t.bookings.court}</span>
          <span className="truncate font-medium text-text-primary">{booking.court_name}</span>
        </div>
        <div className="flex justify-between gap-3">
          <span className="shrink-0 text-text-tertiary">{t.bookings.date}</span>
          <span className="font-medium text-text-primary">{formatDate(booking.date)}</span>
        </div>
        <div className="flex justify-between gap-3">
          <span className="shrink-0 text-text-tertiary">{t.bookings.time}</span>
          <span className="score-text font-medium text-text-primary">
            {formatHourRange(booking.starts_at, booking.ends_at)}
          </span>
        </div>
        <div className="flex justify-between gap-3">
          <span className="shrink-0 text-text-tertiary">{t.bookings.client}</span>
          <span className="truncate font-medium text-text-primary">{booking.client_name}</span>
        </div>
      </div>
    </div>
  );
}
