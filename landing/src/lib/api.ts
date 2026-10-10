/**
 * Server-side client for the public API the storefront reads from.
 *
 * The base URL is an argument, not an import, so the module has no build-time
 * dependencies and runs under node:test. The page passes PUBLIC_API_URL from astro:env.
 */
import { InvalidHubData, parseHubData, type HubData } from './hub.ts';

/** How long a page waits for the API before it answers 503. */
export const API_TIMEOUT_MS = 3000;

/** not_found: the API has no such record (HTTP 404). unavailable: anything else. */
export type ApiErrorKind = 'not_found' | 'unavailable';

export class ApiError extends Error {
  readonly kind: ApiErrorKind;
  readonly status: number | null;

  constructor(kind: ApiErrorKind, status: number | null, options?: { cause?: unknown }) {
    super(`public API ${kind}${status === null ? '' : ` (HTTP ${status})`}`, options);
    this.name = 'ApiError';
    this.kind = kind;
    this.status = status;
  }
}

export interface PublicApiOptions {
  baseUrl: string;
  fetchImpl?: typeof fetch;
  timeoutMs?: number;
}

export interface PublicApi {
  hubData(city: string): Promise<HubData>;
}

export function createPublicApi({
  baseUrl,
  fetchImpl = fetch,
  timeoutMs = API_TIMEOUT_MS,
}: PublicApiOptions): PublicApi {
  const root = baseUrl.replace(/\/+$/, '');

  async function getJson(path: string): Promise<unknown> {
    let response: Response;
    try {
      response = await fetchImpl(`${root}${path}`, {
        headers: { Accept: 'application/json' },
        signal: AbortSignal.timeout(timeoutMs),
      });
    } catch (cause) {
      // Network failure or timeout: the record may exist, so this is not a 404.
      throw new ApiError('unavailable', null, { cause });
    }
    if (response.status === 404) throw new ApiError('not_found', 404);
    if (!response.ok) throw new ApiError('unavailable', response.status);
    try {
      return await response.json();
    } catch (cause) {
      throw new ApiError('unavailable', response.status, { cause });
    }
  }

  return {
    async hubData(city) {
      const json = await getJson(`/public/hubs/${encodeURIComponent(city)}/data`);
      try {
        return parseHubData(json);
      } catch (cause) {
        if (cause instanceof InvalidHubData) throw new ApiError('unavailable', 200, { cause });
        throw cause;
      }
    },
  };
}
