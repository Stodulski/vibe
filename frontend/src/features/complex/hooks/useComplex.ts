import { useQuery } from '@tanstack/react-query';
import { complexApi } from '../api/complex.api';
import { queryKeys } from '@/shared/lib/queryKeys';

export function useComplex(id: string | null) {
  // `enabled` gates the query on `id` being non-null, so `queryFn` only ever
  // runs once it's a real string.
  const complexId = id ?? '';
  return useQuery({
    queryKey: queryKeys.complexes.detail(complexId),
    queryFn: ({ signal }) => complexApi.getById(complexId, signal),
    select: (data) => data.complex,
    enabled: !!id,
    staleTime: 10 * 60 * 1000,
  });
}
