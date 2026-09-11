import { useQuery } from '@tanstack/react-query';
import { complexApi } from '../api/complex.api';
import { queryKeys } from '@/shared/lib/queryKeys';

export function useComplexes() {
  return useQuery({
    queryKey: queryKeys.complexes.all,
    queryFn: ({ signal }) => complexApi.list(signal),
    select: (data) => data.complexes,
    staleTime: 10 * 60 * 1000,
  });
}
