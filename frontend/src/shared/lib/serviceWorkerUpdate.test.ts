import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import type { registerSW } from 'virtual:pwa-register';
import {
  applyPendingServiceWorkerUpdate,
  isUpdatePending,
  setupServiceWorkerUpdates,
  SW_UPDATE_INTERVAL_MS,
} from './serviceWorkerUpdate';
import { API_CACHE_NAME } from './apiCache';
import { markUnsavedWork } from './unsavedWork';

type RegisterOptions = NonNullable<Parameters<typeof registerSW>[0]>;

/** A fake `registerSW` that records the options and hands back a spy updateSW. */
function fakeRegister() {
  let options: RegisterOptions | undefined;
  const updateSW = vi.fn<(reloadPage?: boolean) => Promise<void>>().mockResolvedValue(undefined);
  const register = ((opts?: RegisterOptions) => {
    options = opts;
    return updateSW;
  }) as typeof registerSW;
  return {
    register,
    updateSW,
    options: () => {
      if (!options) throw new Error('registerSW was not called');
      return options;
    },
  };
}

/**
 * Puts a browser in front of the module: a spy `reload`, a `serviceWorker` that
 * can dispatch `controllerchange`, and a Cache Storage that records deletes.
 */
function stubBrowser() {
  const reload = vi.fn();
  vi.stubGlobal('location', { reload });
  const serviceWorker = new EventTarget();
  vi.stubGlobal('navigator', { serviceWorker });
  const deleteCache = vi.fn().mockResolvedValue(true);
  vi.stubGlobal('caches', { delete: deleteCache });
  return { reload, serviceWorker, deleteCache };
}

/** Drives the tab between foreground and background. */
function setVisibility(state: 'visible' | 'hidden') {
  Object.defineProperty(document, 'visibilityState', { value: state, configurable: true });
  document.dispatchEvent(new Event('visibilitychange'));
}

afterEach(() => {
  vi.unstubAllGlobals();
  Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true });
});

describe('setupServiceWorkerUpdates', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('registers immediately and never reloads the page on its own', () => {
    const reload = vi.fn();
    vi.stubGlobal('location', { reload });
    const sw = fakeRegister();

    setupServiceWorkerUpdates(sw.register);
    sw.options().onNeedRefresh?.();

    expect(sw.options().immediate).toBe(true);
    expect(reload).not.toHaveBeenCalled();
    expect(sw.updateSW).not.toHaveBeenCalled();
    vi.unstubAllGlobals();
  });

  it('marks the update pending when a new worker is waiting, with no visible prompt', () => {
    const sw = fakeRegister();
    setupServiceWorkerUpdates(sw.register);
    expect(isUpdatePending()).toBe(false);

    sw.options().onNeedRefresh?.();

    expect(isUpdatePending()).toBe(true);
    expect(sw.updateSW).not.toHaveBeenCalled();
  });
});

describe('setupServiceWorkerUpdates cache purge', () => {
  it('purges the runtime API cache before handing over to the new worker', () => {
    const browser = stubBrowser();
    const sw = fakeRegister();
    setupServiceWorkerUpdates(sw.register);
    sw.options().onNeedRefresh?.();
    expect(browser.deleteCache).not.toHaveBeenCalled();

    applyPendingServiceWorkerUpdate();

    expect(browser.deleteCache).toHaveBeenCalledWith(API_CACHE_NAME);
  });
});

describe('setupServiceWorkerUpdates update checks', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('checks for a new worker on the interval and whenever the tab becomes visible', () => {
    vi.useFakeTimers();
    try {
      const sw = fakeRegister();
      setupServiceWorkerUpdates(sw.register);
      const update = vi.fn().mockResolvedValue(undefined);
      const registration = { update } as unknown as ServiceWorkerRegistration;

      sw.options().onRegisteredSW?.('/sw.js', registration);
      expect(update).not.toHaveBeenCalled();

      vi.advanceTimersByTime(SW_UPDATE_INTERVAL_MS);
      expect(update).toHaveBeenCalledTimes(1);

      Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true });
      document.dispatchEvent(new Event('visibilitychange'));
      expect(update).toHaveBeenCalledTimes(2);
    } finally {
      vi.useRealTimers();
    }
  });

  it('does nothing on registration when the browser hands back no registration', () => {
    const sw = fakeRegister();
    setupServiceWorkerUpdates(sw.register);

    expect(() => {
      sw.options().onRegisteredSW?.('/sw.js', undefined);
    }).not.toThrow();
  });
});

