import { useQuery } from '@tanstack/react-query';
import { HTTPError } from 'ky';
import { publicBookingApi } from '../api/public-booking.api';
import { queryKeys } from '@/shared/lib/queryKeys';

export function useComplexBySlug(slug: string | undefined) {
  // `enabled` gates the actual fetch; a real slug is never missing when
  // `queryFn` actually runs — the '' fallback is a type-level-only
  // placeholder, never observed by the API.
  const safeSlug = slug ?? '';
  return useQuery({
    queryKey: queryKeys.publicComplex.bySlug(safeSlug),
    queryFn: ({ signal }) => publicBookingApi.getComplex(safeSlug, signal),
    enabled: !!slug,
    staleTime: 10 * 60 * 1000,
    retry: (failureCount, error) => {
      if (error instanceof HTTPError && error.response.status === 404) return false;
      return failureCount < 3;
    },
  });
}
