import { useEffect, useState } from 'react';
import { useQuery, type UseQueryResult } from '@tanstack/react-query';
import { HTTPError } from 'ky';
import { publicBookingApi } from '../api/public-booking.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ApiResponseError } from '@/shared/lib/apiParse';
import type { BookingStatusDetails } from '@/shared/types/api.types';

const TERMINAL_STATUSES = ['confirmed', 'cancelled', 'completed'];
const POLLING_TIMEOUT_MS = 120_000; // 120 seconds

type BookingStatusResult = UseQueryResult<BookingStatusDetails | undefined> & {
  timedOut: boolean;
  /** `resolveLink` (`internal/bookings/public.go`) answered 410: the token resolved but the link is no longer live. */
  linkExpired: boolean;
  /** `resolveLink` answered 404: the token never existed. */
  linkNotFound: boolean;
};

export function useBookingStatus(token: string | null): BookingStatusResult {
  // Reset `timedOut` when `token` changes, without a `setState`-in-effect:
  // this is React's documented "adjust state during render" pattern for
  // resetting derived state on a prop change.
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

  // `enabled` gates the actual fetch; a real token is never missing
  // when `queryFn` actually runs — the '' fallback is a type-level-only
  // placeholder, never observed by the API.
  const safeToken = token ?? '';
  const query = useQuery({
    queryKey: queryKeys.bookingStatus.byId(safeToken),
    queryFn: ({ signal }) => publicBookingApi.getBookingStatus(safeToken, signal),
    select: (data) => data.booking,
    enabled: !!token,
    retry: (failureCount, error) => {
      // 404/410 are permanent — the token will never resolve differently, so
      // retrying only delays the client seeing the right message. A schema
      // rejection (`ApiResponseError`) is the same kind of permanent failure:
      // the server just answered with a shape this client doesn't understand,
      // and retrying the same request gets the same answer.
      if (error instanceof HTTPError && (error.response.status === 404 || error.response.status === 410)) {
        return false;
      }
      if (error instanceof ApiResponseError) return false;
      return failureCount < 1;
    },
    refetchInterval: (q) => {
      if (timedOut) return false;
      const err = q.state.error;
      if (err instanceof HTTPError && (err.response.status === 404 || err.response.status === 410)) return false;
      // Without this, a malformed response keeps the interval alive forever:
      // every 2s the query refetches, gets the same shape, throws the same
      // `ApiResponseError` again, and the caller never sees anything but the
      // "still processing" state — this is what stops that loop and lets the
      // query settle into (and stay in) its terminal error state instead.
      if (err instanceof ApiResponseError) return false;
      const status = q.state.data?.booking.status;
      if (status && TERMINAL_STATUSES.includes(status)) return false;
      return 2000;
    },
  });

  const linkExpired = query.error instanceof HTTPError && query.error.response.status === 410;
  const linkNotFound = query.error instanceof HTTPError && query.error.response.status === 404;

  return { ...query, timedOut, linkExpired, linkNotFound };
}
