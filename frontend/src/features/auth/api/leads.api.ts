import api from '@/shared/lib/ky';

/**
 * Which form the person left. `register` is the password register form (the
 * address was typed); `google` is the second step of the Google sign-up (the
 * address was verified by Google, the phone number was never given). The
 * server tags the spreadsheet row with a distinct origin for each.
 */
export type AbandonedLeadSource = 'register' | 'google';

/**
 * Fire-and-forget: captures the email of someone who started registering but
 * hasn't finished, so it can be followed up on later. Never surfaces errors
 * to the user — losing this signal is not worth interrupting the flow.
 */
export function captureAbandonedRegistrationLead(email: string, source: AbandonedLeadSource = 'register'): void {
  void api.post('public/leads/abandoned-registration', { json: { email, source } }).catch(() => {
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
export function captureAbandonedRegistrationLeadBeacon(email: string, source: AbandonedLeadSource = 'register'): void {
  const apiUrl: string = import.meta.env.VITE_API_URL ?? '/api/v1';
  const url = `${apiUrl}/public/leads/abandoned-registration`;
  const blob = new Blob([JSON.stringify({ email, source })], { type: 'text/plain' });
  navigator.sendBeacon(url, blob);
}
