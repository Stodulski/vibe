import { useEffect, useRef } from 'react';
import { useMutation } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { authApi } from '../api/auth.api';
import { useAuthSuccessHandler } from './authSuccess';
import { takeGoogleReturnPath } from '../lib/googleSignInReturn';
import { getProblem } from '@/shared/lib/ApiError';
import { getHttpStatus } from '@/shared/lib/utils';
import type { GoogleFinishRequest, GoogleNeedsProfileResponse } from '@/shared/types/api.types';

function needsProfile(data: unknown): data is GoogleNeedsProfileResponse {
  return typeof data === 'object' && data !== null && 'needs_profile' in data;
}

/**
 * Every way this page can end up back on `/login`. `LoginPage` turns each
 * into one line of copy (`useGoogleLoginError`).
 */
const GOOGLE_REJECTED = '/login?error=google_rejected';
const GOOGLE_EXPIRED = '/login?error=google_expired';
const GOOGLE_UNAVAILABLE = '/login?error=google_unavailable';
const GOOGLE_RATE_LIMITED = '/login?error=google_rate_limited';

/**
 * Second half of the OIDC authorization-code flow: trades the `code` and
 * `state` Google appended to `/auth/google/callback?code=…&state=…` for a
 * session.
 *
 * `state` is bound to the `google_oauth_state` cookie `/auth/google/start`
 * set before the browser ever left for Google — the backend checks the two
 * match, single-use, and refuses a replay — so nothing here has to read or
 * double-submit a cookie the way the old `g_csrf_token` redirect flow did.
 *
 * Success is handled exactly like the old popup/redirect exchanges did — an
 * existing account logs in through `useAuthSuccessHandler` (same store
 * updates, same redirect target), an unknown email hands off to
 * `/register/google` with the profile token in router state, never in the
 * URL.
 *
 * Failure never shows a toast: this page has nothing to show it on, because
 * it navigates away immediately. It goes to `/login?error=…` instead.
 *
 * The code and state are single-use, so the finish call runs exactly once
 * per mount: the `startedRef` guard survives React StrictMode's deliberate
 * double-invocation of effects (same component instance, so the ref is the
 * same object) as well as any re-render.
 */
export function useGoogleFinish(code: string | null, state: string | null, error: string | null) {
  const navigate = useNavigate();
  const handleAuthSuccess = useAuthSuccessHandler();
  const startedRef = useRef(false);
  // Read out of `sessionStorage` in the effect below and kept here because
  // `onSuccess` runs long after it, on a page that cannot work the
  // destination out for itself.
  const returnPathRef = useRef<string | undefined>(undefined);

  const { mutate } = useMutation({
    mutationFn: (data: GoogleFinishRequest) => authApi.googleFinish(data),
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
    onError: (err: unknown) => {
      const status = getHttpStatus(err);

      // 429 means the finish call itself was throttled, distinct from
      // `google_unavailable`: the sign-in almost worked, and the copy says so
      // rather than implying Google is down.
      if (status === 429) {
        void navigate(GOOGLE_RATE_LIMITED, { replace: true });
        return;
      }

      // 422 is the other refusal `/finish` has, and it names which half
      // failed: `credential` means the ID token or its nonce was rejected —
      // `google_rejected` — anything else (the backend only ever names
      // `code`) is a missing, mismatched, expired or replayed state, or
      // Google's own `invalid_grant` — `google_expired`. Everything else
      // (a 5xx, a dropped connection, a body that failed its schema) is the
      // API being unreachable as far as the visitor cares.
      if (status === 422) {
        const field = getProblem(err)?.errors[0]?.field;
        void navigate(field === 'credential' ? GOOGLE_REJECTED : GOOGLE_EXPIRED, { replace: true });
        return;
      }

      void navigate(GOOGLE_UNAVAILABLE, { replace: true });
    },
  });

  useEffect(() => {
    if (startedRef.current) return;
    startedRef.current = true;

    // Google sent its own OAuth refusal instead of a code: `access_denied` is
    // the visitor backing out of the account chooser, not a failure worth a
    // message, so it returns quietly. Any other value Google's authorization
    // server can send back (`server_error`, `temporarily_unavailable`, …) is
    // as unavailable as a dropped connection to this app.
    if (error) {
      void navigate(error === 'access_denied' ? '/login' : GOOGLE_UNAVAILABLE, { replace: true });
      return;
    }

    // No `code` or `state` in the URL means this page was reached by hand,
    // by a stale bookmark, or by a callback that lost its query — there is
    // nothing to exchange. This reads the same as a state the backend has
    // already forgotten, not as a rejected sign-in: nothing here can tell the
    // two apart.
    if (!code || !state) {
      void navigate(GOOGLE_EXPIRED, { replace: true });
      return;
    }

    // Taken once, here, and not on every failure exit above: the early
    // returns leave `/login`, where a later attempt will park its own
    // destination anyway, and reading it out on the way past would throw away
    // a perfectly good one on a transient stumble.
    returnPathRef.current = takeGoogleReturnPath();

    mutate({ code, state });
  }, [code, state, error, mutate, navigate]);
}
