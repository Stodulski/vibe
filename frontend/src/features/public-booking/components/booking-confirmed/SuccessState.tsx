import { CheckCircle2 } from 'lucide-react';
import { StatusHero } from '@/shared/components/common/StatusHero';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { BookingStatus, BookingStatusDetails } from '@/shared/types/api.types';
import { BookingSummary } from './BookingSummary';
import { SuccessFooterActions } from './SuccessFooterActions';
import { mergeBookingInfo } from './mergeBookingInfo';
import { cancellationLineText, canCancelBooking } from './cancellationCopy';
import type { BookingInfo } from './types';

const t = ES_AR;

interface SuccessStateProps {
  bookingInfo: BookingInfo | null;
  /** `GET /book/status`'s answer — undefined while the first request is still in flight, or for the cache-only render before it lands. */
  bookingDetails: BookingStatusDetails | undefined;
  token: string | null;
  status: BookingStatus | undefined;
  slug: string;
}

// The confirmed booking as a ticket: badge and title centered above one card
// (date, time, court, and the money split), the cancellation window as a
// plain sentence underneath, and the two footer actions — "Nueva reserva"
// primary, "Cancelar reserva" neutral.
export function SuccessState({ bookingInfo, bookingDetails, token, status, slug }: SuccessStateProps) {
  const info = mergeBookingInfo(bookingDetails, bookingInfo);
  const cancellationText = info && cancellationLineText(info);

  return (
    <StatusHero icon={CheckCircle2} tone="success" size="large" title={t.publicBooking.bookingSuccess} align="center">
      {info && (
        <div className="w-full max-w-sm animate-fade-in" style={{ animationDelay: '200ms' }}>
          <BookingSummary bookingInfo={info} />

          {cancellationText && <p className="mt-6 text-center text-sm text-text-secondary">{cancellationText}</p>}
        </div>
      )}

      <SuccessFooterActions slug={slug} token={token} status={status} canCancel={canCancelBooking(info)} />
    </StatusHero>
  );
}
