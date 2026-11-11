import { createHash, randomBytes } from 'node:crypto';
import { execFileSync } from 'node:child_process';

/**
 * Direct access to the isolated E2E database, for the few fixtures no API
 * exposes. Always the E2E database (`make e2e`'s :5433), never a dev one: a
 * missing database must fail loud rather than write test data elsewhere.
 */
const DB = {
  host: process.env.E2E_DB_HOST ?? 'localhost',
  port: process.env.E2E_DB_PORT ?? '5433',
  user: process.env.E2E_DB_USER ?? 'vibe',
  name: process.env.E2E_DB_NAME ?? 'vibe_e2e',
  password: process.env.E2E_DB_PASSWORD ?? 'vibe_e2e',
};

/** The E2E database's DSN, for tools that take one (e.g. `cmd/mpcredkey`). */
export const E2E_DB_DSN = `postgres://${DB.user}:${DB.password}@${DB.host}:${DB.port}/${DB.name}?sslmode=disable`;

/** Runs one SQL statement against the E2E database as the schema owner. */
export function runE2eSql(sql: string): void {
  execFileSync(
    'psql',
    ['-h', DB.host, '-p', DB.port, '-U', DB.user, '-d', DB.name, '-v', 'ON_ERROR_STOP=1', '-c', sql],
    {
      stdio: 'pipe',
      env: { ...process.env, PGPASSWORD: DB.password },
    },
  );
}

/**
 * Gives a booking a working link token and returns its plaintext, the value a
 * client finds in their confirmation link (`/book/cancel?token=...`).
 *
 * Only a storefront booking's API answer carries its token; an owner-created
 * booking's never leaves the server. This stores one the way
 * `bookingstore.MintLinkToken` does: SHA-256 of the plaintext, raw bytes.
 */
export function mintBookingLinkToken(bookingId: string, complexId: string): string {
  const plaintext = randomBytes(32).toString('base64url');
  const hash = createHash('sha256').update(plaintext).digest('hex');
  runE2eSql(
    `INSERT INTO booking_link_tokens (booking_id, token_hash, expires_at, complex_id)
     VALUES ('${bookingId}', decode('${hash}', 'hex'), NOW() + INTERVAL '30 days', '${complexId}');`,
  );
  return plaintext;
}
