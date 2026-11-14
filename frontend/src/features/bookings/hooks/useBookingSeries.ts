import { useQuery } from '@tanstack/react-query';
import { bookingSeriesApi } from '../api/bookingSeries.api';
import { queryKeys } from '@/shared/lib/queryKeys';

/** The fixed weekly series a booking belongs to; idle while `seriesId` is null (an ordinary booking). */
export function useBookingSeries(complexId: string | null, seriesId: string | null | undefined) {
  return useQuery({
    queryKey: queryKeys.bookings.series(complexId ?? '', seriesId ?? ''),
    queryFn: ({ signal }) => {
      // Never runs while `enabled` is false; defensive/type-only, like `useBooking`.
      if (!complexId || !seriesId) return Promise.reject(new Error('complexId and seriesId are required'));
      return bookingSeriesApi.get(complexId, seriesId, signal);
    },
    enabled: !!complexId && !!seriesId,
    staleTime: 60 * 1000,
  });
}
