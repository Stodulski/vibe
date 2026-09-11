import api from '@/shared/lib/ky';

/**
 * Which form the person left. `register` is the password register form (the
 * address was typed); `google` is the second step of the Google sign-up (the
 * address was verified by Google, the phone number was never given). The
 * server tags the spreadsheet row with a distinct origin for each.
 */
export type AbandonedLeadSource = 'register' | 'google';

/**
 * What is known about the person who left: the email always, and whatever
 * they had filled in before leaving. The server bounds the lengths and
 * forwards the rest as typed; a phone typed halfway is still a lead.
 */
export interface AbandonedLead {
  email: string;
  /** Defaults to `register`. */
  source?: AbandonedLeadSource;
  first_name?: string;
  last_name?: string;
  phone?: string;
}

/** The JSON body: only the fields that carry something, `source` always. */
function leadBody(lead: AbandonedLead): Record<string, string> {
  const body: Record<string, string> = { email: lead.email, source: lead.source ?? 'register' };
  for (const key of ['first_name', 'last_name', 'phone'] as const) {
    const value = lead[key]?.trim();
    if (value) body[key] = value;
  }
  return body;
}

/**
 * Fire-and-forget: captures the email of someone who started registering but
 * hasn't finished, so it can be followed up on later. Never surfaces errors
 * to the user — losing this signal is not worth interrupting the flow.
 */
export function captureAbandonedRegistrationLead(lead: AbandonedLead): void {
  void api.post('public/leads/abandoned-registration', { json: leadBody(lead) }).catch(() => {
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
export function captureAbandonedRegistrationLeadBeacon(lead: AbandonedLead): void {
  const apiUrl: string = import.meta.env.VITE_API_URL ?? '/api/v1';
  const url = `${apiUrl}/public/leads/abandoned-registration`;
  const blob = new Blob([JSON.stringify(leadBody(lead))], { type: 'text/plain' });
  navigator.sendBeacon(url, blob);
}
