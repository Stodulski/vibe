/**
 * The /baja page's logic: the request it makes, the token it reads, and what each answer means
 * to the person. Pure apart from the `fetch` it is given, so node:test runs it directly.
 */

/** How long the page waits for the API before it asks the person to try again. */
export const UNSUBSCRIBE_TIMEOUT_MS = 10_000;

/** The opt-out endpoint under the public API base (PUBLIC_API_URL, which ends in /api/v1). */
export function unsubscribeUrl(baseUrl: string): string {
  return `${baseUrl.replace(/\/+$/, '')}/public/email/unsubscribe`;
}

/** The `token` query parameter, trimmed, or null when it is missing or blank. */
export function tokenFromSearch(search: string): string | null {
  const token = new URLSearchParams(search).get('token')?.trim() ?? '';
  return token === '' ? null : token;
}

/**
 * done: the address is off the list (204). invalid: the API does not accept the link (400, or no
 * token at all). retry: anything else, including a network failure or a timeout.
 */
export type UnsubscribeOutcome = 'done' | 'invalid' | 'retry';

export function outcomeForStatus(status: number): UnsubscribeOutcome {
  if (status === 204) return 'done';
  if (status === 400) return 'invalid';
  return 'retry';
}

export interface UnsubscribeOptions {
  baseUrl: string;
  token: string | null;
  fetchImpl?: typeof fetch;
  timeoutMs?: number;
}

/** Sends the opt-out for a token. A missing token is invalid, and no request is made. */
export async function unsubscribe({
  baseUrl,
  token,
  fetchImpl = fetch,
  timeoutMs = UNSUBSCRIBE_TIMEOUT_MS,
}: UnsubscribeOptions): Promise<UnsubscribeOutcome> {
  if (token === null) return 'invalid';
  try {
    const response = await fetchImpl(unsubscribeUrl(baseUrl), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token }),
      signal: AbortSignal.timeout(timeoutMs),
    });
    return outcomeForStatus(response.status);
  } catch {
    return 'retry';
  }
}
