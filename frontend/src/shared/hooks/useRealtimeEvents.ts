import { useEffect, useRef, useCallback, type RefObject } from 'react';
import { useQueryClient, type QueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { queryKeys } from '@/shared/lib/queryKeys';
import { bootstrapSession, refreshAccessToken } from '@/shared/lib/ky';
import { env } from '@/shared/lib/env';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

const MAX_RETRIES = 3;
const RETRY_DELAY = 3_000;

/**
 * Why the stream closed, as told by the server's named terminal event
 * (`event: expired` / `event: unauthorized`, see `internal/realtime/stream.go`
 * on the server) — set by that event's listener just before the connection
 * drops and consumed once by the `onerror` handler that follows it.
 * `null` means the connection dropped without one, i.e. an ordinary network
 * failure.
 */
type StreamCloseReason = 'expired' | 'unauthorized' | null;

/**
 * Closes the failed connection and schedules a reconnect.
 *
 * An ordinary drop (the proxy cut the stream, the network blinked, the
 * server restarted) says nothing about the session, so it must not spend
 * the refresh token: `bootstrapSession` reads `/auth/me` and rotates only
 * if that answers 401. Refreshing unconditionally here rotated the token on
 * every drop while the access token was still valid; a rotation the page
 * never got to store (a drop right before it closed) then left the next
 * document presenting a spent token, which the server treats as theft
 * after its concurrent-refresh grace and revokes every session on it.
 */
function handleSSEError(
  es: EventSource,
  esRef: RefObject<EventSource | null>,
  retriesRef: RefObject<number>,
  retryTimerRef: RefObject<ReturnType<typeof setTimeout> | undefined>,
  connectRef: RefObject<() => void>,
) {
  es.close();
  esRef.current = null;

  if (retriesRef.current >= MAX_RETRIES) return;
  retriesRef.current++;

  const scheduleReconnect = () => {
    retryTimerRef.current = setTimeout(() => {
      connectRef.current();
    }, RETRY_DELAY);
  };

  bootstrapSession()
    .then((session) => {
      // `null` is definitive: the access token is gone and the refresh was
      // refused. Signed out, stop retrying.
      if (!session) return;
      scheduleReconnect();
    })
    .catch(() => {
      // The probe itself failed (a 5xx, a network error): that says nothing
      // about the session, and it is the same blip that dropped the stream.
      // Keep retrying within the budget above.
      scheduleReconnect();
    });
}

/**
 * Reconnects right away after a server-initiated stream rotation
 * (`event: expired`). This is not a failure — the server closes the stream on
 * a schedule tied to the access token's own lifetime — so it skips the retry
 * budget and backoff delay that `handleSSEError` applies to genuine
 * connection failures.
 */
function handleStreamExpired(
  es: EventSource,
  esRef: RefObject<EventSource | null>,
  retriesRef: RefObject<number>,
  connectRef: RefObject<() => void>,
) {
  es.close();
  esRef.current = null;
  retriesRef.current = 0;

  refreshAccessToken()
    .then(() => {
      connectRef.current();
    })
    .catch(() => {
      // Refresh failed — session expired, stop retrying.
    });
}

/**
 * Stops for good after the server tells us this caller is no longer
 * authorized (`event: unauthorized`) — reconnecting would just be denied
 * again. Surfaces it once so the dashboard doesn't silently go stale.
 */
function handleStreamUnauthorized(
  es: EventSource,
  esRef: RefObject<EventSource | null>,
  retryTimerRef: RefObject<ReturnType<typeof setTimeout> | undefined>,
) {
  clearTimeout(retryTimerRef.current);
  es.close();
  esRef.current = null;
  toast.error(t.dashboard.realtimeAccessEnded);
}

/** The mutable state `wireStreamListeners` needs to react to what the stream tells it. */
interface StreamRefs {
  esRef: RefObject<EventSource | null>;
  retriesRef: RefObject<number>;
  retryTimerRef: RefObject<ReturnType<typeof setTimeout> | undefined>;
  connectRef: RefObject<() => void>;
  lastCloseReasonRef: RefObject<StreamCloseReason>;
  unloadingRef: RefObject<boolean>;
}

/**
 * Registers every listener a stream needs: cache invalidation on
 * `booking_changed`, recording why on the two named terminal events, and
 * dispatching on the `onerror` that follows any of them (or an ordinary
 * network failure) to the right handler above.
 */
function wireStreamListeners(es: EventSource, complexId: string, queryClient: QueryClient, refs: StreamRefs) {
  es.addEventListener('booking_changed', () => {
    void Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeys.bookings.byComplex(complexId) }),
      queryClient.invalidateQueries({ queryKey: queryKeys.dashboard.stats(complexId) }),
      queryClient.invalidateQueries({ queryKey: queryKeys.dashboard.occupancy(complexId) }),
      queryClient.invalidateQueries({ queryKey: queryKeys.dashboard.clients(complexId) }),
    ]);
  });

  // Named terminal events arrive as ordinary SSE messages before the server
  // closes the connection, which is what actually triggers `onerror` below —
  // so these just record why, for `onerror` to read.
  es.addEventListener('expired', () => {
    refs.lastCloseReasonRef.current = 'expired';
  });
  es.addEventListener('unauthorized', () => {
    refs.lastCloseReasonRef.current = 'unauthorized';
  });

  es.onopen = () => {
    refs.retriesRef.current = 0;
  };

  es.onerror = () => {
    // The browser drops the stream when the document unloads, and that
    // arrives here as an ordinary error. Refreshing from a dying document
    // rotates the refresh token on the server while the response is thrown
    // away with the page, so the next document presents a stale token and
    // is signed out.
    if (refs.unloadingRef.current) {
      es.close();
      refs.esRef.current = null;
      return;
    }
    const reason = refs.lastCloseReasonRef.current;
    refs.lastCloseReasonRef.current = null;

    if (reason === 'unauthorized') {
      handleStreamUnauthorized(es, refs.esRef, refs.retryTimerRef);
      return;
    }
    if (reason === 'expired') {
      handleStreamExpired(es, refs.esRef, refs.retriesRef, refs.connectRef);
      return;
    }
    handleSSEError(es, refs.esRef, refs.retriesRef, refs.retryTimerRef, refs.connectRef);
  };
}

