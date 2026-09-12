import { renderHook, waitFor } from '@testing-library/react';
import { toast } from 'sonner';
import * as ky from '@/shared/lib/ky';
import { useRealtimeEvents } from './useRealtimeEvents';
import { createQueryWrapper } from '@/test/test-utils';

vi.mock('@/shared/lib/queryKeys', () => ({
  queryKeys: {
    bookings: { byComplex: (id: string) => ['bookings', id] },
    dashboard: {
      stats: (id: string) => ['dashboard', 'stats', id],
      occupancy: (id: string) => ['dashboard', 'occupancy', id],
      clients: (id: string) => ['dashboard', 'clients', id],
    },
  },
}));

vi.mock('sonner', () => ({ toast: { error: vi.fn() } }));

// A spy wrapping the real `refreshAccessToken` rather than a stub: the
// network call it makes goes through the real `ky` client and is answered by
// the MSW handler for `POST auth/refresh` (src/test/msw/handlers.ts), so this
// still proves the hook calls (or doesn't call) the real refresh path — not
// just a mock. `vitest.config.ts`'s `restoreMocks: true` tears this spy back
// down to the un-instrumented function before every test, so it is
// re-created fresh in each `beforeEach` below rather than once here.
//
// The return type is captured through this helper (rather than writing out
// `vi.spyOn<typeof ky, 'refreshAccessToken'>` inline) because a `* as ky`
// namespace import is a readonly object type, and instantiating that generic
// explicitly against it does not resolve the same way plain inference does.
function spyOnRefreshAccessToken() {
  return vi.spyOn(ky, 'refreshAccessToken');
}
let refreshAccessToken: ReturnType<typeof spyOnRefreshAccessToken>;

class MockEventSource {
  url: string;
  withCredentials: boolean;
  readyState = 1;
  onerror: ((ev: Event) => void) | null = null;
  listeners: Record<string, EventListener[]> = {};
  closed = false;

  constructor(url: string, opts?: { withCredentials?: boolean }) {
    this.url = url;
    this.withCredentials = opts?.withCredentials ?? false;
    MockEventSource.instances.push(this);
  }

  addEventListener(type: string, listener: EventListener) {
    this.listeners[type] ??= [];
    this.listeners[type].push(listener);
  }

  removeEventListener() {
    /* noop mock */
  }

  close() {
    this.closed = true;
  }

  dispatchEvent(type: string) {
    this.listeners[type]?.forEach((fn) => {
      fn(new Event(type));
    });
  }

  static instances: MockEventSource[] = [];
  static reset() {
    MockEventSource.instances = [];
  }
}

function firstInstance(): MockEventSource {
  const es = MockEventSource.instances[0];
  if (!es) throw new Error('Expected a MockEventSource instance to exist');
  return es;
}

// Hoisted to file scope (rather than nested in the describe below) partly to
// keep that describe callback's own line count under the repo's
// max-lines-per-function cap, which counts a nested `beforeEach`/`afterEach`'s
// lines as part of the enclosing describe.
beforeEach(() => {
  MockEventSource.reset();
  vi.stubGlobal('EventSource', MockEventSource);
  Object.defineProperty(document, 'hidden', { value: false, writable: true, configurable: true });
  refreshAccessToken = spyOnRefreshAccessToken();
  vi.mocked(toast.error).mockReset();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('useRealtimeEvents', () => {
  it('does not connect when complexId is null', () => {
    renderHook(
      () => {
        useRealtimeEvents(null);
      },
      { wrapper: createQueryWrapper() },
    );
    expect(MockEventSource.instances).toHaveLength(0);
  });

  it('creates an EventSource with the correct URL when complexId is provided', () => {
    renderHook(
      () => {
        useRealtimeEvents('complex-1');
      },
      { wrapper: createQueryWrapper() },
    );
    expect(MockEventSource.instances).toHaveLength(1);
    expect(firstInstance().url).toContain('/complexes/complex-1/events');
  });

  it('sets withCredentials to true', () => {
    renderHook(
      () => {
        useRealtimeEvents('complex-1');
      },
      { wrapper: createQueryWrapper() },
    );
    expect(firstInstance().withCredentials).toBe(true);
  });

  it('registers a booking_changed event listener', () => {
    renderHook(
      () => {
        useRealtimeEvents('complex-1');
      },
      { wrapper: createQueryWrapper() },
    );
    const es = firstInstance();
    const bookingChangedListeners = es.listeners.booking_changed;
    expect(bookingChangedListeners).toBeDefined();
    expect(bookingChangedListeners?.length).toBe(1);
  });

  it('closes the EventSource on unmount', () => {
    const { unmount } = renderHook(
      () => {
        useRealtimeEvents('complex-1');
      },
      {
        wrapper: createQueryWrapper(),
      },
    );
    const es = firstInstance();
    expect(es.closed).toBe(false);
    unmount();
    expect(es.closed).toBe(true);
  });
});

describe('useRealtimeEvents — server-closed streams', () => {
  beforeEach(() => {
    MockEventSource.reset();
    vi.stubGlobal('EventSource', MockEventSource);
    Object.defineProperty(document, 'hidden', { value: false, writable: true, configurable: true });
    refreshAccessToken = spyOnRefreshAccessToken();
    vi.mocked(toast.error).mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('stops reconnecting after the server closes the stream as unauthorized', async () => {
    renderHook(
      () => {
        useRealtimeEvents('complex-1');
      },
      { wrapper: createQueryWrapper() },
    );
    const es = firstInstance();

    // The server sends the named event, then closes the connection — which is
    // what actually fires `onerror` on a real EventSource.
    es.dispatchEvent('unauthorized');
    es.onerror?.(new Event('error'));

    expect(es.closed).toBe(true);
    expect(toast.error).toHaveBeenCalledTimes(1);

    // Give any (wrongly) scheduled reconnect a chance to run.
    await new Promise((resolve) => {
      setTimeout(resolve, 0);
    });
    expect(refreshAccessToken).not.toHaveBeenCalled();
    expect(MockEventSource.instances).toHaveLength(1);
  });

  it('reconnects without a delay or a retry after the server closes the stream as expired', async () => {
    renderHook(
      () => {
        useRealtimeEvents('complex-1');
      },
      { wrapper: createQueryWrapper() },
    );
    const es = firstInstance();

    es.dispatchEvent('expired');
    es.onerror?.(new Event('error'));

    expect(es.closed).toBe(true);
    await waitFor(() => {
      expect(MockEventSource.instances).toHaveLength(2);
    });

    expect(refreshAccessToken).toHaveBeenCalledTimes(1);
    expect(toast.error).not.toHaveBeenCalled();
  });
});

describe('useRealtimeEvents — document unload', () => {
  it('closes the stream on pagehide and does not refresh the token from the error that follows', async () => {
    renderHook(
      () => {
        useRealtimeEvents('complex-1');
      },
      { wrapper: createQueryWrapper() },
    );
    const es = firstInstance();

    window.dispatchEvent(new Event('pagehide'));
    expect(es.closed).toBe(true);

    // The browser reports the dropped connection as an ordinary error.
    es.onerror?.(new Event('error'));

    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(refreshAccessToken).not.toHaveBeenCalled();
    expect(MockEventSource.instances).toHaveLength(1);
  });
});
