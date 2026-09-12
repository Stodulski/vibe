import { describe, it, expect, vi } from 'vitest';
import { act, renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement, type ReactNode } from 'react';
import { makeClient } from '@/test/factories';
import { useClientActions } from './useClientActions';

vi.mock('../api/clients.api', () => ({
  clientsApi: { getById: vi.fn(), update: vi.fn() },
}));

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return createElement(QueryClientProvider, { client: queryClient }, children);
}

describe('useClientActions — selection is an id resolved from the query cache, not a stored object (03-owner-pages-dashboard.md M8)', () => {
  it('replaces the selected client with the fresher one the cache resolves to', async () => {
    const { clientsApi } = await import('../api/clients.api');
    const fresh = makeClient({ id: 'cl1', is_blocked: true });
    vi.mocked(clientsApi.getById).mockResolvedValueOnce({ client: fresh, recent_bookings: [] });

    const { result } = renderHook(() => useClientActions('c1'), { wrapper });

    // Selected from a grid card that only ever had the stale "not blocked" copy.
    const staleFromGrid = makeClient({ id: 'cl1', is_blocked: false });
    act(() => {
      result.current.handleSelectClient(staleFromGrid);
    });

    expect(result.current.selectedClient?.is_blocked).toBe(false);
    await waitFor(() => {
      expect(result.current.selectedClient?.is_blocked).toBe(true);
    });
  });

  it('tracks the client to block separately from the one whose detail is open', async () => {
    const { clientsApi } = await import('../api/clients.api');
    vi.mocked(clientsApi.getById).mockResolvedValue({
      client: makeClient({ id: 'cl2' }),
      recent_bookings: [],
    });

    const { result } = renderHook(() => useClientActions('c1'), { wrapper });

    act(() => {
      result.current.handleSelectClient(makeClient({ id: 'cl1' }));
    });
    act(() => {
      result.current.handleBlockClient(makeClient({ id: 'cl2' }));
    });

    expect(result.current.selectedClient?.id).toBe('cl1');
    expect(result.current.blockClient?.id).toBe('cl2');
  });
});

// A sibling describe, not nested in the one above: max-lines-per-function
// counts a describe callback's whole body, so this stays a genuinely
// separate top-level call.
describe('useClientActions — block confirmation (03-owner-pages-dashboard.md M1/M8)', () => {
  it('reads the block direction from the fresh cached client, not a stale is_blocked snapshot', async () => {
    const { clientsApi } = await import('../api/clients.api');
    // Already blocked server-side by the time this session opened the menu —
    // confirming should unblock, not block again.
    vi.mocked(clientsApi.getById).mockResolvedValueOnce({
      client: makeClient({ id: 'cl1', is_blocked: true }),
      recent_bookings: [],
    });
    vi.mocked(clientsApi.update).mockResolvedValueOnce({
      client: makeClient({ id: 'cl1', is_blocked: false }),
    });

    const { result } = renderHook(() => useClientActions('c1'), { wrapper });

    act(() => {
      result.current.handleBlockClient(makeClient({ id: 'cl1', is_blocked: false }));
    });
    await waitFor(() => {
      expect(result.current.blockClient?.is_blocked).toBe(true);
    });

    act(() => {
      result.current.handleConfirmBlock();
    });

    await waitFor(() => {
      expect(clientsApi.update).toHaveBeenCalled();
    });
    expect(clientsApi.update).toHaveBeenCalledWith('c1', 'cl1', { is_blocked: false });
  });

  // 03-owner-pages-dashboard.md M1: `useUpdateClient(selectedComplexId ?? '')`
  // used to rely only on a comment saying `null` never reaches a confirm here.
  // This locks in the actual guard: with no complex selected, confirming a
  // block is a disabled no-op, not a request to `complexes//clients/:id`.
  it('does not call the API to confirm a block when no complex is selected', async () => {
    const { clientsApi } = await import('../api/clients.api');

    const { result } = renderHook(() => useClientActions(null), { wrapper });

    act(() => {
      result.current.handleBlockClient(makeClient({ id: 'cl1', is_blocked: false }));
    });
    act(() => {
      result.current.handleConfirmBlock();
    });

    expect(clientsApi.update).not.toHaveBeenCalled();
  });
});
