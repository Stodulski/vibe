/**
 * The two ways the confirm page lets a slot hold go, best effort.
 *
 * Both go out as a beacon rather than through the ky client: every caller is
 * about to leave the page ("Cambiar horario", the browser's back, a closed
 * tab, a reload), and a beacon is the one request a browser promises to finish
 * after the document is gone. It carries no body, so it is a simple CORS
 * request with no preflight; the answer is always a `204` and nobody reads it.
 * Where beacons do not exist, a `keepalive` fetch does the same job. If both
 * fail, the hold ends on its own when it expires.
 */
function holdUrl(apiBaseUrl: string, token: string, action: 'release' | 'leave'): string {
  return `${apiBaseUrl.replace(/\/+$/, '')}/book/holds/${encodeURIComponent(token)}/${action}`;
}

function sendHoldBeacon(apiBaseUrl: string, token: string, action: 'release' | 'leave'): void {
  const url = holdUrl(apiBaseUrl, token, action);
  try {
    if (typeof navigator !== 'undefined' && typeof navigator.sendBeacon === 'function' && navigator.sendBeacon(url)) {
      return;
    }
    void fetch(url, { method: 'POST', keepalive: true, credentials: 'omit' }).catch(() => undefined);
  } catch {
    // Best effort: the hold expires by itself.
  }
}

/** Gives the hold back now (`POST /book/holds/{token}/release`): the visitor chose another time. */
export function releaseHold(apiBaseUrl: string, token: string): void {
  sendHoldBeacon(apiBaseUrl, token, 'release');
}

/**
 * The page went away (`POST /book/holds/{token}/leave`): the server lets the
 * hold go a minute later unless the page loads again and takes it back
 * (`publicBookingApi.reclaimHold`), so a reload does not lose the slot.
 */
export function leaveHold(apiBaseUrl: string, token: string): void {
  sendHoldBeacon(apiBaseUrl, token, 'leave');
}

/**
 * Gives the hold back now and resolves once the server answered (or could not
 * be reached), for a page that stays and wants the freed slot on its grid
 * afterwards. A `keepalive` fetch, so it also survives the page going away.
 */
export async function releaseHoldNow(apiBaseUrl: string, token: string): Promise<void> {
  try {
    await fetch(holdUrl(apiBaseUrl, token, 'release'), { method: 'POST', keepalive: true, credentials: 'omit' });
  } catch {
    // Best effort: the hold expires by itself.
  }
}
