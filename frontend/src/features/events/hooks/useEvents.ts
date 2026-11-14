import { useQuery } from '@tanstack/react-query';
import { eventsApi } from '../api/events.api';
import { queryKeys } from '@/shared/lib/queryKeys';

/** The complex's events whose date is in the range, cancelled ones included, soonest first. */
export function useEvents(complexId: string, dateFrom: string, dateTo: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.events.list(complexId, dateFrom, dateTo),
    queryFn: ({ signal }) => eventsApi.list(complexId, dateFrom, dateTo, signal),
    enabled: enabled && !!complexId,
    select: (data) => data.events,
  });
}
