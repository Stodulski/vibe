import { useComplexes } from './useComplexes';
import type { Complex } from '@/shared/types/api.types';

interface SelectedComplexState {
  complex: Complex | null;
  complexes: Complex[];
  selectedComplexId: string | null;
  needsOnboarding: boolean;
  isLoading: boolean;
  isFetching: boolean;
  isError: boolean;
  error: Error | null;
  refetch: () => void;
}

/**
 * An account owns at most one complex (`GET /complexes` returns 0 or 1
 * items), so there is nothing to select between — this derives the single
 * complex, if any, straight from the query. `selectedComplexId` keeps its
 * name so the many consumers destructuring it don't need to change.
 */
export function useSelectedComplex(): SelectedComplexState {
  const {
    data: complexes,
    isLoading,
    isFetching,
    isSuccess,
    isError,
    error,
    refetch: refetchComplexes,
  } = useComplexes();

  const complex = complexes?.[0] ?? null;
  const selectedComplexId = complex?.id ?? null;
  // Only a *successful* empty list means the account has no complex yet.
  // A failed query (network, 5xx, timeout) also leaves `complexes`
  // undefined, and used to read the same as "needs onboarding" — sending
  // the owner to the create-complex form, where submitting it 403'd with
  // "the account already owns a complex".
  const needsOnboarding = isSuccess && complexes.length === 0;

  return {
    complex,
    complexes: complexes ?? [],
    selectedComplexId,
    needsOnboarding,
    isLoading,
    isFetching,
    isError,
    error,
    // Widened to `() => void`: consumers trigger a retry and re-render off
    // `isLoading`/`isError` — none of them need the refetch promise itself.
    refetch: () => {
      void refetchComplexes();
    },
  };
}