/**
 * Connects to the SSE endpoint for the given complex and invalidates
 * React Query caches when booking data changes on the server.
 * Pauses connection when tab is hidden to save resources.
 * Automatically refreshes the access token on auth failure.
 *
 * The server may close the stream with a named terminal event before it
 * drops the connection: `expired` (routine rotation or a re-check that
 * couldn't be completed — reconnecting is correct) and `unauthorized` (the
 * periodic re-authorization denied this caller — reconnecting is not). Both
 * are followed by an ordinary `onerror`, so `lastCloseReasonRef` records
 * which one preceded it for `onerror` to act on.
 */
export function useRealtimeEvents(complexId: string | null) {
  const queryClient = useQueryClient();
  const esRef = useRef<EventSource | null>(null);
  const retriesRef = useRef(0);
  const retryTimerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const lastCloseReasonRef = useRef<StreamCloseReason>(null);
  const unloadingRef = useRef(false);
  // Holds the latest `connect` so the retry timer below can call it without
  // referencing `connect` inside its own closure (self-recursive `useCallback`
  // bodies aren't supported by the React Compiler).
  const connectRef = useRef<() => void>(() => {
    /* set after `connect` is defined below */
  });

  const connect = useCallback(() => {
    if (!complexId || esRef.current) return;

    const url = `${env.VITE_API_URL}/complexes/${complexId}/events`;
    const es = new EventSource(url, { withCredentials: true });
    esRef.current = es;
    lastCloseReasonRef.current = null;

    wireStreamListeners(es, complexId, queryClient, {
      esRef,
      retriesRef,
      retryTimerRef,
      connectRef,
      lastCloseReasonRef,
      unloadingRef,
    });
  }, [complexId, queryClient]);

  useEffect(() => {
    connectRef.current = connect;
  }, [connect]);

  const disconnect = useCallback(() => {
    clearTimeout(retryTimerRef.current);
    if (esRef.current) {
      esRef.current.close();
      esRef.current = null;
    }
  }, []);

  useEffect(() => {
    if (!complexId) return;

    if (!document.hidden) connect();

    const handleVisibility = () => {
      if (document.hidden) {
        disconnect();
      } else {
        retriesRef.current = 0;
        connect();
      }
    };

    document.addEventListener('visibilitychange', handleVisibility);
    // Close the stream ourselves before the browser tears the document down,
    // so its `onerror` never runs a token refresh on the way out (see
    // `wireStreamListeners`).
    const handlePageHide = () => {
      unloadingRef.current = true;
      disconnect();
    };
    window.addEventListener('pagehide', handlePageHide);

    return () => {
      disconnect();
      document.removeEventListener('visibilitychange', handleVisibility);
      window.removeEventListener('pagehide', handlePageHide);
    };
  }, [complexId, connect, disconnect]);
}
