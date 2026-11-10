/**
 * Gives a slot hold back (`POST /book/holds/{token}/release`), best effort.
 *
 * It goes out as a beacon rather than through the ky client: every caller is
 * about to leave the page ("Cambiar horario", the browser's back, a closed
 * tab), and a beacon is the one request a browser promises to finish after the
 * document is gone. It carries no body, so it is a simple CORS request with no
 * preflight; the answer is always a `204` and nobody reads it. Where beacons do
 * not exist, a `keepalive` fetch does the same job. If both fail, the hold ends
 * on its own when it expires.
 */
export function releaseHold(apiBaseUrl: string, token: string): void {
  const url = `${apiBaseUrl.replace(/\/+$/, '')}/book/holds/${encodeURIComponent(token)}/release`;
  try {
    if (typeof navigator !== 'undefined' && typeof navigator.sendBeacon === 'function' && navigator.sendBeacon(url)) {
      return;
    }
    void fetch(url, { method: 'POST', keepalive: true, credentials: 'omit' }).catch(() => undefined);
  } catch {
    // Best effort: the hold expires by itself.
  }
}
