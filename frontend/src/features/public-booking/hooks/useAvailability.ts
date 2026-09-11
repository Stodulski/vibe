import { useQuery, keepPreviousData } from '@tanstack/react-query';
import { publicBookingApi } from '../api/public-booking.api';
import { queryKeys } from '@/shared/lib/queryKeys';
import type { DurationMinutes } from '@/shared/types/api.types';

export function useAvailability(slug: string | undefined, date: string, duration: DurationMinutes) {
  // `enabled` gates the actual fetch; the queryKey/queryFn are still built
  // eagerly (TanStack Query's contract), so a real slug is never missing
  // when `queryFn` actually runs — the '' fallback is a type-level-only
  // placeholder, never observed by the API.
  const safeSlug = slug ?? '';
  return useQuery({
    // `duration` is part of the key (see `queryKeys.availability.bySlugAndDate`)
    // so switching 60/90/120 hits its own cache entry instead of serving
    // slots priced for the previous duration.
    queryKey: queryKeys.availability.bySlugAndDate(safeSlug, date, duration),
    queryFn: ({ signal }) => publicBookingApi.getAvailability(safeSlug, date, duration, signal),
    select: (data) => data.availability,
    enabled: !!slug && !!date,
    staleTime: 30 * 1000,
    // Duration and date each own a cache entry, so moving between them used to
    // empty the grid and redraw it from a skeleton. That is the wrong feeling
    // for a filter: someone comparing 60 against 90 minutes loses the very
    // thing they were comparing, mid-comparison. The previous answer stays on
    // screen — dimmed, and marked busy — until the new one replaces it in
    // place.
    placeholderData: keepPreviousData,
  });
}
