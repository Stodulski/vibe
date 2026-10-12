import { safeLocalStorage } from '@/shared/lib/safeStorage';
import { STORAGE_KEYS } from '@/shared/lib/storageKeys';

/**
 * Whether this browser has signed in and has not signed out since.
 *
 * The session cookies are HttpOnly, so no script can read them. This flag is
 * the one thing the page can read before the session check answers, and it
 * decides two things:
 * - whether a guest page waits for the check (only a browser that may hold a
 *   session waits; a first-time visitor sees the form at once);
 * - whether a refused refresh is retried (a browser that has never signed in
 *   has no session for a sibling tab to be renewing, so the retry is skipped).
 *
 * It is written where the session is known: a sign-in, a session read that
 * found a user or none, and every sign-out. It is a hint, not a credential. A
 * stale value costs one extra wait or a skipped retry; the server still decides
 * every request.
 */
export function hasSessionHint(): boolean {
  return safeLocalStorage.get(STORAGE_KEYS.SESSION_HINT) === '1';
}

export function setSessionHint(signedIn: boolean): void {
  if (signedIn) {
    safeLocalStorage.set(STORAGE_KEYS.SESSION_HINT, '1');
  } else {
    safeLocalStorage.remove(STORAGE_KEYS.SESSION_HINT);
  }
}
