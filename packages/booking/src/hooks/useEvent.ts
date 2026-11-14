import { useQuery } from '@tanstack/react-query';
import { HTTPError } from 'ky';
import { usePublicBookingApi } from '../api/context';
import { queryKeys } from '../lib/queryKeys';

/** One event for its sign-up page. A 404 is final; anything else gets a retry. */
export function useEvent(slug: string, eventId: string) {
  const api = usePublicBookingApi();
  return useQuery({
    queryKey: queryKeys.events.byId(slug, eventId),
    queryFn: ({ signal }) => api.getEvent(slug, eventId, signal),
    select: (data) => data.event,
    throwOnError: false,
    staleTime: 30_000,
    retry: (failureCount, error) => {
      if (error instanceof HTTPError && error.response.status === 404) return false;
      return failureCount < 2;
    },
  });
}
