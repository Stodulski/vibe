import { useMutation } from '@tanstack/react-query';
import { usePublicBookingApi } from '../api/context';
import type { WaitlistJoinRequest } from '../api/types';

/**
 * Puts an email on the waitlist of one day. No retries: the request is a POST,
 * and the form decides what a failure means to the visitor.
 */
export function useJoinWaitlist(slug: string) {
  const api = usePublicBookingApi();
  return useMutation({
    mutationFn: (data: WaitlistJoinRequest) => api.joinWaitlist(slug, data),
  });
}
