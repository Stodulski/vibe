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

// Only ever a same-origin path: this is where the callback sends the owner
// back to. Rejects protocol-relative (`//x`), backslash (`/\x`, which
// browsers read as `//x`), percent-encoded slash/backslash lead-ins,
// control characters and anything that resolves to another origin.
function isSameOriginPath(path: string): boolean {
  if (!path.startsWith('/') || /^\/(?:[/\\]|%2f|%5c)/i.test(path)) return false;
  // eslint-disable-next-line no-control-regex
  if (/[\u0000-\u001f\u007f]/.test(path)) return false;
  try {
    return new URL(path, window.location.origin).origin === window.location.origin;
  } catch {
    return false;
  }
}

const entrySchema = z.object({
  complexId: z.string().min(1),
  codeVerifier: z.string().min(1),
  returnPath: z.string().refine(isSameOriginPath),
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
 * already-consumed, expired (same ten minutes as pruning) or unreadable entry.
 */
export function consumeMPOAuthSession(state: string, now: number = Date.now()): MPOAuthSession | null {
  const key = entryKey(state);
  const entry = safeSessionStorage.getJSON(key, entrySchema);
  safeSessionStorage.remove(key);
  return entry && now - entry.createdAt <= MAX_AGE_MS ? entry : null;
}

/** Removes every OAuth attempt, current and legacy — on logout. */
export function clearMPOAuthSessions(): void {
  removeLegacyKeys();
  for (const key of safeSessionStorage.keys().filter(isEntryKey)) safeSessionStorage.remove(key);
}
