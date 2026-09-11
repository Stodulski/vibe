import { ChevronRight } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn, formatDateShort } from '@/shared/lib/utils';
import { BookingStatusText } from '@/shared/components/common/StatusBadges';
import type { Booking } from '@/shared/types/api.types';

const t = ES_AR;

interface ClientRecentBookingsProps {
  bookings: Booking[];
  onSelect: (booking: Booking) => void;
  className?: string;
}

export function ClientRecentBookings({ bookings, onSelect, className }: ClientRecentBookingsProps) {
  return (
    <div className={cn('flex flex-col space-y-3', className)}>
      <h3 className="shrink-0 text-micro font-semibold uppercase tracking-wider text-text-tertiary">
        {t.clients.recentBookings}
      </h3>
      {bookings.length === 0 ? (
        <p className="py-4 text-center text-sm text-text-tertiary">{t.common.noResults}</p>
      ) : (
        <div className="min-h-0 flex-1 divide-y divide-border-subtle overflow-y-auto pb-4">
          {bookings.slice(0, 10).map((b) => (
            <button
              key={b.id}
              type="button"
              onClick={() => {
                onSelect(b);
              }}
              className="flex w-full items-center justify-between gap-2 py-2.5 text-left text-xs transition-colors hover:bg-bg-base/40"
            >
              <div className="flex min-w-0 items-center gap-3">
                <span className="score-text shrink-0 text-text-secondary">{formatDateShort(b.date)}</span>
                <span className="truncate text-text-secondary">{b.court_name}</span>
              </div>
              <div className="flex shrink-0 items-center gap-1">
                <BookingStatusText status={b.status} className="text-xs" />
                <ChevronRight className="size-3.5 text-text-tertiary" aria-hidden="true" />
              </div>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
