import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { createQueryWrapper } from '@/test/test-utils';
import { consumeMPOAuthSession } from '@/shared/lib/mpOAuthSession';

let attemptCount = 0;
vi.mock('@/shared/lib/mpAuth', () => ({
  createMPAuthAttempt: vi.fn(() => {
    attemptCount += 1;
    return Promise.resolve({
      state: `state-${String(attemptCount)}`,
      verifier: `v${String(attemptCount)}`,
      challenge: 'c1',
    });
  }),
  buildMPAuthUrl: vi.fn((attempt: { state: string }) => `https://mp.test/auth?state=${attempt.state}`),
}));

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

interface RegisteredAttempt {
  complexId: string;
  body: { state: string; code_verifier: string };
}

// Records every POST to mp/oauth-attempts. `gate`, when given, holds the
// response until it resolves, to observe the hook while registration is in flight.
function mockRegister(status = 204, gate?: Promise<void>) {
  const calls: RegisteredAttempt[] = [];
  server.use(
    http.post('*/complexes/:complexId/mp/oauth-attempts', async ({ request, params }) => {
      calls.push({
        complexId: String(params.complexId),
        body: (await request.json()) as RegisteredAttempt['body'],
      });
      if (gate) await gate;
      if (status === 204) return new HttpResponse(null, { status: 204 });
      return HttpResponse.json({ type: 'about:blank', title: 'nope', status }, { status });
    }),
  );
  return calls;
}

describe('useMPConnect mp/status response validation', () => {
  it('resolves with a valid mp/status response', async () => {
    server.use(
      http.get('*/complexes/:complexId/mp/status', () =>
        HttpResponse.json({ connected: true, mp_user_id: 'MP-1', app_id: 'app-1' }),
      ),
    );
    const { useMPConnect } = await import('./useMPConnect');
    const { result } = renderHook(() => useMPConnect('c1'), { wrapper: createQueryWrapper() });
    await waitFor(() => {
      expect(result.current.connected).toBe(true);
    });
  });

  // The task's schema wiring — before it, a malformed `connected` field would
  // have silently cast to whatever shape the caller declared, instead of
  // surfacing as a query error.
  it('reports isError instead of silently accepting a malformed connected field', async () => {
    server.use(http.get('*/complexes/:complexId/mp/status', () => HttpResponse.json({ connected: 'yes' })));
    const { useMPConnect } = await import('./useMPConnect');
    const { result } = renderHook(() => useMPConnect('c1'), { wrapper: createQueryWrapper() });
    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
  });
});

describe('useMPConnect options', () => {
  it('does not fetch the status query when enabled is false', async () => {
    let calls = 0;
    server.use(
      http.get('*/complexes/:complexId/mp/status', () => {
        calls += 1;
        return HttpResponse.json({ connected: true, mp_user_id: 'MP-1', app_id: 'app-1' });
      }),
    );
    const { useMPConnect } = await import('./useMPConnect');
    const { result } = renderHook(() => useMPConnect('c1', { enabled: false }), {
      wrapper: createQueryWrapper(),
    });
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(calls).toBe(0);
    expect(result.current.connected).toBe(false);
  });

  it('polls the status query on the given refetchInterval', async () => {
    let calls = 0;
    server.use(
      http.get('*/complexes/:complexId/mp/status', () => {
        calls += 1;
        return HttpResponse.json({ connected: false, app_id: 'app-1' });
      }),
    );
    const { useMPConnect } = await import('./useMPConnect');
    renderHook(() => useMPConnect('c1', { refetchInterval: 20 }), { wrapper: createQueryWrapper() });
    await waitFor(() => {
      expect(calls).toBeGreaterThanOrEqual(1);
    });
    // A 20ms refetch interval races `waitFor`'s own ~50ms polling interval,
    // so the first check above may already observe more than one call —
    // what actually matters is that the interval kept firing past the
    // initial fetch, not that this assertion catches it at exactly 1.
    await waitFor(
      () => {
        expect(calls).toBeGreaterThanOrEqual(2);
      },
      { timeout: 3000 },
    );
  });
});

describe('useMPConnect OAuth attempt', () => {
  beforeEach(() => {
    attemptCount = 0;
    localStorage.clear();
    sessionStorage.clear();
    server.use(
      http.get('*/complexes/:complexId/mp/status', () => HttpResponse.json({ connected: false, app_id: 'app-1' })),
    );
    mockRegister();
  });

  it('prepares the auth URL without persisting anything', async () => {
    const { useMPConnect } = await import('./useMPConnect');
    const { result } = renderHook(() => useMPConnect('c1'), { wrapper: createQueryWrapper() });
    await waitFor(() => {
      expect(result.current.authUrl).toBe('https://mp.test/auth?state=state-1');
    });
    expect(localStorage.length).toBe(0);
  });

  it('binds the verifier and return path to the state nonce on click', async () => {
    const { useMPConnect } = await import('./useMPConnect');
    const { result } = renderHook(() => useMPConnect('c1'), { wrapper: createQueryWrapper() });
    await waitFor(() => {
      expect(result.current.authUrl).not.toBeNull();
    });

    act(() => {
      result.current.handleConnectClick();
    });

    expect(consumeMPOAuthSession('state-1')).toEqual({
      complexId: 'c1',
      codeVerifier: 'v1',
      returnPath: window.location.pathname,
      createdAt: expect.any(Number) as number,
    });
  });

  it('does nothing on click while the attempt is still being prepared', async () => {
    const { useMPConnect } = await import('./useMPConnect');
    const { result } = renderHook(() => useMPConnect('c1'), { wrapper: createQueryWrapper() });
    act(() => {
      result.current.handleConnectClick();
    });
    expect(localStorage.length).toBe(0);
  });
});

