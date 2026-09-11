import { useState } from 'react';
import { env } from '@/shared/lib/env';

/**
 * Shared plumbing for the Cloudflare Turnstile challenge on the
 * register/login/forgot-password forms: the current token and whether
 * submit should be blocked because a challenge is configured but not yet
 * solved.
 *
 * Deliberately does not own the `TurnstileField` ref — `eslint-plugin-react-hooks`'s
 * `refs` rule flags any object returned from a custom hook that bundles a
 * `useRef` value together with derived helpers, treating property/method
 * access on it as an unsafe read of a ref during render (see
 * `useIntersectionObserver`, which returns *only* a ref for the same
 * reason, never bundled with other state). Callers keep their own
 * `useRef<TurnstileFieldHandle>(null)` and build `resetTurnstile` from it.
 *
 * `required` is `false` (and everything else a no-op) when
 * `VITE_TURNSTILE_SITE_KEY` is unset — `TurnstileField` itself renders
 * nothing in that case, so a self-hosted instance without Turnstile sees no
 * change in behavior.
 */
export function useTurnstileChallenge() {
  const [token, setToken] = useState<string>();
  const required = Boolean(env.VITE_TURNSTILE_SITE_KEY);

  return {
    onTokenChange: setToken,
    required,
    /** Combined with a mutation's own `isPending`, decides the submit button's `disabled`. */
    isBlocked: (isPending: boolean) => isPending || (required && !token),
    /** Spread the result into the request payload — omits the key entirely when there's no token. */
    payload: (): { turnstile_token?: string } => (token ? { turnstile_token: token } : {}),
  };
}
