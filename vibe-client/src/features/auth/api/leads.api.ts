import api from '@/shared/lib/ky';

/**
 * Fire-and-forget: captures the email of someone who started registering but
 * hasn't finished, so it can be followed up on later. Never surfaces errors
 * to the user — losing this signal is not worth interrupting the flow.
 */
export function captureAbandonedRegistrationLead(email: string): void {
  void api.post('public/leads/abandoned-registration', { json: { email } }).catch(() => {
    /* best-effort */
  });
}

/**
 * Same capture, but via navigator.sendBeacon — the only delivery mechanism
 * reliable enough to complete during a 'pagehide'/'visibilitychange' unload,
 * where an in-flight fetch can get cancelled by the browser.
 *
 * The Blob is sent as text/plain, not application/json: sendBeacon can only
 * send CORS-safelisted content types (it can't do a preflight), so with the
 * API on a different origin from the app, application/json made the browser
 * silently drop the request. The Go handler decodes the body as JSON either
 * way — it never looks at Content-Type.
 */
export function captureAbandonedRegistrationLeadBeacon(email: string): void {
  const apiUrl: string = import.meta.env.VITE_API_URL ?? '/api/v1';
  const url = `${apiUrl}/public/leads/abandoned-registration`;
  const blob = new Blob([JSON.stringify({ email })], { type: 'text/plain' });
  navigator.sendBeacon(url, blob);
}
