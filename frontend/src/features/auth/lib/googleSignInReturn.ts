import { safeSessionStorage } from '@/shared/lib/safeStorage';
import { isSafeRedirect } from '../hooks/authSuccess';

/**
 * Where the destination waits while the browser is away at Google.
 *
 * `sessionStorage`, not `localStorage`, and deliberately: the value must die
 * with the tab. A destination remembered in one tab has no business steering
 * a sign-in started in another one tomorrow.
 */
const RETURN_PATH_KEY = 'vibe.google-signin.from';

/**
 * Parks the page a visitor was trying to reach before they were sent to sign
 * in, so it survives the Google redirect hop.
 *
 * In popup mode this was free: the page never left, so `useAuthSuccessHandler`
 * could still read the `/login` router state (or `?from=`) when the credential
 * came back. Redirect mode hands the browser to Google and gets it back on
 * `/auth/google/return`, a URL that carries only `?code=` — so a person who
 * was heading for `/bookings?date=…`, got bounced to
 * `/login?from=%2Fbookings…`, and then chose Google used to land on the role
 * default instead of where they were going.
 *
 * Anything {@link isSafeRedirect} rejects is not stored, and — this is the
 * part that matters — neither is `undefined`: the key is removed instead, so
 * a destination left over from an earlier, abandoned attempt can never be
 * picked up by a later sign-in that had no destination of its own.
 */
export function rememberGoogleReturnPath(from: string | undefined): void {
  if (isSafeRedirect(from)) {
    safeSessionStorage.set(RETURN_PATH_KEY, from);
  } else {
    safeSessionStorage.remove(RETURN_PATH_KEY);
  }
}

/**
 * Reads the parked destination and removes it in the same breath — it is
 * spent by the sign-in that reads it, exactly like the code it travels with.
 *
 * Re-filtered on the way out rather than trusted because it was filtered on
 * the way in: `sessionStorage` is writable by anything running on this
 * origin, and a redirect target is not something to take on faith from
 * storage. Returns `undefined` for an absent, empty or rejected value, which
 * is the same thing to every caller — the role default.
 */
export function takeGoogleReturnPath(): string | undefined {
  const stored = safeSessionStorage.get(RETURN_PATH_KEY) ?? undefined;
  safeSessionStorage.remove(RETURN_PATH_KEY);
  return isSafeRedirect(stored) ? stored : undefined;
}
