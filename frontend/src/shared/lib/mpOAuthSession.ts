import { z } from 'zod';
import { safeSessionStorage } from './safeStorage';

/**
 * One sessionStorage entry per MercadoPago OAuth attempt, keyed by the
 * `state` nonce that goes out in the authorization URL and comes back on the
 * callback. Binding the PKCE verifier to its own nonce is what lets two
 * attempts coexist (a second click, a second tab, a back-button retry): the
 * callback looks up exactly the verifier that matches the challenge it was
 * started with, instead of whichever one was written last.
 */
const ENTRY_PREFIX = 'mp_oauth_';

// Keys written before the entries were bound to their nonce: one global
// verifier, one return path, and a complexId-only entry per nonce. Nothing
// reads them any more; they are swept on the next write and on logout.
const LEGACY_VERIFIER_KEY = 'mp_code_verifier';
const LEGACY_RETURN_PATH_KEY = 'mp_return_path';
const LEGACY_NONCE_PREFIX = 'mp_oauth_complex_';

/** An attempt older than this is abandoned: MercadoPago's own code is short-lived. */
const MAX_AGE_MS = 10 * 60 * 1000;
/** Upper bound on entries kept, so repeated clicks can never grow storage. */
const MAX_ENTRIES = 5;

const entrySchema = z.object({
  complexId: z.string().min(1),
  codeVerifier: z.string().min(1),
  // Only ever a same-origin path: this is where the callback sends the owner
  // back to, so a protocol-relative value must never be accepted.
  returnPath: z
    .string()
    .startsWith('/')
    .refine((p) => !p.startsWith('//')),
  createdAt: z.number(),
});

export type MPOAuthSession = z.infer<typeof entrySchema>;

export const DEFAULT_RETURN_PATH = '/settings';

function entryKey(state: string): string {
  return ENTRY_PREFIX + state;
}

function isEntryKey(key: string): boolean {
  return key.startsWith(ENTRY_PREFIX) && !key.startsWith(LEGACY_NONCE_PREFIX);
}

function removeLegacyKeys(): void {
  safeSessionStorage.remove(LEGACY_VERIFIER_KEY);
  safeSessionStorage.remove(LEGACY_RETURN_PATH_KEY);
  for (const key of safeSessionStorage.keys()) {
    if (key.startsWith(LEGACY_NONCE_PREFIX)) safeSessionStorage.remove(key);
  }
}

/** Drops expired and unreadable entries, then the oldest ones beyond the cap. */
function prune(now: number, keep: number): void {
  const live: { key: string; createdAt: number }[] = [];
  for (const key of safeSessionStorage.keys().filter(isEntryKey)) {
    const entry = safeSessionStorage.getJSON(key, entrySchema);
    if (!entry || now - entry.createdAt > MAX_AGE_MS) {
      safeSessionStorage.remove(key);
      continue;
    }
    live.push({ key, createdAt: entry.createdAt });
  }
  live.sort((a, b) => b.createdAt - a.createdAt);
  for (const { key } of live.slice(keep)) safeSessionStorage.remove(key);
}

/**
 * Persists the attempt for `state`. Called when the owner clicks "Conectar",
 * right before the browser leaves for MercadoPago — never earlier, so merely
 * opening the card leaves nothing behind.
 */
export function saveMPOAuthSession(
  state: string,
  session: Omit<MPOAuthSession, 'createdAt'>,
  now: number = Date.now(),
): void {
  removeLegacyKeys();
  prune(now, MAX_ENTRIES - 1);
  safeSessionStorage.set(entryKey(state), JSON.stringify({ ...session, createdAt: now }));
}

/**
 * Reads and deletes the attempt for `state` in one step. The entry is gone
 * before the caller does anything with it, so a reload of the callback URL
 * cannot replay a single-use authorization code. `null` for an unknown,
 * already-consumed or unreadable entry.
 */
export function consumeMPOAuthSession(state: string): MPOAuthSession | null {
  const key = entryKey(state);
  const entry = safeSessionStorage.getJSON(key, entrySchema);
  safeSessionStorage.remove(key);
  return entry;
}

/** Removes every OAuth attempt, current and legacy — on logout. */
export function clearMPOAuthSessions(): void {
  removeLegacyKeys();
  for (const key of safeSessionStorage.keys().filter(isEntryKey)) safeSessionStorage.remove(key);
}
