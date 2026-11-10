import { useState } from 'react';
import { HTTPError } from 'ky';
import { usePublicBooking } from '../../hooks/usePublicBooking';
import { BOOKING_INFO_KEY } from '../../lib/storedBookingInfo';
import type { BookingSlotInfo } from '../../components/BookingForm';
import type { PublicBookingFormData } from '../../schemas/public-booking.schema';
import { buildBookingRequest } from './buildBookingRequest';
import { buildBookingInfo } from './buildBookingInfo';
import { isStillInProgress, safeSessionStorage } from '@vibe/ui';
import { publicComplexPath } from '../../lib/publicPaths';
import { navigateTo } from '../../lib/navigation';
import { clearBookingResult, clearConfirmDraft, saveBookingResult } from '../../lib/handoff';

/** The Turnstile challenge as the submit sees it. Omitted, or `required: false`, when the storefront has none. */
export interface ConfirmTurnstile {
  required: boolean;
  token: string | undefined;
  /** Throws the spent token away and asks the widget for a fresh one. */
  reset: () => void;
}

export function useConfirmBookingSubmit(
  slug: string | undefined,
  slotInfo: BookingSlotInfo,
  turnstile?: ConfirmTurnstile,
) {
  const submitBlocked = Boolean(turnstile?.required && !turnstile.token);

  const [redirecting, setRedirecting] = useState(false);
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
  });

  function handleSubmit(formData: PublicBookingFormData) {
    // The button is already disabled; this also covers Enter in a field.
    if (submitBlocked) return;
    setPaymentLinkError(false);
    setLastFormData(formData);
    mutation.mutate(buildBookingRequest(slotInfo, formData, turnstile?.token), {
      onError: (error) => {
        // Any HTTP answer spent the token, so the next attempt needs a fresh
        // one. Two cases keep it, because keeping the payload keeps the
        // Idempotency-Key, and the retry must reuse that key to receive the
        // first attempt's answer: a timeout or network failure (no answer at
        // all), and a 409 "still in progress" (the first request is still
        // running on the server).
        if (error instanceof HTTPError && !isStillInProgress(error)) turnstile?.reset();
      },
      onSuccess: (data) => {
        turnstile?.reset();
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
          setRedirecting(true);
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

  return { handleSubmit, isLoading: mutation.isPending, submitBlocked, redirecting, paymentLinkError, retry };
}
