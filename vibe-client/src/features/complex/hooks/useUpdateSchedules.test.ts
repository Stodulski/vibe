import { describe, it, expect, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import { useUpdateSchedules } from './useUpdateSchedules';
import { queryKeys } from '@/shared/lib/queryKeys';

vi.mock('../api/complex.api', () => ({
  complexApi: {
    updateSchedules: vi.fn().mockResolvedValue({
      schedules: [
        {
          id: 's1',
          complex_id: 'c1',
          day: 'monday',
          open_time: '09:00',
          close_time: '20:00',
          is_closed: false,
        },
      ],
    }),
  },
}));

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

function createWrapper(queryClient: QueryClient) {
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

// Finding M14: the query at `queryKeys.complexes.schedules(complexId)` is
// actually fed by `getPublicComplex(slug)` (see useSchedules.ts) and reduced
// with `select: (data) => data.schedules` — the cache entry itself holds the
// full `PublicComplexResponse`, not a bare `{ schedules }`. The mutation only
// has `{ schedules }` from `PUT .../schedules`, so writing it with
// `setQueryData` under the same key replaced that full response with a
// partial object any other reader of the key (usePriceConfigForm) would
// receive as if it were complete.
describe('useUpdateSchedules', () => {
  it('invalidates the schedules query instead of writing a mismatched shape into the cache', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const key = queryKeys.complexes.schedules('c1');

    // Seed the cache the way the real query populates it.
    const seeded = {
      complex: { id: 'c1', slug: 'padel-club' },
      courts: [],
      schedules: [
        {
          id: 's1',
          complex_id: 'c1',
          day: 'monday',
          open_time: '08:00',
          close_time: '23:00',
          is_closed: false,
        },
      ],
    };
    queryClient.setQueryData(key, seeded);

    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries');
    const setSpy = vi.spyOn(queryClient, 'setQueryData');

    const { result } = renderHook(() => useUpdateSchedules('c1'), {
      wrapper: createWrapper(queryClient),
    });

    result.current.mutate({
      schedules: [{ day: 'monday', open_time: '09:00', close_time: '20:00', is_closed: false }],
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateSpy).toHaveBeenCalledWith(expect.objectContaining({ queryKey: key }));
    expect(setSpy).not.toHaveBeenCalled();

    // The full response shape must survive untouched until the invalidated
    // query actually refetches — never replaced by the mutation's partial
    // `{ schedules }` payload.
    expect(queryClient.getQueryData(key)).toMatchObject({ complex: seeded.complex, courts: [] });
  });
});
