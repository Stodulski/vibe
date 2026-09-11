import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';

const { mockGet } = vi.hoisted(() => ({ mockGet: vi.fn() }));

vi.mock('@/shared/lib/ky', () => ({
  default: { get: mockGet, delete: vi.fn() },
  withSignal: (signal?: AbortSignal) => (signal ? { signal } : {}),
}));

vi.mock('@/shared/lib/mpAuth', () => ({
  generatePKCE: vi.fn().mockResolvedValue({ verifier: 'v1', challenge: 'c1' }),
  buildMPAuthUrl: vi.fn().mockReturnValue('https://mp.test/auth'),
}));

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

function jsonOf(body: unknown) {
  return { json: vi.fn().mockResolvedValue(body) };
}

function createWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

describe('useMPConnect mp/status response validation', () => {
  beforeEach(() => vi.clearAllMocks());

  it('resolves with a valid mp/status response', async () => {
    mockGet.mockReturnValue(jsonOf({ connected: true, mp_user_id: 'MP-1', app_id: 'app-1' }));
    const { useMPConnect } = await import('./useMPConnect');
    const { result } = renderHook(() => useMPConnect('c1'), { wrapper: createWrapper() });
    await waitFor(() => {
      expect(result.current.connected).toBe(true);
    });
  });

  // The task's schema wiring — before it, a malformed `connected` field would
  // have silently cast to whatever shape the caller declared, instead of
  // surfacing as a query error.
  it('reports isError instead of silently accepting a malformed connected field', async () => {
    mockGet.mockReturnValue(jsonOf({ connected: 'yes' }));
    const { useMPConnect } = await import('./useMPConnect');
    const { result } = renderHook(() => useMPConnect('c1'), { wrapper: createWrapper() });
    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
  });
});

describe('useMPConnect options', () => {
  beforeEach(() => vi.clearAllMocks());

  it('does not fetch the status query when enabled is false', async () => {
    mockGet.mockReturnValue(jsonOf({ connected: true, mp_user_id: 'MP-1', app_id: 'app-1' }));
    const { useMPConnect } = await import('./useMPConnect');
    const { result } = renderHook(() => useMPConnect('c1', { enabled: false }), {
      wrapper: createWrapper(),
    });
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(mockGet).not.toHaveBeenCalled();
    expect(result.current.connected).toBe(false);
  });

  it('polls the status query on the given refetchInterval', async () => {
    mockGet.mockReturnValue(jsonOf({ connected: false, app_id: 'app-1' }));
    const { useMPConnect } = await import('./useMPConnect');
    renderHook(() => useMPConnect('c1', { refetchInterval: 20 }), { wrapper: createWrapper() });
    await waitFor(() => {
      expect(mockGet).toHaveBeenCalledTimes(1);
    });
    await waitFor(
      () => {
        expect(mockGet.mock.calls.length).toBeGreaterThanOrEqual(2);
      },
      { timeout: 3000 },
    );
  });
});
