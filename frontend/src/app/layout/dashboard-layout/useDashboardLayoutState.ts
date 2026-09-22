import { useSelectedComplex } from '@/features/complex/hooks/useSelectedComplex';
import { useRealtimeEvents } from '@/shared/hooks/useRealtimeEvents';

export function useDashboardLayoutState() {
  const { needsOnboarding, isLoading, isFetching, selectedComplexId, isError, error, refetch, complexes } =
    useSelectedComplex();
  useRealtimeEvents(selectedComplexId);

  return { needsOnboarding, isLoading, isFetching, selectedComplexId, isError, error, refetch, complexes };
}
