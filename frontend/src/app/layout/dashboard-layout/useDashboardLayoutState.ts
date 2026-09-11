import { useSelectedComplex } from '@/features/complex/hooks/useSelectedComplex';
import { useRealtimeEvents } from '@/shared/hooks/useRealtimeEvents';

export function useDashboardLayoutState() {
  const { needsOnboarding, isLoading, selectedComplexId } = useSelectedComplex();
  useRealtimeEvents(selectedComplexId);

  return { needsOnboarding, isLoading, selectedComplexId };
}
