import { afterAll, afterEach, beforeAll } from 'vitest';
import { server } from './msw/server';

// Stands in for the backend across the suite, in every environment. Node-only
// API tests reach the real ky client too. `onUnhandledRequest: 'error'` fails a
// test the moment it fires a request no handler covers.
beforeAll(() => {
  server.listen({ onUnhandledRequest: 'error' });
});

afterEach(() => {
  server.resetHandlers();
});

afterAll(() => {
  server.close();
});

// DOM-only setup. Pure-logic tests run under `// @vitest-environment node`, where
// there is no document and nothing below may touch one.
if (typeof document !== 'undefined') {
  // Node 26 ships its own `localStorage` global, which can take the slot
  // happy-dom would fill and read as undefined. Give the tests a working Storage
  // when that happens; it only fills a gap.
  installStorage('localStorage');
  installStorage('sessionStorage');

  await import('@testing-library/jest-dom/vitest');
  const { cleanup } = await import('@testing-library/react');

  afterEach(() => {
    cleanup();
    localStorage.clear();
    sessionStorage.clear();
  });

  if (typeof globalThis.ResizeObserver === 'undefined') {
    globalThis.ResizeObserver = class ResizeObserver {
      observe() {
        /* noop polyfill */
      }
      unobserve() {
        /* noop polyfill */
      }
      disconnect() {
        /* noop polyfill */
      }
    };
  }

  if (typeof Element.prototype.scrollIntoView === 'undefined') {
    Element.prototype.scrollIntoView = function () {
      /* noop polyfill */
    };
  }
}

function installStorage(name: 'localStorage' | 'sessionStorage'): void {
  const existing = (globalThis as Record<string, unknown>)[name];
  if (existing && typeof (existing as Storage).clear === 'function') return;

  const entries = new Map<string, string>();
  const storage: Storage = {
    get length() {
      return entries.size;
    },
    clear() {
      entries.clear();
    },
    getItem(key) {
      return entries.get(key) ?? null;
    },
    key(index) {
      return [...entries.keys()][index] ?? null;
    },
    removeItem(key) {
      entries.delete(key);
    },
    setItem(key, value) {
      entries.set(key, value);
    },
  };

  Object.defineProperty(globalThis, name, { configurable: true, writable: true, value: storage });
}
