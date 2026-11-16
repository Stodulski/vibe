/**
 * Credentials of the fake Mercado Pago (see ./server.ts): who a bearer token
 * belongs to, and the OAuth grants that issue and rotate seller tokens.
 *
 * Two parties exist, as on the real marketplace:
 *   - the platform (the Vibe application), whose token is MP_FAKE_PLATFORM_TOKEN
 *     and must equal the backend's MP_ACCESS_TOKEN;
 *   - sellers (the complexes), each a numeric collector id with OAuth access and
 *     refresh tokens. One seller is seeded at start (MP_FAKE_COLLECTOR_ID with the
 *     tokens the fixtures write); a spec may register more through
 *     `POST /__fake/sellers` to have a seller it can expire on its own.
 *
 * Like Mercado Pago, a refresh rotates the refresh token (the spent one stops
 * working) and leaves the previous access token valid. The seeded refresh token
 * is the exception: every fixture writes the same one into every complex, and
 * the backend's 12-hour sweep refreshes each of them, so spending it once must
 * not disconnect the rest.
 */
import { randomBytes } from 'node:crypto';
import type { IncomingMessage } from 'node:http';
import { COLLECTOR_ID } from './store.ts';

export const PLATFORM_TOKEN = process.env.MP_FAKE_PLATFORM_TOKEN ?? 'e2e-fake-platform-token';
export const APP_ID = process.env.MP_FAKE_APP_ID ?? 'e2e-fake-app';
export const CLIENT_SECRET = process.env.MP_FAKE_CLIENT_SECRET ?? 'e2e-fake-client-secret';
/** The seller credentials the fixtures seal into every complex they connect (helpers/public-booking-fixtures.ts). */
export const SEED_ACCESS_TOKEN = 'fake-e2e-access-token';
export const SEED_REFRESH_TOKEN = 'fake-e2e-refresh-token';

export type Party = { kind: 'platform' } | { kind: 'seller'; collectorId: number };

interface AccessToken {
  collectorId: number;
  expired: boolean;
}

interface RefreshToken {
  collectorId: number;
  /** Survives being spent: the seeded one, shared by every fixture-connected complex. */
  reusable: boolean;
  spent: boolean;
}

export interface SellerCredentials {
  collector_id: number;
  access_token: string;
  refresh_token: string;
}

const accessTokens = new Map<string, AccessToken>();
const refreshTokens = new Map<string, RefreshToken>();
let nextCollectorId = COLLECTOR_ID + 1;

function issue(collectorId: number): SellerCredentials {
  const access = `APP_USR-fake-${String(collectorId)}-${randomBytes(12).toString('hex')}`;
  const refresh = `TG-fake-${String(collectorId)}-${randomBytes(12).toString('hex')}`;
  accessTokens.set(access, { collectorId, expired: false });
  refreshTokens.set(refresh, { collectorId, reusable: false, spent: false });
  return { collector_id: collectorId, access_token: access, refresh_token: refresh };
}

accessTokens.set(SEED_ACCESS_TOKEN, { collectorId: COLLECTOR_ID, expired: false });
refreshTokens.set(SEED_REFRESH_TOKEN, { collectorId: COLLECTOR_ID, reusable: true, spent: false });

/** A new seller with its own collector id and tokens. */
export function registerSeller(): SellerCredentials {
  return issue(nextCollectorId++);
}

/** Expires every access token the seller holds now; tokens issued later work. Returns how many. */
export function expireSeller(collectorId: number): number {
  let count = 0;
  for (const token of accessTokens.values()) {
    if (token.collectorId === collectorId && !token.expired) {
      token.expired = true;
      count++;
    }
  }
  return count;
}

export function bearerOf(req: IncomingMessage): string {
  const header = req.headers.authorization ?? '';
  const match = /^Bearer\s+(\S+)$/i.exec(header);
  return match?.[1] ?? '';
}

export type Authentication = { ok: true; party: Party } | { ok: false; reason: string };

/** Who the request's bearer token belongs to, or why it is refused (Mercado Pago answers 401). */
export function authenticate(req: IncomingMessage): Authentication {
  const token = bearerOf(req);
  if (token === '') return { ok: false, reason: 'missing access token' };
  if (token === PLATFORM_TOKEN) return { ok: true, party: { kind: 'platform' } };
  const seller = accessTokens.get(token);
  if (!seller) return { ok: false, reason: 'invalid access token' };
  if (seller.expired) return { ok: false, reason: 'expired access token' };
  return { ok: true, party: { kind: 'seller', collectorId: seller.collectorId } };
}

/** A label for the request log: platform, seller:<id>, expired:<id>, unknown or none. */
export function describeBearer(req: IncomingMessage): string {
  const token = bearerOf(req);
  if (token === '') return 'none';
  if (token === PLATFORM_TOKEN) return 'platform';
  const seller = accessTokens.get(token);
  if (!seller) return 'unknown';
  return `${seller.expired ? 'expired' : 'seller'}:${String(seller.collectorId)}`;
}

export type GrantResult =
  { ok: true; credentials: SellerCredentials } | { ok: false; status: number; error: string; message: string };

/** The `POST /oauth/token` grants the backend uses: authorization_code and refresh_token. */
export function grant(body: Record<string, unknown>): GrantResult {
  if (body.client_id !== APP_ID || body.client_secret !== CLIENT_SECRET) {
    return { ok: false, status: 401, error: 'invalid_client', message: 'invalid client_id or client_secret' };
  }
  if (body.grant_type === 'authorization_code') {
    if (typeof body.code !== 'string' || body.code === '') {
      return { ok: false, status: 400, error: 'invalid_grant', message: 'missing code' };
    }
    return { ok: true, credentials: issue(COLLECTOR_ID) };
  }
  if (body.grant_type === 'refresh_token') {
    const token = typeof body.refresh_token === 'string' ? refreshTokens.get(body.refresh_token) : undefined;
    if (!token || token.spent) {
      return { ok: false, status: 400, error: 'invalid_grant', message: 'invalid or already used refresh_token' };
    }
    if (!token.reusable) token.spent = true;
    return { ok: true, credentials: issue(token.collectorId) };
  }
  return { ok: false, status: 400, error: 'unsupported_grant_type', message: 'unsupported grant_type' };
}
