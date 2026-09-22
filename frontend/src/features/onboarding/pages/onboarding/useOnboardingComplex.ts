import { useComplexes } from '@/features/complex';
import { useCourts } from '@/features/courts';

interface UseOnboardingComplexArgs {
  stateComplexId: string | null;
  justCreatedId: string | null;
}

/**
 * An account owns at most one complex, so `useComplexes()` already answers
 * which one is being onboarded once it has loaded. Before that — or right
 * after creating it, before the query catches up — `stateComplexId` (an
 * MP-connect deep link) or `justCreatedId` (this session's own create) fill
 * the gap.
 */
export function useOnboardingComplex({ stateComplexId, justCreatedId }: UseOnboardingComplexArgs) {
  const {
    data: complexes,
    isLoading: complexesLoading,
    isError: complexesError,
    refetch: refetchComplexes,
  } = useComplexes();

  const complexId = complexes?.[0]?.id ?? stateComplexId ?? justCreatedId ?? null;
  const currentComplex = complexes?.[0] ?? null;

  // Fetch courts for the complex to determine if step 2 is complete.
  const { data: courts, isLoading: courtsLoading } = useCourts(complexId);

  return { complexId, currentComplex, courts, complexesLoading, courtsLoading, complexesError, refetchComplexes };
}
