import ky from 'ky';
import type { Options } from 'ky';
import { useStore } from '@/shared/stores';
import { env } from '@/shared/lib/env';
import { parseWith } from '@/shared/lib/apiParse';
import { refreshResponseSchema } from '@/shared/schemas/auth.schema';
import type { RefreshResponse } from '@/shared/types/api.types';

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

const api = ky.create({
  prefix: env.VITE_API_URL,
  timeout: 30_000,
  credentials: 'include',
  hooks: {
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
          const path = window.location.pathname;
          if (path !== '/login' && path !== '/register') {
            window.location.href = '/login';
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
