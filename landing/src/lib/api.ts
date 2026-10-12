/**
 * Server-side client for the public API the storefront reads from.
 *
 * The base URL is an argument, not an import, so the module has no build-time
 * dependencies and runs under node:test. The page passes PUBLIC_API_URL from astro:env.
 */
import { InvalidHubData, isRecord, parseHubData, type HubData } from './hub.ts';
import { parseHubIndex, type HubIndex } from './hub-index.ts';

/** How long a page waits for the API before it answers 503. */
export const API_TIMEOUT_MS = 3000;

/** How long the sitemap waits for the API. It lists every complex, so it is larger than a hub. */
export const SITEMAP_TIMEOUT_MS = 5000;

/**
 * not_found: the API has no such record (HTTP 404). gone: the API says the complex was deleted
 * and names it (HTTP 410). unavailable: anything else, including a 410 the page cannot read.
 */
export type ApiErrorKind = 'not_found' | 'gone' | 'unavailable';

/** A deleted complex, as the API's 410 body names it. */
export interface GoneComplex {
  name: string;
  city: string;
  citySlug: string;
}

export class ApiError extends Error {
  readonly kind: ApiErrorKind;
  readonly status: number | null;
  /** Set only for kind 'gone': the deleted complex the 410 body names. */
  readonly gone: GoneComplex | null;

  constructor(kind: ApiErrorKind, status: number | null, options?: { cause?: unknown; gone?: GoneComplex }) {
    super(`public API ${kind}${status === null ? '' : ` (HTTP ${status})`}`, options);
    this.name = 'ApiError';
    this.kind = kind;
    this.status = status;
    this.gone = options?.gone ?? null;
  }
}

/** The complex a 410 body names: {"gone": {"name", "city", "city_slug"}}, or null when it does not. */
function goneOf(json: unknown): GoneComplex | null {
  if (!isRecord(json) || !isRecord(json.gone)) return null;
  const { name, city, city_slug: citySlug } = json.gone;
  if (typeof name !== 'string' || typeof city !== 'string' || typeof citySlug !== 'string') return null;
  if (name.trim() === '' || city.trim() === '' || citySlug === '') return null;
  return { name, city, citySlug };
}

/**
 * A 410 that names its complex is 'gone'. One whose body does not parse is 'unavailable', as
 * every payload the landing cannot read is: the API says the record is gone, but without a
 * name and a city the page has nothing honest to say, and a 404 would be a guess.
 */
async function goneError(response: Response): Promise<ApiError> {
  let json: unknown;
  try {
    json = await response.json();
  } catch (cause) {
    return new ApiError('unavailable', 410, { cause });
  }
  const gone = goneOf(json);
  return gone ? new ApiError('gone', 410, { gone }) : new ApiError('unavailable', 410);
}

export interface PublicApiOptions {
  baseUrl: string;
  fetchImpl?: typeof fetch;
  timeoutMs?: number;
}

export interface PublicApi {
  /** The public complex record, unparsed. The page validates it against the booking schema. */
  complex(slug: string): Promise<unknown>;
  hubData(city: string): Promise<HubData>;
  /** Every city that has a complex, for the city index. */
  hubIndex(): Promise<HubIndex>;
  /** The backend's sitemap document, unfiltered. */
  sitemapXml(): Promise<string>;
}

export function createPublicApi({
  baseUrl,
  fetchImpl = fetch,
  timeoutMs = API_TIMEOUT_MS,
}: PublicApiOptions): PublicApi {
  const root = baseUrl.replace(/\/+$/, '');

  async function get(path: string, accept: string): Promise<Response> {
    let response: Response;
    try {
      response = await fetchImpl(`${root}${path}`, {
        headers: { Accept: accept },
        signal: AbortSignal.timeout(timeoutMs),
      });
    } catch (cause) {
      // Network failure or timeout: the record may exist, so this is not a 404.
      throw new ApiError('unavailable', null, { cause });
    }
    if (response.status === 404) throw new ApiError('not_found', 404);
    if (response.status === 410) throw await goneError(response);
    if (!response.ok) throw new ApiError('unavailable', response.status);
    return response;
  }

  async function getJson(path: string): Promise<unknown> {
    const response = await get(path, 'application/json');
    try {
      return await response.json();
    } catch (cause) {
      throw new ApiError('unavailable', response.status, { cause });
    }
  }

  async function getText(path: string): Promise<string> {
    const response = await get(path, 'application/xml');
    try {
      return await response.text();
    } catch (cause) {
      throw new ApiError('unavailable', response.status, { cause });
    }
  }

  return {
    complex(slug) {
      return getJson(`/public/complexes/${encodeURIComponent(slug)}`);
    },
    async hubData(city) {
      const json = await getJson(`/public/hubs/${encodeURIComponent(city)}/data`);
      try {
        return parseHubData(json);
      } catch (cause) {
        if (cause instanceof InvalidHubData) throw new ApiError('unavailable', 200, { cause });
        throw cause;
      }
    },
    async hubIndex() {
      const json = await getJson('/public/hubs');
      try {
        return parseHubIndex(json);
      } catch (cause) {
        if (cause instanceof InvalidHubData) throw new ApiError('unavailable', 200, { cause });
        throw cause;
      }
    },
    sitemapXml() {
      return getText('/sitemap.xml');
    },
  };
}
