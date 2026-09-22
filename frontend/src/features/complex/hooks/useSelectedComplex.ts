import { useComplexes } from './useComplexes';
import type { Complex } from '@/shared/types/api.types';

interface SelectedComplexState {
  complex: Complex | null;
  complexes: Complex[];
  selectedComplexId: string | null;
  needsOnboarding: boolean;
  isLoading: boolean;
}

/**
 * An account owns at most one complex (`GET /complexes` returns 0 or 1
 * items), so there is nothing to select between — this derives the single
 * complex, if any, straight from the query. `selectedComplexId` keeps its
 * name so the many consumers destructuring it don't need to change.
 */
export function useSelectedComplex(): SelectedComplexState {
  const { data: complexes, isLoading } = useComplexes();

  const complex = complexes?.[0] ?? null;
  const selectedComplexId = complex?.id ?? null;
  const needsOnboarding = !isLoading && (!complexes || complexes.length === 0);

  return {
    complex,
    complexes: complexes ?? [],
    selectedComplexId,
    needsOnboarding,
    isLoading,
  };
}
