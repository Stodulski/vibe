import { useMutation } from '@tanstack/react-query';
import { usePublicBookingApi } from '../api/context';
import type { WaitlistTarget } from '../api/types';

/** Puts an email on the waitlist of one place, with the Turnstile token when there is one. */
export function useJoinPlaceWaitlist(slug: string) {
  const api = usePublicBookingApi();
  return useMutation({
    mutationFn: ({ target, email, token }: { target: WaitlistTarget; email: string; token?: string | undefined }) =>
      api.joinPlaceWaitlist(slug, { ...target, email, ...(token ? { turnstile_token: token } : {}) }),
  });
}
