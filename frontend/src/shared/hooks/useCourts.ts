import { useQuery } from '@tanstack/react-query';
import { listCourts } from '@/shared/api/courts.api';
import { queryKeys } from '@/shared/lib/queryKeys';

/**
 * The complex's courts with their prices. In `shared/` so the schedule tab
 * (`features/complex`) and the courts feature observe one cache entry; see
 * `shared/api/courts.api.ts`.
 */
export function useCourts(complexId: string | null) {
  return useQuery({
    queryKey: queryKeys.courts.byComplex(complexId ?? ''),
    queryFn: ({ signal }) => {
      // Invariant: `queryFn` only runs when `enabled` is true (i.e. when
      // `complexId` is non-null) — TanStack Query never invokes it
      // otherwise. This guard exists only to satisfy the type checker
      // without a non-null assertion.
      if (!complexId) return Promise.reject(new Error('useCourts: complexId is required'));
      return listCourts(complexId, signal);
    },
    select: (data) => data.courts,
    enabled: !!complexId,
    staleTime: 5 * 60 * 1000,
  });
}
