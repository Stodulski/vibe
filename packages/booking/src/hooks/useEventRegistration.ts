import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { HTTPError } from 'ky';
import { toast } from 'sonner';
import { usePublicBookingApi } from '../api/context';
import { BOOKING_MESSAGES } from '../messages';
import { getHttpErrorMessage } from '../lib/httpErrors';
import { ApiResponseError } from '../lib/apiParse';
import { queryKeys } from '../lib/queryKeys';

const POLLING_TIMEOUT_MS = 120_000;
const POLL_INTERVAL_MS = 2000;

function isFinalHttpError(error: unknown): boolean {
  return error instanceof HTTPError && (error.response.status === 404 || error.response.status === 410);
}

/**
 * A person's own spot, read through the token the sign-up returned. It polls
 * while the spot is still held, because the payment confirms it by webhook a
 * moment after the checkout returns; every other status is terminal.
 */
export function useEventRegistration(token: string | null) {
  const api = usePublicBookingApi();
  const [prevToken, setPrevToken] = useState(token);
  const [timedOut, setTimedOut] = useState(false);
  if (token !== prevToken) {
    setPrevToken(token);
    setTimedOut(false);
  }

  useEffect(() => {
    if (!token) return;
    const timer = setTimeout(() => {
      setTimedOut(true);
    }, POLLING_TIMEOUT_MS);
    return () => {
      clearTimeout(timer);
    };
  }, [token]);

  const safeToken = token ?? '';
  const query = useQuery({
    queryKey: queryKeys.eventRegistration.byToken(safeToken),
    queryFn: ({ signal }) => api.getEventRegistration(safeToken, signal),
    enabled: !!token,
    throwOnError: false,
    retry: (failureCount, error) => {
      if (isFinalHttpError(error) || error instanceof ApiResponseError) return false;
      return failureCount < 1;
    },
    refetchInterval: (q) => {
      if (timedOut) return false;
      if (isFinalHttpError(q.state.error) || q.state.error instanceof ApiResponseError) return false;
      if (q.state.data && q.state.data.status !== 'held') return false;
      return POLL_INTERVAL_MS;
    },
  });

  const linkNotFound = isFinalHttpError(query.error);
  return { ...query, timedOut, linkNotFound };
}

/** Cancels the person's own spot. The server's refund message is shown as it comes. */
export function useCancelEventRegistration(token: string) {
  const api = usePublicBookingApi();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => api.cancelEventRegistration({ token }),
    onSuccess: (data) => {
      queryClient.setQueryData(queryKeys.eventRegistration.byToken(token), data.registration);
    },
    onError: (error: unknown) => {
      toast.error(getHttpErrorMessage(error, BOOKING_MESSAGES.events.cancelError));
    },
  });
}
