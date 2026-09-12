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
      <h3 className="text-micro text-text-tertiary shrink-0 font-semibold tracking-wider uppercase">
        {t.clients.recentBookings}
      </h3>
      {bookings.length === 0 ? (
        <p className="text-text-tertiary py-4 text-center text-sm">{t.common.noResults}</p>
      ) : (
        <div className="divide-border-subtle min-h-0 flex-1 divide-y overflow-y-auto pb-4">
          {bookings.slice(0, 10).map((b) => (
            <button
              key={b.id}
              type="button"
              onClick={() => {
                onSelect(b);
              }}
              className="hover:bg-bg-base/40 flex w-full items-center justify-between gap-2 py-2.5 text-left text-xs transition-colors"
            >
              <div className="flex min-w-0 items-center gap-3">
                <span className="score-text text-text-secondary shrink-0">{formatDateShort(b.date)}</span>
                <span className="text-text-secondary truncate">{b.court_name}</span>
              </div>
              <div className="flex shrink-0 items-center gap-1">
                <BookingStatusText status={b.status} className="text-xs" />
                <ChevronRight className="text-text-tertiary size-3.5" aria-hidden="true" />
              </div>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
