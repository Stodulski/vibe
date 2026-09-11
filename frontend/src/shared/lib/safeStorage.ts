import type { z } from 'zod';

/**
 * Thin, never-throwing wrapper over `localStorage`/`sessionStorage`.
 *
 * Both APIs throw in ordinary, non-buggy situations a component has no way to
 * prevent: Safari private browsing throws on `setItem` (and, on old
 * versions, on merely reading `window.localStorage`), a full quota throws
 * `QuotaExceededError`, and a user or corporate policy can disable storage
 * outright. None of that is a bug in the caller — it's what browser storage
 * *is* — so every method here swallows the failure and returns a safe
 * fallback (`null` / a no-op) instead of letting it propagate into whatever
 * click handler or render happened to touch storage first.
 */
export interface SafeStorage {
  get(key: string): string | null;
  set(key: string, value: string): void;
  remove(key: string): void;
  /**
   * Reads `key`, `JSON.parse`s it, and validates the result against `schema`.
   * Returns `null` for anything this cannot use: no entry, storage
   * unavailable, unparseable JSON, or a shape `schema` rejects — one empty
   * case for a caller to design for instead of three failure modes.
   */
  getJSON<T>(key: string, schema: z.ZodType<T>): T | null;
}

function createSafeStorage(getArea: () => Storage): SafeStorage {
  function get(key: string): string | null {
    try {
      return getArea().getItem(key);
    } catch {
      return null;
    }
  }

  function set(key: string, value: string): void {
    try {
      getArea().setItem(key, value);
    } catch {
      // Quota exceeded, private mode, or storage disabled — nothing a caller
      // can do about it here, so the write is silently dropped.
    }
  }

  function remove(key: string): void {
    try {
      getArea().removeItem(key);
    } catch {
      // Same as above: best-effort cleanup, never worth crashing over.
    }
  }

  function getJSON<T>(key: string, schema: z.ZodType<T>): T | null {
    const raw = get(key);
    if (raw == null) return null;
    try {
      const parsed: unknown = JSON.parse(raw);
      const result = schema.safeParse(parsed);
      return result.success ? result.data : null;
    } catch {
      return null;
    }
  }

  return { get, set, remove, getJSON };
}

export const safeLocalStorage = createSafeStorage(() => window.localStorage);
export const safeSessionStorage = createSafeStorage(() => window.sessionStorage);
