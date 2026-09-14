import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { toast } from 'sonner';
import type { registerSW } from 'virtual:pwa-register';
import { ES_AR } from '@/shared/i18n/es_AR';
import {
  applyPendingServiceWorkerUpdate,
  isUpdatePending,
  setupServiceWorkerUpdates,
  SW_UPDATE_INTERVAL_MS,
  SW_UPDATE_TOAST_ID,
} from './serviceWorkerUpdate';
import { API_CACHE_NAME } from './apiCache';
import { markUnsavedWork } from './unsavedWork';

vi.mock('sonner', () => ({ toast: Object.assign(vi.fn(), { dismiss: vi.fn() }) }));

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

function lastToastOptions() {
  const call = vi.mocked(toast).mock.calls.at(-1);
  if (!call) throw new Error('toast was not called');
  return call[1] as {
    id?: string;
    duration?: number;
    action?: { label: string; onClick: (event: unknown) => void };
  };
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

  it('shows one persistent toast with the update action when a new worker is waiting', () => {
    const sw = fakeRegister();
    setupServiceWorkerUpdates(sw.register);

    sw.options().onNeedRefresh?.();

    expect(toast).toHaveBeenCalledWith(ES_AR.common.updateAvailable, expect.anything());
    const opts = lastToastOptions();
    expect(opts.id).toBe(SW_UPDATE_TOAST_ID);
    expect(opts.duration).toBe(Infinity);
    expect(opts.action?.label).toBe(ES_AR.common.updateNow);
  });

  it('asks the waiting worker to take over, with a reload, only when the action is clicked', () => {
    const sw = fakeRegister();
    setupServiceWorkerUpdates(sw.register);
    sw.options().onNeedRefresh?.();
    expect(sw.updateSW).not.toHaveBeenCalled();

    lastToastOptions().action?.onClick(new Event('click'));

    expect(sw.updateSW).toHaveBeenCalledTimes(1);
    expect(sw.updateSW).toHaveBeenCalledWith(true);
  });

  it('reloads once the new worker takes control after the click, and not before', () => {
    const reload = vi.fn();
    vi.stubGlobal('location', { reload });
    const serviceWorker = new EventTarget();
    vi.stubGlobal('navigator', { serviceWorker });
    const sw = fakeRegister();
    setupServiceWorkerUpdates(sw.register);
    sw.options().onNeedRefresh?.();

    // A takeover nobody asked for (e.g. every tab closed and reopened) is
    // not a reason to reload this tab.
    serviceWorker.dispatchEvent(new Event('controllerchange'));
    expect(reload).not.toHaveBeenCalled();

    lastToastOptions().action?.onClick(new Event('click'));
    expect(reload).not.toHaveBeenCalled();

    serviceWorker.dispatchEvent(new Event('controllerchange'));
    expect(reload).toHaveBeenCalledTimes(1);
    vi.unstubAllGlobals();
  });

  it('reuses the same toast id on repeated checks instead of stacking toasts', () => {
    const sw = fakeRegister();
    setupServiceWorkerUpdates(sw.register);

    sw.options().onNeedRefresh?.();
    sw.options().onNeedRefresh?.();

    const ids = vi.mocked(toast).mock.calls.map((call) => (call[1] as { id?: string }).id);
    expect(ids).toEqual([SW_UPDATE_TOAST_ID, SW_UPDATE_TOAST_ID]);
  });
});

describe('setupServiceWorkerUpdates cache purge', () => {
  it('purges the runtime API cache before handing over to the new worker', () => {
    const del = vi.fn().mockResolvedValue(true);
    vi.stubGlobal('caches', { delete: del });
    const sw = fakeRegister();
    setupServiceWorkerUpdates(sw.register);
    sw.options().onNeedRefresh?.();
    expect(del).not.toHaveBeenCalled();

    lastToastOptions().action?.onClick(new Event('click'));

    expect(del).toHaveBeenCalledWith(API_CACHE_NAME);
    vi.unstubAllGlobals();
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
// that reloads on its own interrupts everybody. These pin the two moments
// where neither is true.
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

  it('dismisses the fallback toast, so no stale prompt survives the handover', () => {
    stubBrowser();
    const sw = fakeRegister();
    setupServiceWorkerUpdates(sw.register);
    sw.options().onNeedRefresh?.();

    applyPendingServiceWorkerUpdate();

    expect(toast.dismiss).toHaveBeenCalledWith(SW_UPDATE_TOAST_ID);
  });

  // Two triggers and a toast can all fire for one waiting worker; a second
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
