import { useQuery } from '@tanstack/react-query';
import { complexApi } from '../api/complex.api';
import { queryKeys } from '@/shared/lib/queryKeys';

export function useSchedules(complexId: string, slug: string | undefined) {
  // `enabled` gates the query on `slug` being non-undefined, so `queryFn`
  // only ever runs once it's a real string.
  const publicSlug = slug ?? '';
  return useQuery({
    queryKey: queryKeys.complexes.schedules(complexId),
    queryFn: ({ signal }) => complexApi.getPublicComplex(publicSlug, signal),
    select: (data) => data.schedules,
    enabled: !!slug,
    staleTime: 10 * 60 * 1000,
  });
}
