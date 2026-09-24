import { useEffect, useRef } from 'react';
import { useMutation } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { authApi } from '../api/auth.api';
import { useAuthSuccessHandler } from './authSuccess';
import { takeGoogleReturnPath } from '../lib/googleSignInReturn';
import { readCookie } from '@/shared/lib/cookies';
import { getHttpStatus } from '@/shared/lib/utils';
import type { GoogleExchangeRequest, GoogleNeedsProfileResponse } from '@/shared/types/api.types';

/**
 * The cookie Google Identity Services sets on this origin during the redirect
 * hop, non-`HttpOnly` so the page can read it back. It is the browser's half
 * of the double submit: the backend stored its hash with the code and refuses
 * a mismatch.
 */
const CSRF_COOKIE = 'g_csrf_token';

function needsProfile(data: unknown): data is GoogleNeedsProfileResponse {
  return typeof data === 'object' && data !== null && 'needs_profile' in data;
}

/**
 * Every way this page can end up back on `/login`. The backend's redirect
 * handler mints `google_rejected`, `google_unavailable` and
 * `google_rate_limited` itself (it never renders a problem page — it always
 * 303s to the app), and the exchange below adds `google_expired` and its own
 * `google_rate_limited` for the same reason (a 429 straight from this
 * endpoint, not relayed through the redirect handler); they share one
 * mechanism, the `?error=` query string, because the backend can only speak
 * through the URL. `LoginPage` turns each into one line of copy.
 */
const GOOGLE_REJECTED = '/login?error=google_rejected';
const GOOGLE_EXPIRED = '/login?error=google_expired';
const GOOGLE_UNAVAILABLE = '/login?error=google_unavailable';
const GOOGLE_RATE_LIMITED = '/login?error=google_rate_limited';

/**
 * Second hop of the Google redirect flow: trades the single-use `code` the
 * backend put in `/auth/google/return?code=…`, plus the `g_csrf_token` cookie
 * Google set alongside it, for a session.
 *
 * Both are needed, and that is a security property rather than a formality:
 * a code on its own proves only that *a* Google sign-in happened, so anyone
 * could mint one with their own account and send the victim a
 * `/auth/google/return?code=…` link that quietly logs them in as the
 * attacker. The cookie cannot be set on this origin from anywhere else, so
 * requiring it is what makes the link useless. A missing cookie therefore
 * fails closed — no request at all — rather than being sent as an empty
 * string and letting the backend decide.
 *
 * Success is handled exactly like the old popup exchange — an existing
 * account logs in through `useAuthSuccessHandler` (same store updates, same
 * redirect target), an unknown email hands off to `/register/google` with the
 * profile token in router state, never in the URL.
 *
 * Failure never shows a toast: this page has nothing to show it on, because
 * it navigates away immediately. It goes to `/login?error=…` instead, the
 * same channel the backend's own refusals use.
 *
 * The code is single-use and expires in 120 s, so it is spent exactly once
 * per mount: the `startedRef` guard survives React StrictMode's deliberate
 * double-invocation of effects (same component instance, so the ref is the
 * same object) as well as any re-render.
 */
export function useGoogleExchange(code: string | null) {
  const navigate = useNavigate();
  const handleAuthSuccess = useAuthSuccessHandler();
  const startedRef = useRef(false);
  // Read out of `sessionStorage` in the effect below and kept here because
  // `onSuccess` runs long after it, on a page that cannot work the
  // destination out for itself.
  const returnPathRef = useRef<string | undefined>(undefined);

  const { mutate } = useMutation({
    mutationFn: (data: GoogleExchangeRequest) => authApi.googleExchange(data),
    onSuccess: (data) => {
      const from = returnPathRef.current;

      if (needsProfile(data)) {
        // The destination rides along in router state rather than being
        // re-parked: `/register/google` is one more step before there is a
        // session, and `useGoogleComplete` finishes through the same
        // `useAuthSuccessHandler`, which reads `from` straight out of
        // `location.state` (`loginRedirectStateSchema`). Omitted entirely
        // when there is nothing to carry, so the state still parses as the
        // plain `{ profile_token, profile }` it was.
        void navigate('/register/google', {
          state: from
            ? { profile_token: data.profile_token, profile: data.profile, from: { pathname: from } }
            : { profile_token: data.profile_token, profile: data.profile },
          replace: true,
        });
        return;
      }
      handleAuthSuccess(data, { from });
    },
    onError: (error: unknown) => {
      const status = getHttpStatus(error);

      // 429 means the exchange itself was throttled, distinct from
      // `google_unavailable`: the sign-in almost worked, and the copy says so
      // rather than implying Google is down.
      if (status === 429) {
        void navigate(GOOGLE_RATE_LIMITED, { replace: true });
        return;
      }

      // 422 is the other refusal this endpoint has: an invalid, already-spent
      // or expired code (RFC 9457 `validation`, `errors[0].field === 'code'`).
      // Everything else — a 5xx, a dropped connection, a body that failed its
      // schema — is the API being unreachable as far as the visitor cares.
      void navigate(status === 422 ? GOOGLE_EXPIRED : GOOGLE_UNAVAILABLE, { replace: true });
    },
  });

  useEffect(() => {
    if (startedRef.current) return;
    startedRef.current = true;

    // No `code` in the URL means this page was reached by hand, by a stale
    // bookmark, or from a redirect that lost its query — there is nothing to
    // exchange, so it reads as a rejected sign-in rather than a broken API.
    if (!code) {
      void navigate(GOOGLE_REJECTED, { replace: true });
      return;
    }

    // No cookie means it was never set, was already dropped, or cookies are
    // blocked outright. There is no request worth making — the backend would
    // refuse it with the same 422 an unknown code gets — so this takes the
    // same exit as that 422 does, and says so with the same sentence.
    const csrfToken = readCookie(CSRF_COOKIE);
    if (!csrfToken) {
      void navigate(GOOGLE_EXPIRED, { replace: true });
      return;
    }

    // Taken once, here, and not on every failure exit above: the two early
    // returns leave `/login`, where a later attempt will park its own
    // destination anyway, and reading it out on the way past would throw away
    // a perfectly good one on a transient stumble.
    returnPathRef.current = takeGoogleReturnPath();

    mutate({ code, g_csrf_token: csrfToken });
  }, [code, mutate, navigate]);
}
