import { MutationCache, QueryCache, QueryClient } from '@tanstack/react-query';
import type { BookingConfig } from '../config';

type ErrorReporter = NonNullable<BookingConfig['reportError']>;

/**
 * The reporter of the most recently mounted `BookingRoot`. The browser client is
 * created once, so its caches cannot capture a config directly; they forward to
 * whichever config mounted last.
 */
let activeReporter: ErrorReporter | undefined;
let browserClient: QueryClient | undefined;

function forwardError(error: unknown): void {
  activeReporter?.(error);
}

/**
 * A query client with the defaults of the app's own client: five minutes fresh,
 * ten minutes cached, no focus refetch. Retries are off because the HTTP client
 * already retries idempotent requests. Failures never reach an error boundary,
 * so every screen renders its own error state.
 */
export function createBookingQueryClient(): QueryClient {
  return new QueryClient({
    queryCache: new QueryCache({ onError: forwardError }),
    mutationCache: new MutationCache({ onError: forwardError }),
    defaultOptions: {
      queries: {
        staleTime: 5 * 60 * 1000,
        gcTime: 10 * 60 * 1000,
        retry: false,
        throwOnError: false,
        refetchOnWindowFocus: false,
      },
      mutations: { retry: 0 },
    },
  });
}

/**
 * The browser singleton, so the cache survives a remount of `BookingRoot`. On the
 * server every call gets a new client, so nothing is shared between requests.
 */
export function getBookingQueryClient(reporter: ErrorReporter | undefined): QueryClient {
  activeReporter = reporter;
  if (typeof window === 'undefined') {
    return createBookingQueryClient();
  }
  browserClient ??= createBookingQueryClient();
  return browserClient;
}
