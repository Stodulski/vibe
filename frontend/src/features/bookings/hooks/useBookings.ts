import { useQuery } from '@tanstack/react-query';
import { bookingsApi } from '../api/bookings.api';
import { queryKeys } from '@/shared/lib/queryKeys';

export function useBookings(complexId: string | null, date: string) {
  return useQuery({
    queryKey: queryKeys.bookings.byDate(complexId ?? '', date),
    queryFn: ({ signal }) => {
      // TanStack Query never invokes `queryFn` while `enabled: false`, so this
      // branch is defensive/type-only — it never actually executes.
      if (!complexId) return Promise.reject(new Error('complexId is required'));
      return bookingsApi.list(complexId, date, undefined, undefined, signal);
    },
    select: (data) => data.bookings,
    enabled: !!complexId && !!date,
    staleTime: 5 * 60 * 1000,
  });
}
