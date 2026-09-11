import { useQuery } from '@tanstack/react-query';
import { queryKeys } from '@/shared/lib/queryKeys';
import { fetchMPFees } from '../api/mpFees.api';

const SIX_HOURS = 6 * 60 * 60 * 1000;

/**
 * The landing republishes this file rarely (its own `vigente_desde`/`generado`
 * fields track when the rates actually change), so a multi-hour `staleTime`
 * avoids refetching it on every settings-tab visit — unlike the app's own
 * 5-minute default for data that changes per booking.
 */
export function useMPFees() {
  return useQuery({
    queryKey: queryKeys.mpFees.costs,
    queryFn: ({ signal }) => fetchMPFees(signal),
    staleTime: SIX_HOURS,
    gcTime: SIX_HOURS,
    // No explicit `retry` — inherits the app-wide `queryClient` default (1
    // retry in production; `false` in tests via `test-utils`), the same as
    // every other query in this codebase.
  });
}
