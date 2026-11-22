import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { HTTPError } from 'ky';
import { usePublicBookingApi } from '../../api/context';
import { getProblem } from '../../lib/ApiError';
import { navigateTo } from '../../lib/navigation';
import { publicComplexPath } from '../../lib/publicPaths';
import { BOOKING_MESSAGES } from '../../messages';

/** The 409 kind the checkout routes answer when the booking is paid. */
const BOOKING_PAID_KIND = 'booking-paid';
/** The 409 kind they answer while MercadoPago is still processing a payment. */
const PAYMENT_IN_PROCESS_KIND = 'payment-in-process';

function conflictKind(error: unknown): string | undefined {
  if (!(error instanceof HTTPError) || error.response.status !== 409) return undefined;
  return getProblem(error)?.kind ?? 'conflict';
}

/**
 * What the failure page can do with the booking a rejected payment left
 * behind, through the link token MercadoPago sent back: pay again on the same
 * checkout, or give the slot back and pick another time.
 *
 * `gone` is set once the server says the booking can no longer be paid; from
 * then on only another time is offered. `inProcess` holds the server's words
 * while MercadoPago is still processing a payment: neither action can go
 * ahead until it settles. Only a booking the server calls paid (`booking-paid`)
 * goes to the booking's own page; any other refused or failed release leaves
 * `stuck` set, and the page links back to the complex.
 */
export function useFailedCheckout(slug: string, token: string) {
  const api = usePublicBookingApi();
  const [gone, setGone] = useState(false);
  const [inProcess, setInProcess] = useState<string | null>(null);
  const showBooking = () => {
    navigateTo(`${publicComplexPath(slug)}/book/success?token=${encodeURIComponent(token)}`);
  };
  const handledConflict = (error: unknown): boolean => {
    const kind = conflictKind(error);
    if (kind === BOOKING_PAID_KIND) {
      showBooking();
      return true;
    }
    if (kind === PAYMENT_IN_PROCESS_KIND) {
      setInProcess(getProblem(error)?.detail ?? BOOKING_MESSAGES.publicBooking.paymentFailedInProcess);
      return true;
    }
    return false;
  };

  const retry = useMutation({
    mutationFn: () => api.retryCheckout(token),
    onSuccess: ({ initPoint }) => {
      navigateTo(initPoint);
    },
    onError: (error) => {
      if (handledConflict(error)) return;
      if (conflictKind(error)) setGone(true);
    },
  });

  const release = useMutation({
    mutationFn: () => api.releaseCheckout(token),
    onSuccess: () => {
      navigateTo(publicComplexPath(slug));
    },
    onError: (error) => {
      handledConflict(error);
    },
  });

  const handled = (error: unknown) => {
    const kind = conflictKind(error);
    return kind === BOOKING_PAID_KIND || kind === PAYMENT_IN_PROCESS_KIND;
  };
  const stuck = release.isError && !handled(release.error);
  const failed = (retry.isError && !gone && !handled(retry.error)) || stuck;

  return {
    gone,
    inProcess,
    failed,
    stuck,
    busy: retry.isPending || release.isPending,
    retry: () => {
      setInProcess(null);
      retry.mutate();
    },
    release: () => {
      setInProcess(null);
      release.mutate();
    },
  };
}
