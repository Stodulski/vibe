import { describe, it, expect, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import { createQueryWrapper } from '@/test/test-utils';

vi.mock('@/shared/lib/mpAuth', () => ({
  generatePKCE: vi.fn().mockResolvedValue({ verifier: 'v1', challenge: 'c1' }),
  buildMPAuthUrl: vi.fn().mockReturnValue('https://mp.test/auth'),
}));

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

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
