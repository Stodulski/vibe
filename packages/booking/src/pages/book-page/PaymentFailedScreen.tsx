import { XCircle } from 'lucide-react';
import { Button } from '@vibe/ui';
import { formatDateFull, formatHourRange, formatPrice } from '@vibe/ui';
import { publicComplexPath } from '../../lib/publicPaths';
import { BOOKING_MESSAGES } from '../../messages';
import { readStoredBookingInfo } from '../../lib/storedBookingInfo';
import { whatsappChatHref } from '../../lib/whatsapp';
import { keepTogether } from '../../lib/keepTogether';
import { useFailedCheckout } from './useFailedCheckout';
import { failedSlotPath } from './failedSlotPath';

const t = BOOKING_MESSAGES;

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
 * A rejected payment leaves the booking pending and its slot held. When the
 * redirect carries the booking's link token (`booklink.Failure`), the page
 * offers paying again on that same checkout, or giving the slot back before
 * picking another time, so the visitor is never locked out by their own
 * booking. Without the token (an older link) it only links back to the hours of
 * that day. "Volver al complejo" is always there, so the way out never depends
 * on an action failing first.
 */
export function PaymentFailedScreen({ slug, token }: { slug: string; token?: string | undefined }) {
  const booking = readStoredBookingInfo();
  const whatsAppHref = booking ? whatsappChatHref(booking.complexPhone) : null;

  return (
    <div className="animate-fade-in mx-auto flex w-full max-w-md flex-col items-center gap-5 px-2 py-10 text-center sm:gap-6 sm:px-0 sm:py-16">
      <XCircle className="text-error-text size-12" aria-hidden="true" />

      <div className="mb-4 space-y-2">
        <h1 className="text-text-primary text-xl font-bold">{t.publicBooking.paymentFailed}</h1>
        <p className="text-text-secondary text-sm">
          {token ? t.publicBooking.paymentFailedHeldDescription : t.publicBooking.paymentFailedDescription}
        </p>
      </div>

      {booking && (
        <div className="border-border-subtle bg-bg-subtle w-full rounded-2xl border p-4 text-left">
          <p className="text-text-tertiary text-xs font-semibold tracking-wide uppercase">
            {t.publicBooking.paymentFailedSlotLabel}
          </p>
          <p className="text-text-primary mt-2 text-sm font-semibold whitespace-nowrap">{keepTogether(booking.courtName)}</p>
          <p className="text-text-secondary mt-0.5 text-sm">
            {formatDateFull(booking.date)} · {formatHourRange(booking.startsAt, booking.endsAt, ' a ')}
          </p>
          <p className="text-text-secondary mt-0.5 text-sm tabular-nums">{formatPrice(booking.price)}</p>
        </div>
      )}

      <div className="flex w-full flex-col gap-2">
        {token ? (
          <FailedCheckoutActions slug={slug} token={token} />
        ) : (
          <Button asChild size="lg" className="min-h-12">
            <a href={failedSlotPath(slug, booking)}>{t.publicBooking.paymentFailedChooseAnother}</a>
          </Button>
        )}
        {booking && whatsAppHref && (
          <Button asChild variant="ghost" className="min-h-12">
            <a
              href={whatsAppHref}
              target="_blank"
              rel="noopener noreferrer"
              aria-label={`${t.publicBooking.writeOnWhatsAppTo} ${booking.complexName}`}
            >
              {t.publicBooking.paymentFailedWhatsApp}
            </a>
          </Button>
        )}
        {booking && (
          <Button asChild variant="ghost" className="min-h-12">
            <a href={`tel:${booking.complexPhone}`}>{t.publicBooking.paymentFailedCallComplex}</a>
          </Button>
        )}
      </div>
    </div>
  );
}

/** Pay again on the same checkout, or give the slot back and pick another time. */
function FailedCheckoutActions({ slug, token }: { slug: string; token: string }) {
  const { gone, inProcess, failed, busy, retry, release } = useFailedCheckout(slug, token);

  return (
    <>
      {gone && (
        <p role="alert" className="text-text-secondary text-sm">
          {t.publicBooking.paymentFailedRetryGone}
        </p>
      )}
      {inProcess && (
        <p role="alert" className="text-text-secondary text-sm">
          {inProcess}
        </p>
      )}
      {failed && (
        <p role="alert" className="text-error-text text-sm">
          {t.publicBooking.paymentFailedActionError}
        </p>
      )}
      {!gone && (
        <Button size="lg" className="min-h-12" disabled={busy} onClick={retry}>
          {t.publicBooking.paymentFailedRetry}
        </Button>
      )}
      <Button size="lg" variant={gone ? 'default' : 'outline'} className="min-h-12" disabled={busy} onClick={release}>
        {t.publicBooking.paymentFailedChooseAnother}
      </Button>
      <Button asChild variant="ghost" className="min-h-12">
        <a href={publicComplexPath(slug)}>{t.publicBooking.paymentFailedBackToComplex}</a>
      </Button>
    </>
  );
}
