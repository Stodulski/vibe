import { useQuery } from '@tanstack/react-query';
import api from '@/shared/lib/ky';
import { queryKeys } from '@/shared/lib/queryKeys';
import { parseWith } from '@/shared/lib/apiParse';
import { bookingsListResponseSchema } from '@/shared/schemas/booking.schema';
import type { BookingsListResponse } from '@/shared/types/api.types';

export function useBookingsByDate(complexId: string | null, date: string) {
  // `enabled` gates the query on `complexId` being non-null, so `queryFn`
  // only ever runs once it's a real string.
  const id = complexId ?? '';
  return useQuery({
    queryKey: queryKeys.bookings.byDate(id, date),
    queryFn: ({ signal }): Promise<BookingsListResponse> =>
      api
        .get(`complexes/${id}/bookings`, {
          searchParams: { date, limit: '200' },
          signal,
        })
        .json()
        .then(parseWith(bookingsListResponseSchema, 'useBookingsByDate')),
    select: (data) => data.bookings,
    enabled: !!complexId && !!date,
    staleTime: 5 * 60 * 1000,
  });
}
