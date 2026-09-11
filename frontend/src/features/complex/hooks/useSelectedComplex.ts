import { useComplexes } from './useComplexes';
import { useStore } from '@/shared/stores';
import type { Complex } from '@/shared/types/api.types';

interface SelectedComplexState {
  complex: Complex | null;
  complexes: Complex[];
  selectedComplexId: string | null;
  needsOnboarding: boolean;
  isLoading: boolean;
}

/**
 * The effective complex id: whatever is stored, if it still names a complex
 * the owner actually has, otherwise the first one. Derived every render
 * instead of written back into the store by an effect — the effect used to
 * leave one render where `complexes` had arrived but the store's id hadn't
 * been corrected yet, which `isLoading` had to paper over as "still loading"
 * even though the query itself was done. A stale or absent id resolving to
 * the first complex in the same render removes that gap outright.
 */
function effectiveComplexId(complexes: Complex[] | undefined, storedId: string | null): string | null {
  if (complexes?.some((c) => c.id === storedId)) return storedId;
  return complexes?.[0]?.id ?? null;
}

export function useSelectedComplex(): SelectedComplexState {
  const storedId = useStore((s) => s.selectedComplexId);
  const { data: complexes, isLoading } = useComplexes();

  const selectedComplexId = effectiveComplexId(complexes, storedId);
  const complex = complexes?.find((c) => c.id === selectedComplexId) ?? null;
  const needsOnboarding = !isLoading && (!complexes || complexes.length === 0);

  return {
    complex,
    complexes: complexes ?? [],
    selectedComplexId,
    needsOnboarding,
    isLoading,
  };
}