describe('useMPConnect attempt rotation', () => {
  beforeEach(() => {
    attemptCount = 0;
    localStorage.clear();
    sessionStorage.clear();
    server.use(
      http.get('*/complexes/:complexId/mp/status', () => HttpResponse.json({ connected: false, app_id: 'app-1' })),
    );
    mockRegister();
  });

  it('prepares a fresh attempt after a persist, so a second tab gets its own state and verifier', async () => {
    const { useMPConnect } = await import('./useMPConnect');
    const { result } = renderHook(() => useMPConnect('c1'), { wrapper: createQueryWrapper() });
    await waitFor(() => {
      expect(result.current.authUrl).toBe('https://mp.test/auth?state=state-1');
    });

    act(() => {
      result.current.handleConnectClick();
    });
    await waitFor(() => {
      expect(result.current.authUrl).toBe('https://mp.test/auth?state=state-2');
    });
    act(() => {
      result.current.handleConnectClick();
    });

    // Both attempts stay resolvable: each new tab returns with its own state.
    expect(consumeMPOAuthSession('state-1')?.codeVerifier).toBe('v1');
    expect(consumeMPOAuthSession('state-2')?.codeVerifier).toBe('v2');
  });

  it('does not change the link under the owner before the fresh attempt is ready', async () => {
    const { useMPConnect } = await import('./useMPConnect');
    const { result } = renderHook(() => useMPConnect('c1'), { wrapper: createQueryWrapper() });
    await waitFor(() => {
      expect(result.current.authUrl).not.toBeNull();
    });

    act(() => {
      result.current.handleConnectClick();
    });

    expect(result.current.authUrl).not.toBeNull();
  });
});

// The callback often lands in another tab, browser or the installed PWA, where
// local storage is empty, so the server has to hold the attempt the link carries.
describe('useMPConnect attempt registration', () => {
  beforeEach(() => {
    attemptCount = 0;
    localStorage.clear();
    sessionStorage.clear();
    server.use(
      http.get('*/complexes/:complexId/mp/status', () => HttpResponse.json({ connected: false, app_id: 'app-1' })),
    );
  });

  it('enables the link only after the server stored the state and verifier the link carries', async () => {
    let release!: () => void;
    const calls = mockRegister(
      204,
      new Promise<void>((resolve) => {
        release = resolve;
      }),
    );
    const { useMPConnect } = await import('./useMPConnect');
    const { result } = renderHook(() => useMPConnect('c1'), { wrapper: createQueryWrapper() });

    await waitFor(() => {
      expect(calls).toHaveLength(1);
    });
    expect(calls[0]).toEqual({ complexId: 'c1', body: { state: 'state-1', code_verifier: 'v1' } });
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(result.current.authUrl).toBeNull();

    release();
    await waitFor(() => {
      expect(result.current.authUrl).toBe('https://mp.test/auth?state=state-1');
    });
  });

  it('still enables the link when the server refuses the attempt', async () => {
    const calls = mockRegister(500);
    const { useMPConnect } = await import('./useMPConnect');
    const { result } = renderHook(() => useMPConnect('c1'), { wrapper: createQueryWrapper() });

    await waitFor(() => {
      expect(result.current.authUrl).toBe('https://mp.test/auth?state=state-1');
    });
    expect(calls).toHaveLength(1);
  });

  it('prepares nothing for a complex that is already connected', async () => {
    server.use(
      http.get('*/complexes/:complexId/mp/status', () =>
        HttpResponse.json({ connected: true, mp_user_id: 'MP-1', app_id: 'app-1' }),
      ),
    );
    const calls = mockRegister();
    const { useMPConnect } = await import('./useMPConnect');
    const { result } = renderHook(() => useMPConnect('c1'), { wrapper: createQueryWrapper() });

    await waitFor(() => {
      expect(result.current.connected).toBe(true);
    });
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(calls).toHaveLength(0);
    expect(result.current.authUrl).toBeNull();
  });
});

describe('useMPConnect attempt refresh', () => {
  beforeEach(() => {
    attemptCount = 0;
    localStorage.clear();
    sessionStorage.clear();
    server.use(
      http.get('*/complexes/:complexId/mp/status', () => HttpResponse.json({ connected: false, app_id: 'app-1' })),
    );
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // The server keeps an attempt for an hour; a card left open longer than that
  // must not offer a link whose state the server has already forgotten.
  it('registers and links a fresh attempt every 50 minutes while mounted', async () => {
    // Fake timers would also freeze `waitFor`'s own polling, so the hook's
    // interval is captured instead and fired by hand.
    const refreshTicks: (() => void)[] = [];
    const realSetInterval = globalThis.setInterval;
    vi.spyOn(globalThis, 'setInterval').mockImplementation(((handler: () => void, ms?: number) => {
      if (ms === 50 * 60 * 1000) refreshTicks.push(handler);
      return realSetInterval(handler, ms);
    }) as typeof setInterval);
    const calls = mockRegister();
    const { useMPConnect } = await import('./useMPConnect');
    const { result } = renderHook(() => useMPConnect('c1'), { wrapper: createQueryWrapper() });
    await waitFor(() => {
      expect(result.current.authUrl).toBe('https://mp.test/auth?state=state-1');
    });

    expect(refreshTicks).toHaveLength(1);
    act(() => {
      refreshTicks[0]?.();
    });

    await waitFor(() => {
      expect(result.current.authUrl).toBe('https://mp.test/auth?state=state-2');
    });
    expect(calls.map((c) => c.body.state)).toEqual(['state-1', 'state-2']);
  });
});
