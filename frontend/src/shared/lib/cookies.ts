/**
 * Reads one cookie from `document.cookie`.
 *
 * Only non-`HttpOnly` cookies are visible here, which is the whole point of
 * the one caller today: Google Identity Services' redirect flow sets
 * `g_csrf_token` on the app origin precisely so the page can read it back and
 * double-submit it (see `useGoogleExchange`). Session cookies are `HttpOnly`
 * and stay invisible to this.
 *
 * Returns `undefined` for a cookie that is absent, empty, or unreadable —
 * `document.cookie` is `''` when cookies are blocked, and a caller that must
 * fail closed can treat all three the same way.
 *
 * The value is `decodeURIComponent`'d, because that is how a cookie value
 * carrying anything outside the cookie-octet set was written. A value that is
 * not valid percent-encoding is returned raw rather than throwing: a cookie
 * this cannot decode is still the cookie the server set.
 */
export function readCookie(name: string): string | undefined {
  const prefix = `${name}=`;

  for (const part of document.cookie.split(';')) {
    const entry = part.trim();
    if (!entry.startsWith(prefix)) continue;

    const raw = entry.slice(prefix.length);
    if (!raw) return undefined;

    try {
      return decodeURIComponent(raw);
    } catch {
      return raw;
    }
  }

  return undefined;
}
