import { useQuery } from '@tanstack/react-query';
import { bookingsApi } from '../api/bookings.api';
import { queryKeys } from '@/shared/lib/queryKeys';

export function useBooking(complexId: string | null, bookingId: string | null) {
  return useQuery({
    queryKey: queryKeys.bookings.detail(bookingId ?? ''),
    queryFn: ({ signal }) => {
      // TanStack Query never invokes `queryFn` while `enabled: false`, so this
      // branch is defensive/type-only — it never actually executes.
      if (!complexId || !bookingId) return Promise.reject(new Error('complexId and bookingId are required'));
      return bookingsApi.getById(complexId, bookingId, signal);
    },
    enabled: !!complexId && !!bookingId,
    staleTime: 60 * 1000,
  });
}
