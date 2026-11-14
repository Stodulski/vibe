import { useQuery } from '@tanstack/react-query';
import { eventsApi } from '../api/events.api';
import { queryKeys } from '@/shared/lib/queryKeys';

/** One event with everyone who held a spot in it. */
export function useEvent(complexId: string, eventId: string | null) {
  return useQuery({
    queryKey: queryKeys.events.detail(complexId, eventId ?? ''),
    queryFn: ({ signal }) => eventsApi.get(complexId, eventId ?? '', signal),
    enabled: !!complexId && !!eventId,
    // Payments land while the owner looks: a spot taken after the sheet opened shows up on refocus.
    staleTime: 30 * 1000,
  });
}
