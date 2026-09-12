import { describe, it, expect, vi } from 'vitest';
import { Suspense } from 'react';
import { render, waitFor } from '@testing-library/react';
import { lazyRetry } from './routeHelpers';

/**
 * Replaces `window.sessionStorage` wholesale with a stub whose `setItem`
 * always throws — happy-dom's real `sessionStorage` is Proxy-backed, so
 * `vi.spyOn` on it (or on `Storage.prototype`) silently fails to intercept
 * calls, which is why this swaps the whole object instead. Restored by the
 * returned callback.
 */
function stubThrowingSessionStorage() {
  const original = window.sessionStorage;
  Object.defineProperty(window, 'sessionStorage', {
    configurable: true,
    value: {
      getItem: () => null,
      setItem: () => {
        throw new DOMException('SecurityError');
      },
      removeItem: () => {
        /* noop stub */
      },
      clear: () => {
        /* noop stub */
      },
      key: () => null,
      length: 0,
    },
  });
  return () => {
    Object.defineProperty(window, 'sessionStorage', { configurable: true, value: original });
  };
}

/**
 * Replaces `window.location` wholesale with a stub carrying a bare `reload`
 * spy. Holding `reload` as its own variable (rather than reading it back off
 * `window.location.reload`) sidesteps `@typescript-eslint/unbound-method` —
 * there is no bound-method reference to unbind in the first place.
 */
function stubLocationReload() {
  const original = window.location;
  const reload = vi.fn();
  Object.defineProperty(window, 'location', { configurable: true, value: { reload } });
  return {
    reload,
    restore: () => {
      Object.defineProperty(window, 'location', { configurable: true, value: original });
    },
  };
}

describe('lazyRetry', () => {
  // A stale chunk after a deploy makes the dynamic import reject; the
  // recovery path records a `chunk_reload` flag and reloads once. When
  // sessionStorage itself is unavailable (private browsing, disabled
  // storage, a full quota), recording that flag must not turn a recoverable
  // "chunk missing" case into an unhandled crash.
  it('reloads once instead of crashing when sessionStorage is unavailable', async () => {
    const restoreSessionStorage = stubThrowingSessionStorage();
    const { reload, restore: restoreLocation } = stubLocationReload();
    const factory = vi.fn().mockRejectedValue(new Error('chunk load failed'));
    const Component = lazyRetry(factory);

    expect(() => {
      render(
        <Suspense fallback={<div>loading</div>}>
          <Component />
        </Suspense>,
      );
    }).not.toThrow();

    await waitFor(() => {
      expect(reload).toHaveBeenCalledTimes(1);
    });

    restoreLocation();
    restoreSessionStorage();
  });
});
