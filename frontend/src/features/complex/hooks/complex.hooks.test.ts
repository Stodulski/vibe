import { describe, it, expect, vi } from 'vitest';
import { createElement, type ReactNode } from 'react';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createQueryWrapper } from '@/test/test-utils';

vi.mock('../api/complex.api', () => ({
  complexApi: {
    list: vi.fn().mockResolvedValue({ complexes: [{ id: 'c1', name: 'Club' }] }),
    getById: vi.fn().mockResolvedValue({ complex: { id: 'c1' } }),
    delete: vi.fn().mockResolvedValue({ message: 'ok' }),
  },
}));

vi.mock('@/shared/lib/queryKeys', () => ({
  queryKeys: {
    complexes: {
      all: ['complexes'],
      detail: (id: string) => ['complexes', id],
      bySlug: (slug: string) => ['complexes', 'slug', slug],
      schedules: (id: string) => ['complexes', id, 'schedules'],
    },
  },
}));

const navigateMock = vi.fn();
vi.mock('react-router-dom', () => ({
  useNavigate: () => navigateMock,
}));

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

describe('useComplex', () => {
  it('is disabled when id is null', async () => {
    const { useComplex } = await import('./useComplex');
    const { result } = renderHook(() => useComplex(null), { wrapper: createQueryWrapper() });
    expect(result.current.fetchStatus).toBe('idle');
  });
});

describe('useDeleteComplex', () => {
  it('writes the cache to an empty list before navigating away', async () => {
    navigateMock.mockClear();
    const { useDeleteComplex } = await import('./useDeleteComplex');
    const { queryKeys } = await import('@/shared/lib/queryKeys');

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } },
    });
    queryClient.setQueryData(queryKeys.complexes.all, { complexes: [{ id: 'c1', name: 'Club' }] });

    // Snapshot the cache at the moment navigation fires — this is what would
    // have still shown the deleted complex if the cache write hadn't
    // happened first.
    let cacheAtNavigate: unknown;
    navigateMock.mockImplementationOnce(() => {
      cacheAtNavigate = queryClient.getQueryData(queryKeys.complexes.all);
    });

    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(() => useDeleteComplex(), { wrapper });

    await result.current.mutateAsync('c1');

    expect(cacheAtNavigate).toEqual({ complexes: [] });
    expect(queryClient.getQueryData(queryKeys.complexes.all)).toEqual({ complexes: [] });
    expect(navigateMock).toHaveBeenCalledWith('/onboarding', { replace: true });
  });

  it('really awaits the invalidation — navigation waits for an in-flight refetch triggered by it', async () => {
    navigateMock.mockClear();
    const { useDeleteComplex } = await import('./useDeleteComplex');
    const { useComplexes } = await import('./useComplexes');
    const { queryKeys } = await import('@/shared/lib/queryKeys');
    const { complexApi } = await import('../api/complex.api');

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } },
    });
    queryClient.setQueryData(queryKeys.complexes.all, { complexes: [{ id: 'c1', name: 'Club' }] });

    // `invalidateQueries` only awaits a real refetch when something is
    // actively observing the query — hold that refetch open on purpose so a
    // premature `navigate` (fired before the `await` actually resolves)
    // would be caught red-handed.
    // eslint-disable-next-line @typescript-eslint/no-empty-function -- reassigned below before it's ever called
    let resolveRefetch = () => {};
    vi.mocked(complexApi.list).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveRefetch = () => {
            resolve({ complexes: [] });
          };
        }),
    );

    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    // The active observer whose refetch `invalidateQueries` waits on.
    renderHook(() => useComplexes(), { wrapper });

    const { result } = renderHook(() => useDeleteComplex(), { wrapper });

    const mutatePromise = result.current.mutateAsync('c1');
    await waitFor(() => {
      expect(complexApi.list).toHaveBeenCalled();
    });
    expect(navigateMock).not.toHaveBeenCalled();

    resolveRefetch();
    await mutatePromise;

    expect(navigateMock).toHaveBeenCalledWith('/onboarding', { replace: true });
  });
});

describe('useSchedules', () => {
  it('is disabled when slug is undefined', async () => {
    const { useSchedules } = await import('./useSchedules');
    const { result } = renderHook(() => useSchedules('c1', undefined), {
      wrapper: createQueryWrapper(),
    });
    expect(result.current.fetchStatus).toBe('idle');
  });
});
