import { renderHook, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import { useDashboardClientDetail } from './useDashboardClientDetail';
import type { Client, TopClient } from '@/shared/types/api.types';

const useClientActionsMock = vi.fn();
const useClientMock = vi.fn();

vi.mock('@/features/clients', () => ({
  useClientActions: (...args: unknown[]) => useClientActionsMock(...args) as unknown,
  useClient: (...args: unknown[]) => useClientMock(...args) as unknown,
}));

function createWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

const topClient: TopClient = {
  id: 'client-1',
  name: 'Juana Diaz',
  phone: '1122334455',
  booking_count: 3,
  total_spent: 45_000,
};

const realClient: Client = {
  id: 'client-1',
  complex_id: 'c1',
  first_name: 'Juana',
  last_name: 'Diaz',
  phone: '1122334455',
  is_blocked: false,
  total_bookings: 3,
  no_shows: 1,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

// Hoisted to file scope (rather than nested in the describe below) partly to
// keep that describe callback's own line count under the repo's
// max-lines-per-function cap, which counts a nested `beforeEach`'s lines as
// part of the enclosing describe.
beforeEach(() => {
  vi.clearAllMocks();
  useClientActionsMock.mockReturnValue({
    selectedClient: null,
    setDetailOpen: vi.fn(),
    detailOpen: false,
    handleSelectClient: vi.fn(),
  });
  useClientMock.mockReturnValue({ data: undefined });
});

describe('useDashboardClientDetail', () => {
  // 03-owner-pages-dashboard.md M8: the old code fabricated a full `Client`
  // (empty last_name, no_shows: 0, ...) from a `TopClient` just to satisfy
  // the type. Selecting a top client must not invent that data — it should
  // read null until the real record arrives.
  it('does not fabricate a placeholder Client for the selected top client', () => {
    const { result } = renderHook(() => useDashboardClientDetail('c1'), {
      wrapper: createWrapper(),
    });
    act(() => {
      result.current.handleSelectTopClient(topClient);
    });
    expect(result.current.selectedClient).toBeNull();
  });

  it('opens the detail sheet and asks useClient for the real record by id', () => {
    const setDetailOpen = vi.fn();
    useClientActionsMock.mockReturnValue({
      selectedClient: null,
      setDetailOpen,
      detailOpen: false,
      handleSelectClient: vi.fn(),
    });
    const { result } = renderHook(() => useDashboardClientDetail('c1'), {
      wrapper: createWrapper(),
    });
    act(() => {
      result.current.handleSelectTopClient(topClient);
    });
    expect(setDetailOpen).toHaveBeenCalledWith(true);
    expect(useClientMock).toHaveBeenLastCalledWith('c1', 'client-1');
  });

  it('shows the real client once useClient resolves it', () => {
    useClientMock.mockReturnValue({ data: { client: realClient, recent_bookings: [] } });
    const { result } = renderHook(() => useDashboardClientDetail('c1'), {
      wrapper: createWrapper(),
    });
    act(() => {
      result.current.handleSelectTopClient(topClient);
    });
    expect(result.current.selectedClient).toEqual(realClient);
  });

  it('clears the tracked top-client id when the detail sheet closes', () => {
    useClientMock.mockReturnValue({ data: { client: realClient, recent_bookings: [] } });
    const { result, rerender } = renderHook(() => useDashboardClientDetail('c1'), {
      wrapper: createWrapper(),
    });
    act(() => {
      result.current.handleSelectTopClient(topClient);
    });
    act(() => {
      result.current.setDetailOpen(false);
    });
    rerender();
    expect(useClientMock).toHaveBeenLastCalledWith('c1', null);
  });
});
