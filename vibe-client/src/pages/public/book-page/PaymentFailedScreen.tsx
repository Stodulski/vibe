import { Link } from 'react-router-dom';
import { XCircle } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { formatDateFull, formatHourRange, formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { readStoredBookingInfo } from '@/features/public-booking';

const t = ES_AR;

/**
 * What someone sees when MercadoPago rejects their card.
 *
 * It replaces a sonner toast. The toast appeared over the complex page's full
 * availability grid and then dismissed itself, which left the person staring
 * at hundreds of prices with no trace of what they had been doing — a
 * countdown on the one message in the flow they most needed to keep reading.
 *
 * The slot they were paying for is read back from session storage, the same
 * entry the success page reads, written by the confirm page right before the
 * redirect. It can legitimately be missing — another tab, cleared storage —
 * so the summary is the part that disappears, never the explanation or the
 * way forward.
 *
 * No retry button yet, deliberately. Re-posting the same slot collides with
 * the lock the first attempt still holds and comes back as "no longer
 * available"; returning to the original checkout is the right move and needs
 * its own verification first.
 */
export function PaymentFailedScreen({ slug }: { slug: string }) {
  const booking = readStoredBookingInfo();

  return (
    <div className="mx-auto flex w-full max-w-md flex-col items-center gap-5 px-2 py-10 text-center animate-fade-in sm:gap-6 sm:px-0 sm:py-16">
      <XCircle className="size-12 text-error-text" aria-hidden="true" />

      <div className="mb-4 space-y-2">
        <h1 className="text-xl font-bold text-text-primary">{t.publicBooking.paymentFailed}</h1>
        <p className="text-sm text-text-secondary">{t.publicBooking.paymentFailedDescription}</p>
      </div>

      {booking && (
        <div className="w-full rounded-2xl border border-border-subtle bg-bg-subtle p-4 text-left">
          <p className="text-xs font-semibold uppercase tracking-wide text-text-tertiary">
            {t.publicBooking.paymentFailedSlotLabel}
          </p>
          <p className="mt-2 text-sm font-semibold text-text-primary">{booking.courtName}</p>
          <p className="mt-0.5 text-sm text-text-secondary">
            {formatDateFull(booking.date)} · {formatHourRange(booking.startsAt, booking.endsAt, ' a ')}
          </p>
          <p className="mt-0.5 text-sm tabular-nums text-text-secondary">{formatPrice(booking.price)}</p>
        </div>
      )}

      <div className="flex w-full flex-col gap-2">
        <Button asChild size="lg" className="min-h-12">
          <Link to={`/${slug}`}>{t.publicBooking.paymentFailedChooseAnother}</Link>
        </Button>
        {booking && (
          <Button asChild variant="ghost" className="min-h-12">
            <a href={`tel:${booking.complexPhone}`}>{t.publicBooking.paymentFailedCallComplex}</a>
          </Button>
        )}
      </div>
    </div>
  );
}
