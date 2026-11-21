import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { HTTPError } from 'ky';
import { usePublicBookingApi } from '../../api/context';
import { navigateTo } from '../../lib/navigation';
import { publicComplexPath } from '../../lib/publicPaths';

function isConflict(error: unknown): boolean {
  return error instanceof HTTPError && error.response.status === 409;
}

/**
 * What the failure page can do with the booking a rejected payment left
 * behind, through the link token MercadoPago sent back: pay again on the same
 * checkout, or give the slot back and pick another time.
 *
 * `gone` is set once the server says the booking can no longer be paid; from
 * then on only another time is offered. A release refused because the booking
 * was paid after all goes to the booking's own page, which shows the truth.
 */
export function useFailedCheckout(slug: string, token: string) {
  const api = usePublicBookingApi();
  const [gone, setGone] = useState(false);

  const retry = useMutation({
    mutationFn: () => api.retryCheckout(token),
    onSuccess: ({ initPoint }) => {
      navigateTo(initPoint);
    },
    onError: (error) => {
      if (isConflict(error)) setGone(true);
    },
  });

  const release = useMutation({
    mutationFn: () => api.releaseCheckout(token),
    onSuccess: () => {
      navigateTo(publicComplexPath(slug));
    },
    onError: (error) => {
      if (isConflict(error)) {
        navigateTo(`${publicComplexPath(slug)}/book/success?token=${encodeURIComponent(token)}`);
      }
    },
  });

  const failed = (retry.isError && !gone) || (release.isError && !isConflict(release.error));

  return {
    gone,
    failed,
    busy: retry.isPending || release.isPending,
    retry: () => {
      retry.mutate();
    },
    release: () => {
      release.mutate();
    },
  };
}
