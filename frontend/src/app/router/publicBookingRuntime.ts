import { HTTPError, NetworkError, TimeoutError } from 'ky';
import { env } from '@/shared/lib/env';
import { captureException } from '@/shared/lib/observability';
import { ApiError } from '@vibe/booking/api-errors';
import type { BookingConfig } from '@vibe/booking/config';

/**
 * The statuses `reportQueryError` in `shared/lib/queryClient.ts` leaves out:
 * an anonymous visit, a role boundary, a stale link, a rejected form. Mirrored
 * here because that function is private and the booking client has its own
 * query cache.
 */
const EXPECTED_STATUSES = new Set([401, 403, 404, 422]);

/**
 * Sends a booking failure to Sentry, with the same filter and tags the app's
 * query client uses, so the booking pages report exactly what they did before.
 * A render error caught by a booking boundary arrives with its component stack
 * and is tagged with the route, as the app's own boundary tags it.
 */
export function reportBookingError(error: unknown, context?: Record<string, unknown>): void {
  if (context && 'componentStack' in context) {
    captureException(error, {
      tags: { route: window.location.pathname },
      extra: { componentStack: context.componentStack },
    });
    return;
  }

  if (error instanceof HTTPError) {
    if (EXPECTED_STATUSES.has(error.response.status)) return;

    const requestId = error instanceof ApiError ? error.requestId : error.response.headers.get('X-Request-ID');
    captureException(error, {
      tags: {
        status: error.response.status,
        pathname: new URL(error.request.url).pathname,
        ...(requestId ? { request_id: requestId } : {}),
      },
    });
    return;
  }

  if (error instanceof TimeoutError || error instanceof NetworkError) {
    captureException(error);
  }
}

let cached: BookingConfig | undefined;

/**
 * The config the booking pages run with on this app's origin. The API base is
 * resolved to an absolute URL, and the privacy link keeps the exact bytes the
 * booking form used before the feature had its own config.
 */
export function getPublicBookingConfig(): BookingConfig {
  cached ??= {
    apiBaseUrl: new URL(env.VITE_API_URL, window.location.origin).href,
    publicSiteUrl: env.VITE_APP_URL,
    privacyUrl: new URL('/privacidad', env.VITE_LANDING_URL).href,
    reportError: reportBookingError,
  };
  return cached;
}
