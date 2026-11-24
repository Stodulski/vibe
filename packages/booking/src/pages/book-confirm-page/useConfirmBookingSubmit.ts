import { useEffect, useState } from 'react';
import { usePublicBooking } from '../../hooks/usePublicBooking';
import { BOOKING_INFO_KEY } from '../../lib/storedBookingInfo';
import type { PublicBookingFormData } from '../../schemas/public-booking.schema';
import { buildBookingRequest } from './buildBookingRequest';
import { buildBookingInfo } from './buildBookingInfo';
import { safeSessionStorage } from '@vibe/ui';
import { publicComplexPath } from '../../lib/publicPaths';
import { navigateTo } from '../../lib/navigation';
import { clearBookingResult, clearConfirmDraft, saveBookingResult, type ConfirmDraft } from '../../lib/handoff';

/** What the submit tells the page's hold about. See `useSlotHold`. */
export interface ConfirmHoldEvents {
  /** The server says the hold is gone: back to the hours with the expired notice. */
  onExpired: () => void;
  /** The booking converted the hold; leaving the page now must keep it. */
  onConverted: () => void;
  /** A submit started (`true`) or failed (`false`): while one runs, a reclaim's answer is ignored. */
  onSubmitting: (active: boolean) => void;
}

/**
 * Where a visitor sent to MercadoPago lands when this page shows again.
 *
 * MercadoPago sends the browser to the success page by itself (the back_urls,
 * backend/internal/booklink). This page shows again instead when the visitor
 * returns another way: the browser's back restores it from the back/forward
 * cache, or MercadoPago's app took the payment and the visitor switched back to
 * this tab, which never left. Either way it would sit on "Abriendo Mercado
 * Pago..." for good, so it moves to the success page with the booking's
 * credential, which reads the booking's real state: confirmed, still being
 * processed, or not paid.
 */
function useReturnFromCheckout(slug: string | undefined, checkoutToken: string | null) {
  useEffect(() => {
    if (checkoutToken === null) return;
    const outcome = `${publicComplexPath(String(slug))}/book/success?token=${encodeURIComponent(checkoutToken)}`;
    let hidden = false;
    let done = false;
    const showOutcome = () => {
      if (done) return;
      done = true;
      navigateTo(outcome, { replace: true });
    };
    const onPageShow = (event: PageTransitionEvent) => {
      if (event.persisted) showOutcome();
    };
    const onVisibilityChange = () => {
      if (document.visibilityState === 'hidden') hidden = true;
      else if (hidden) showOutcome();
    };
    window.addEventListener('pageshow', onPageShow);
    document.addEventListener('visibilitychange', onVisibilityChange);
    return () => {
      window.removeEventListener('pageshow', onPageShow);
      document.removeEventListener('visibilitychange', onVisibilityChange);
    };
  }, [slug, checkoutToken]);
}

export function useConfirmBookingSubmit(
  slug: string | undefined,
  slotInfo: ConfirmDraft,
  hold: ConfirmHoldEvents,
) {

  // The booking's credential once the browser was sent to MercadoPago.
  const [checkoutToken, setCheckoutToken] = useState<string | null>(null);
  const redirecting = checkoutToken !== null;
  useReturnFromCheckout(slug, checkoutToken);
  const [paymentLinkError, setPaymentLinkError] = useState(false);
  // Kept so "Reintentar" can resubmit exactly what the person already typed —
  // the whole point of the persistent error is that their data is not lost.
  const [lastFormData, setLastFormData] = useState<PublicBookingFormData | null>(null);

  const mutation = usePublicBooking({
    onConflict: () => {
      // Slot was taken — redirect back to slot selection with date pre-selected.
      clearConfirmDraft(String(slug));
      navigateTo(`${publicComplexPath(String(slug))}?date=${slotInfo.date}`, { replace: true });
    },
    onPaymentLinkError: () => {
      setPaymentLinkError(true);
    },
    onHoldExpired: hold.onExpired,
  });

  function handleSubmit(formData: PublicBookingFormData) {
    setPaymentLinkError(false);
    setLastFormData(formData);
    hold.onSubmitting(true);
    mutation.mutate(buildBookingRequest(slotInfo, formData, slotInfo.holdToken), {
      onError: () => {
        hold.onSubmitting(false);
      },
      onSuccess: (data) => {
        hold.onConverted();
        const bookingInfo = buildBookingInfo(slotInfo, formData.client_phone);

        // Persist bookingInfo so it survives the MP redirect. Best-effort: in
        // Safari private browsing (or a full quota) the write fails silently
        // and the summary is lost, but the booking already exists on the
        // server; getting stuck on this screen with a blocked slot would be
        // worse.
        safeSessionStorage.set(BOOKING_INFO_KEY, JSON.stringify(bookingInfo));
        clearConfirmDraft(String(slug));

        if (data.mp_init_point) {
          // A result left by an earlier booking must not answer for this one
          // when MercadoPago brings the browser back.
          clearBookingResult(String(slug));
          setCheckoutToken(data.token);
          window.location.href = data.mp_init_point;
        } else {
          saveBookingResult(String(slug), { token: data.token, bookingInfo });
          navigateTo(`${publicComplexPath(String(slug))}/book/success`, { replace: true });
        }
      },
    });
  }

  function retry() {
    if (lastFormData) handleSubmit(lastFormData);
  }

  return { handleSubmit, isLoading: mutation.isPending, redirecting, paymentLinkError, retry };
}
