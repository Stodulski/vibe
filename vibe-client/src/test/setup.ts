// Global test setup.
//
// See installStorage below for why the DOM environment cannot be relied on to
// provide localStorage.
//
// Pure-logic tests opt out of the DOM via a `// @vitest-environment node`
// docblock, which cuts environment startup per file. Guarding on `document`
// keeps this setup usable from both environments: under `node` there is no
// DOM to extend, so importing the matchers would throw.
if (typeof document !== 'undefined') {
  // Node 26 ships its own `localStorage` global, gated behind
  // --localstorage-file. It is defined on globalThis before the test
  // environment loads, and under vitest `window` *is* globalThis — so the slot
  // is already taken and happy-dom never installs its own storage. Without the
  // flag Node's resolves to undefined, and every test that touches storage
  // dies on `localStorage.clear()`.
  //
  // Rather than pinning Node forever, give the tests a storage that works.
  // This is test-only: the browser and the dev server use the real thing.
  installStorage('localStorage');
  installStorage('sessionStorage');

  await import('@testing-library/jest-dom/vitest');
  const { cleanup } = await import('@testing-library/react');

  // Unmount rendered trees and drop persisted state after every test.
  // Testing Library mounts into document.body; without this, renders pile up
  // and queries start matching leftovers from earlier tests.
  afterEach(() => {
    cleanup();
    localStorage.clear();
    sessionStorage.clear();
  });

  // Polyfill ResizeObserver for components that depend on it (e.g. cmdk)
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

  // Polyfill Element.scrollIntoView for the test DOM (used by cmdk)
  if (typeof Element.prototype.scrollIntoView === 'undefined') {
    Element.prototype.scrollIntoView = function () {
      /* noop polyfill */
    };
  }
}

/**
 * Put a working Storage on `globalThis[name]` when the environment has not.
 *
 * Only fills a gap: if the environment already provides usable storage, this
 * leaves it alone, so the day the underlying conflict is resolved upstream
 * nothing here has to change.
 */
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

  Object.defineProperty(globalThis, name, {
    configurable: true,
    writable: true,
    value: storage,
  });
}
