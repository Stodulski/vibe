import { describe, it, expect } from 'vitest';
import { z } from 'zod';
import { safeLocalStorage, safeSessionStorage } from './safeStorage';

/**
 * Replaces `window[name]` wholesale with a stub whose methods always throw —
 * happy-dom's real storage is Proxy-backed, so `vi.spyOn(Storage.prototype, …)`
 * (or on the instance) silently fails to intercept calls. Confirmed against
 * `src/pages/public/BookConfirmPage.test.tsx`'s `stubThrowingSessionStorage`.
 * Restored by the returned callback.
 */
function stubThrowingStorage(name: 'localStorage' | 'sessionStorage') {
  const original = window[name];
  Object.defineProperty(window, name, {
    configurable: true,
    value: {
      getItem: () => {
        throw new DOMException('SecurityError');
      },
      setItem: () => {
        throw new DOMException('QuotaExceededError');
      },
      removeItem: () => {
        throw new DOMException('SecurityError');
      },
      clear: () => {
        /* noop stub */
      },
      key: () => null,
      length: 0,
    },
  });
  return () => {
    Object.defineProperty(window, name, { configurable: true, value: original });
  };
}

const schema = z.object({ a: z.string() });

describe.each([
  ['safeLocalStorage', safeLocalStorage, 'localStorage'] as const,
  ['safeSessionStorage', safeSessionStorage, 'sessionStorage'] as const,
])('%s', (_label, storage, globalName) => {
  it('round-trips a value through get/set', () => {
    storage.set('k', 'v');
    expect(storage.get('k')).toBe('v');
  });

  it('removes a value', () => {
    storage.set('k', 'v');
    storage.remove('k');
    expect(storage.get('k')).toBeNull();
  });

  it('returns null from get() instead of throwing when the underlying storage throws', () => {
    const restore = stubThrowingStorage(globalName);
    expect(() => storage.get('k')).not.toThrow();
    expect(storage.get('k')).toBeNull();
    restore();
  });

  it('does not throw from set() when the underlying storage throws (e.g. quota exceeded)', () => {
    const restore = stubThrowingStorage(globalName);
    expect(() => {
      storage.set('k', 'v');
    }).not.toThrow();
    restore();
  });

  it('does not throw from remove() when the underlying storage throws', () => {
    const restore = stubThrowingStorage(globalName);
    expect(() => {
      storage.remove('k');
    }).not.toThrow();
    restore();
  });

  describe('getJSON', () => {
    it('returns null when the key is missing', () => {
      expect(storage.getJSON('missing', schema)).toBeNull();
    });

    it('parses and validates a stored value', () => {
      storage.set('k', JSON.stringify({ a: 'hello' }));
      expect(storage.getJSON('k', schema)).toEqual({ a: 'hello' });
    });

    it('returns null for unparseable JSON', () => {
      storage.set('k', 'not json{');
      expect(storage.getJSON('k', schema)).toBeNull();
    });

    it('returns null when the parsed value fails schema validation', () => {
      storage.set('k', JSON.stringify({ a: 123 }));
      expect(storage.getJSON('k', schema)).toBeNull();
    });

    it('returns null instead of throwing when the underlying storage throws', () => {
      const restore = stubThrowingStorage(globalName);
      expect(() => storage.getJSON('k', schema)).not.toThrow();
      expect(storage.getJSON('k', schema)).toBeNull();
      restore();
    });
  });
});
