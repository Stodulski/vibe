import { toast } from 'sonner';
import type { registerSW } from 'virtual:pwa-register';
import { ES_AR } from '@/shared/i18n/es_AR';
import { setupServiceWorkerUpdates, SW_UPDATE_INTERVAL_MS, SW_UPDATE_TOAST_ID } from './serviceWorkerUpdate';

vi.mock('sonner', () => ({ toast: vi.fn() }));

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

function lastToastOptions() {
  const call = vi.mocked(toast).mock.calls.at(-1);
  if (!call) throw new Error('toast was not called');
  return call[1] as {
    id?: string;
    duration?: number;
    action?: { label: string; onClick: (event: unknown) => void };
  };
}

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
