import { useQuery } from '@tanstack/react-query';
import { cashApi } from '../api/cash.api';
import { queryKeys } from '@/shared/lib/queryKeys';

/**
 * Today's bookings that still owe money, for the sell page's "Turnos de hoy".
 * Only fetched while a till is open: the list is for charging at the counter,
 * and a closed till cannot charge anything.
 */
export function useBookingsDue(complexId: string | null, sessionId: string | null) {
  const id = complexId ?? '';
  return useQuery({
    queryKey: queryKeys.cash.bookingsDue(id),
    queryFn: ({ signal }) => cashApi.bookingsDue(id, signal),
    enabled: !!complexId && !!sessionId,
  });
}
