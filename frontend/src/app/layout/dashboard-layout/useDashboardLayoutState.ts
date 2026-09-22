import { useSelectedComplex } from '@/features/complex/hooks/useSelectedComplex';
import { useRealtimeEvents } from '@/shared/hooks/useRealtimeEvents';

export function useDashboardLayoutState() {
  const { needsOnboarding, isLoading, selectedComplexId, isError, error, refetch } = useSelectedComplex();
  useRealtimeEvents(selectedComplexId);

  return { needsOnboarding, isLoading, selectedComplexId, isError, error, refetch };
}
