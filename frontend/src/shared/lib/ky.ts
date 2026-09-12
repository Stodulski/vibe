import ky, { HTTPError, isHTTPError } from 'ky';
import type { Options, RetryOptions } from 'ky';
import { ApiError } from '@/shared/lib/ApiError';
import { useStore } from '@/shared/stores';
import { env } from '@/shared/lib/env';
import { parseWith } from '@/shared/lib/apiParse';
import { currentUserResponseSchema, refreshResponseSchema } from '@/shared/schemas/auth.schema';
import type { CurrentUserResponse, RefreshResponse } from '@/shared/types/api.types';

let refreshPromise: Promise<void> | null = null;

/**
 * How long to wait before the one retry of a failed refresh.
 *
 * Two tabs share the cookie jar. When the access token expires, each tab's
 * first 401 starts its own refresh; the request that leaves second still
 * carries the refresh token the first one is about to spend, and the server
 * refuses it. By the time this delay is over, the sibling's response has put
 * the new cookie in the jar, so the retry sends that one and succeeds — and
 * this tab gets its own access token and CSRF token from it. A refresh token
 * that was really revoked fails the retry too, and only then is the session
 * treated as gone.
 */
export const REFRESH_RETRY_DELAY_MS = 600;

/**
 * A malformed refresh body (e.g. missing `csrf_token`) is validated here and
 * turned into an `ApiResponseError` thrown out of `.json()`'s `.then()`
 * chain, rather than being read as-is — `refreshAccessToken` below has no
 * other check on `data`, so without this an absent `csrf_token` would flow
 * straight into `setCsrfToken(undefined)` and silently leave every later
 * request with no CSRF header. Throwing here instead means a malformed
 * refresh takes the exact same path as a rejected (401) refresh: the retry
 * in `refreshAccessToken`'s `catch`, and — if the retry is malformed too —
 * the caller's own catch in `ky.ts`'s `afterResponse` hook, which logs the
 * user out and redirects to `/login`.
 */
function postRefresh(): Promise<RefreshResponse> {
  return ky
    .post('auth/refresh', {
      prefix: env.VITE_API_URL,
      credentials: 'include',
    })
    .json()
    .then(parseWith(refreshResponseSchema, 'ky.postRefresh'));
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => {
    setTimeout(resolve, ms);
  });
}

export async function refreshAccessToken(): Promise<void> {
  if (refreshPromise) return refreshPromise;

  refreshPromise = (async () => {
    let data: RefreshResponse;
    try {
      data = await postRefresh();
    } catch {
      await delay(REFRESH_RETRY_DELAY_MS);
      data = await postRefresh();
    }

    useStore.getState().setCsrfToken(data.csrf_token);
  })();

  try {
    await refreshPromise;
    return;
  } finally {
    refreshPromise = null;
  }
}

/**
 * Reads the current session the way a fresh document load needs it: without
 * spending the refresh token.
 *
 * `GET /auth/me` answers the user together with the CSRF token bound to the
 * access token the cookie carries, so while that access token is alive a
 * page load rotates nothing. Only a 401 (the access token expired) leads to
 * `refreshAccessToken`, after which the read is repeated with the new cookie.
 * That shrinks the window in which a navigation can abort a refresh whose
 * rotation the server already committed, from every page load to at most one
 * per access-token lifetime.
 *
 * Like `postRefresh`, this bypasses the `api` instance and its hooks: the 401
 * hook below logs the visitor out and sends them to `/login`, which is the
 * wrong answer for an anonymous boot on a public or guest page. Here a 401
 * that no refresh can recover is simply "no session", answered as `null`;
 * the store is left to the caller. Any other failure propagates.
 */
export async function bootstrapSession(signal?: AbortSignal): Promise<CurrentUserResponse | null> {
  try {
    return await getCurrentUser(signal);
  } catch (error) {
    if (!(error instanceof HTTPError) || error.response.status !== 401) {
      throw error;
    }
  }

  try {
    await refreshAccessToken();
  } catch {
    return null;
  }

  return getCurrentUser(signal);
}

function getCurrentUser(signal?: AbortSignal): Promise<CurrentUserResponse> {
  return ky
    .get('auth/me', {
      prefix: env.VITE_API_URL,
      credentials: 'include',
      ...withSignal(signal),
    })
    .json()
    .then(parseWith(currentUserResponseSchema, 'ky.getCurrentUser'));
}

