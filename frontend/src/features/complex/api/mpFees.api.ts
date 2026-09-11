import { parseWith } from '@/shared/lib/apiParse';
import { env } from '@/shared/lib/env';
import { mpFeesResponseSchema, type MPFeesResponse } from '@/shared/schemas/mpFees.schema';

const MP_FEES_PATH = '/datos/mercadopago-costos.json';

/**
 * A public static file the landing publishes, not an `api.vibe.com.ar`
 * endpoint — plain `fetch` instead of the shared `ky` instance, which would
 * send the session cookie and a CSRF header to a cross-origin host that
 * neither needs nor expects them.
 */
export async function fetchMPFees(signal?: AbortSignal): Promise<MPFeesResponse> {
  const response = await fetch(new URL(MP_FEES_PATH, env.VITE_LANDING_URL), signal ? { signal } : {});
  if (!response.ok) {
    throw new Error(`mpFeesApi.fetch: HTTP ${String(response.status)}`);
  }
  const data: unknown = await response.json();
  return parseWith(mpFeesResponseSchema, 'mpFeesApi.fetch')(data);
}