// PWA-09: a build that waits for a click reaches almost nobody, and a build
// that reloads on its own interrupts everybody. These pin the moments where
// neither is true — the update is applied silently instead.
describe('applyPendingServiceWorkerUpdate', () => {
  beforeEach(() => {
    markUnsavedWork('form', false);
  });

  it('reports nothing pending until a new worker is actually waiting', () => {
    const sw = fakeRegister();
    setupServiceWorkerUpdates(sw.register);
    expect(isUpdatePending()).toBe(false);

    sw.options().onNeedRefresh?.();

    expect(isUpdatePending()).toBe(true);
  });

  it('does nothing when no new worker is waiting', () => {
    const browser = stubBrowser();
    const sw = fakeRegister();
    setupServiceWorkerUpdates(sw.register);

    applyPendingServiceWorkerUpdate();

    expect(sw.updateSW).not.toHaveBeenCalled();
    expect(browser.deleteCache).not.toHaveBeenCalled();
  });

  it('purges the API cache, asks for the takeover and reloads on the controller change', () => {
    const browser = stubBrowser();
    const sw = fakeRegister();
    setupServiceWorkerUpdates(sw.register);
    sw.options().onNeedRefresh?.();

    applyPendingServiceWorkerUpdate();

    expect(browser.deleteCache).toHaveBeenCalledWith(API_CACHE_NAME);
    expect(sw.updateSW).toHaveBeenCalledWith(true);
    expect(browser.reload).not.toHaveBeenCalled();

    browser.serviceWorker.dispatchEvent(new Event('controllerchange'));
    expect(browser.reload).toHaveBeenCalledTimes(1);
  });

  // Two silent triggers can both fire for one waiting worker; a second
  // handover would stack a second `controllerchange` listener and reload twice.
  it('is idempotent: a second call while one is in flight does nothing', () => {
    const browser = stubBrowser();
    const sw = fakeRegister();
    setupServiceWorkerUpdates(sw.register);
    sw.options().onNeedRefresh?.();

    applyPendingServiceWorkerUpdate();
    applyPendingServiceWorkerUpdate();
    applyPendingServiceWorkerUpdate();

    expect(sw.updateSW).toHaveBeenCalledTimes(1);
    expect(browser.deleteCache).toHaveBeenCalledTimes(1);

    browser.serviceWorker.dispatchEvent(new Event('controllerchange'));
    expect(browser.reload).toHaveBeenCalledTimes(1);
  });
});

describe('setupServiceWorkerUpdates background trigger', () => {
  beforeEach(() => {
    markUnsavedWork('form', false);
  });

  it('applies a pending update when the tab goes to the background', () => {
    const browser = stubBrowser();
    const sw = fakeRegister();
    setupServiceWorkerUpdates(sw.register);
    sw.options().onNeedRefresh?.();

    setVisibility('hidden');

    expect(browser.deleteCache).toHaveBeenCalledWith(API_CACHE_NAME);
    expect(sw.updateSW).toHaveBeenCalledWith(true);

    browser.serviceWorker.dispatchEvent(new Event('controllerchange'));
    expect(browser.reload).toHaveBeenCalledTimes(1);
  });

  // The whole reason prompt mode exists: a half-filled form must not be
  // reloaded away, not even while nobody is looking at the tab.
  it('leaves a pending update alone while a form holds unsaved changes', () => {
    const browser = stubBrowser();
    const sw = fakeRegister();
    setupServiceWorkerUpdates(sw.register);
    sw.options().onNeedRefresh?.();
    markUnsavedWork('form', true);

    setVisibility('hidden');

    expect(sw.updateSW).not.toHaveBeenCalled();
    expect(browser.deleteCache).not.toHaveBeenCalled();
    expect(browser.reload).not.toHaveBeenCalled();

    // Once the form is clean the next trip to the background lands it.
    markUnsavedWork('form', false);
    setVisibility('hidden');
    expect(sw.updateSW).toHaveBeenCalledWith(true);
  });

  it('does nothing on the way to the background when no new worker is waiting', () => {
    const browser = stubBrowser();
    const sw = fakeRegister();
    setupServiceWorkerUpdates(sw.register);

    setVisibility('hidden');

    expect(sw.updateSW).not.toHaveBeenCalled();
    expect(browser.deleteCache).not.toHaveBeenCalled();
  });
});

describe('setupServiceWorkerUpdates update detected while hidden', () => {
  beforeEach(() => {
    markUnsavedWork('form', false);
  });

  it('applies the update at once when it is detected while the tab is already hidden', () => {
    const browser = stubBrowser();
    setVisibility('hidden');
    const sw = fakeRegister();
    setupServiceWorkerUpdates(sw.register);

    // The periodic check finds the new worker without any `visibilitychange`
    // happening first — the tab was already backgrounded when the deploy
    // landed, so there is no future hide event to wait for.
    sw.options().onNeedRefresh?.();

    expect(isUpdatePending()).toBe(true);
    expect(browser.deleteCache).toHaveBeenCalledWith(API_CACHE_NAME);
    expect(sw.updateSW).toHaveBeenCalledWith(true);
  });

  it('does not apply the update detected while hidden if a form has unsaved changes', () => {
    const browser = stubBrowser();
    markUnsavedWork('form', true);
    setVisibility('hidden');
    const sw = fakeRegister();
    setupServiceWorkerUpdates(sw.register);

    sw.options().onNeedRefresh?.();

    expect(isUpdatePending()).toBe(true);
    expect(sw.updateSW).not.toHaveBeenCalled();
    expect(browser.deleteCache).not.toHaveBeenCalled();
  });

  it('waits for a navigation or a hide when the update is detected while visible', () => {
    const browser = stubBrowser();
    setVisibility('visible');
    const sw = fakeRegister();
    setupServiceWorkerUpdates(sw.register);

    sw.options().onNeedRefresh?.();

    expect(isUpdatePending()).toBe(true);
    expect(sw.updateSW).not.toHaveBeenCalled();
    expect(browser.deleteCache).not.toHaveBeenCalled();

    setVisibility('hidden');
    expect(sw.updateSW).toHaveBeenCalledWith(true);
  });
});