/**
 * Where a session that could not be refreshed has to send the browser.
 *
 * This is a hard navigation — the store is gone and React never gets to
 * render a `<Navigate>` — so the destination cannot ride along in router
 * state the way `ProtectedRoute`'s does. It goes in the query string
 * instead, and `useAuthSuccessHandler` reads it back after the next login
 * and validates it against the same allowlist, so an expired token no
 * longer costs the person the page they were on.
 *
 * Returns `null` when there is nothing to do: already on `/login` or
 * `/register`, where a redirect would only wipe a half-typed form.
 */
export function loginUrlPreserving(location: { pathname: string; search: string }): string | null {
  const { pathname, search } = location;
  if (pathname === '/login' || pathname === '/register') return null;
  return `/login?from=${encodeURIComponent(pathname + search)}`;
}

/**
 * How long a request gets before it is given up on.
 *
 * Ten seconds, not ky's default of ten *thousand* milliseconds' worth of
 * patience on a hung API: past that a spinner is telling the person nothing
 * except that the page is broken, and a `TimeoutError` at least produces a
 * toast and a Sentry event. The one call that legitimately takes longer —
 * the monthly report export, which the backend builds synchronously — passes
 * its own `timeout` per request.
 */
export const REQUEST_TIMEOUT_MS = 10_000;

/**
 * Retry policy, spelled out rather than inherited.
 *
 * ky's defaults happen to be close to this, but they are ky's to change in a
 * major version, and the thing they would silently change is whether a POST
 * that creates a booking can run twice. Only methods that are idempotent by
 * definition are retried, and only on statuses that mean "not now" rather
 * than "no": 4xx other than 408/429 are the caller's fault and repeating them
 * only costs time. POST/PATCH are absent on purpose — the ones that must
 * survive a retry carry an `Idempotency-Key` instead (see the booking hooks).
 */
const RETRY: RetryOptions = {
  limit: 2,
  methods: ['get', 'head', 'options', 'trace'],
  statusCodes: [408, 429, 500, 502, 503, 504],
};

const api = ky.create({
  prefix: env.VITE_API_URL,
  timeout: REQUEST_TIMEOUT_MS,
  retry: RETRY,
  credentials: 'include',
  hooks: {
    beforeError: [
      // Every failure leaving this client is an `ApiError`: the backend's
      // error body read once, here, instead of at each of the twenty-odd
      // call sites that used to re-read `error.data` themselves.
      ({ error }) => (isHTTPError(error) ? new ApiError(error) : error),
    ],
    beforeRequest: [
      ({ request }) => {
        const csrfToken = useStore.getState().csrfToken;
        if (csrfToken) {
          request.headers.set('X-CSRF-Token', csrfToken);
        }
      },
    ],
    afterResponse: [
      async ({ request, options, response }) => {
        if (response.status !== 401) {
          return response;
        }

        // Skip token refresh for auth-only endpoints (login, register, etc.)
        // but allow refresh for protected /auth/me endpoints.
        const url = request.url;
        if (url.includes('/auth/') && !url.includes('/auth/me')) {
          return response;
        }

        try {
          await refreshAccessToken();
          // Retry with new cookies (automatically sent) and new CSRF token.
          const csrfToken = useStore.getState().csrfToken;
          if (csrfToken) {
            request.headers.set('X-CSRF-Token', csrfToken);
          }
          // `options` is ky's own `NormalizedOptions` (the afterResponse hook's
          // resolved view), not the public `Options` input type `ky()`
          // accepts — under `exactOptionalPropertyTypes` a few of its
          // normalized fields (e.g. `window: null`) don't structurally match
          // `Options`'s own declarations, even though ky produced this exact
          // object from an `Options` value in the first place. Retyping it as
          // what it started as is ky's own typing gap, not a real mismatch.
          return await ky(request, { ...options, hooks: {} } as Options);
        } catch {
          useStore.getState().logout();
          const target = loginUrlPreserving(window.location);
          if (target) {
            window.location.href = target;
          }
          return response;
        }
      },
    ],
  },
});

export default api;

/**
 * ky forwards `signal` straight to `fetch`'s `RequestInit`, whose DOM typing
 * is `signal?: AbortSignal | null` — no explicit `| undefined`. Every api
 * wrapper in `src/features/*\/api` takes an optional `signal?: AbortSignal`
 * and used to forward it as `{ signal }`; under `exactOptionalPropertyTypes`,
 * an object literal with an explicit `signal: undefined` key doesn't satisfy
 * that DOM type, even though omitting the key entirely does. Spreading this
 * in keeps the key out of the options object when there is no signal.
 */
export function withSignal(signal?: AbortSignal): { signal: AbortSignal } | Record<string, never> {
  return signal ? { signal } : {};
}
