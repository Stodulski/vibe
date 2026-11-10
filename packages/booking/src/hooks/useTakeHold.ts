import { useMutation } from '@tanstack/react-query';
import { usePublicBookingApi } from '../api/context';
import type { PublicHoldRequest } from '../api/types';

/**
 * Holds a slot (`POST /book/holds`). No retries: the request spends a
 * Turnstile token and takes a hold, so the caller decides what a failure means.
 */
export function useTakeHold() {
  const api = usePublicBookingApi();
  return useMutation({
    mutationFn: (data: PublicHoldRequest) => api.createHold(data),
  });
}
