import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { toast } from 'sonner';
import { http, HttpResponse } from 'msw';
import * as ky from '@/shared/lib/ky';
import { server } from '@/test/msw/server';
import { makeUser } from '@/test/factories';
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

// `bootstrapSession` reaches `refreshAccessToken` through the module's own
// binding, which a spy on the export cannot see; the wire is the proof.
let refreshHits = 0;

/** A drop-test's shared setup: a fresh mock stream and a refused refresh counted on the wire. */
function setupDropTest() {
  MockEventSource.reset();
  vi.stubGlobal('EventSource', MockEventSource);
  Object.defineProperty(document, 'hidden', { value: false, writable: true, configurable: true });
  refreshHits = 0;
  server.use(
    http.post('*/auth/refresh', () => {
      refreshHits++;
      return HttpResponse.json({ error: 'unauthorized' }, { status: 401 });
    }),
  );
}

/** Mounts the hook on `complex-1`, drops its stream with no named event first, and returns that stream. */
function dropFirstStream(): MockEventSource {
  renderHook(
    () => {
      useRealtimeEvents('complex-1');
    },
    { wrapper: createQueryWrapper() },
  );
  const es = firstInstance();
  es.onerror?.(new Event('error'));
  return es;
}

describe('useRealtimeEvents — ordinary stream drops, session alive', () => {
  beforeEach(setupDropTest);
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('reconnects after a plain drop without spending the refresh token while the session is alive', async () => {
    const es = dropFirstStream();

    expect(es.closed).toBe(true);
    await waitFor(
      () => {
        expect(MockEventSource.instances).toHaveLength(2);
      },
      { timeout: 5_000 },
    );
    expect(refreshHits).toBe(0);
  });

  it('refreshes after a plain drop when /auth/me says the access token is gone, then reconnects', async () => {
    let meCalls = 0;
    server.use(
      // Expired before the rotation, alive after it.
      http.get('*/auth/me', () => {
        meCalls++;
        return meCalls === 1
          ? HttpResponse.json({ error: 'unauthorized' }, { status: 401 })
          : HttpResponse.json({ user: makeUser(), pending_email: null, csrf_token: 'csrf-2' });
      }),
      http.post('*/auth/refresh', () => {
        refreshHits++;
        return HttpResponse.json({ csrf_token: 'csrf-2' });
      }),
    );
    dropFirstStream();

    await waitFor(
      () => {
        expect(MockEventSource.instances).toHaveLength(2);
      },
      { timeout: 5_000 },
    );
    expect(refreshHits).toBe(1);
  });
});

describe('useRealtimeEvents — ordinary stream drops, session gone or probe failing', () => {
  beforeEach(setupDropTest);
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('stops for good when /auth/me says the access token is gone and the refresh is refused', async () => {
    server.use(http.get('*/auth/me', () => HttpResponse.json({ error: 'unauthorized' }, { status: 401 })));
    dropFirstStream();

    await waitFor(() => {
      expect(refreshHits).toBeGreaterThanOrEqual(1);
    });
    // A wrongly scheduled reconnect would only fire after RETRY_DELAY, so
    // the check has to outlast it.
    await new Promise((resolve) => {
      setTimeout(resolve, 3_500);
    });
    expect(MockEventSource.instances).toHaveLength(1);
  }, 10_000);

  it('keeps retrying when the probe itself fails for a reason unrelated to the session', async () => {
    server.use(http.get('*/auth/me', () => HttpResponse.json({ error: 'boom' }, { status: 503 })));
    dropFirstStream();

    await waitFor(
      () => {
        expect(MockEventSource.instances).toHaveLength(2);
      },
      { timeout: 5_000 },
    );
    expect(refreshHits).toBe(0);
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
