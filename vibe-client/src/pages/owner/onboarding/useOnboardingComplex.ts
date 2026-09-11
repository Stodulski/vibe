import { useMemo } from 'react';
import { useComplexes } from '@/features/complex';
import { useCourts } from '@/features/courts';

interface UseOnboardingComplexArgs {
  isNewComplex: boolean;
  stateComplexId: string | null;
  justCreatedId: string | null;
}

export function useOnboardingComplex({ isNewComplex, stateComplexId, justCreatedId }: UseOnboardingComplexArgs) {
  const { data: complexes, isLoading: complexesLoading } = useComplexes();

  // The newest complex, as the fallback when nothing said which one.
  //
  // This used to hunt for "the first incomplete complex", and defined
  // incomplete as `!c.mp_user_id`. Since online payment became optional that
  // is actively harmful: an owner who chose "Lo hago después" and finished the
  // wizard, landing on /onboarding again by any route, was picked up by this
  // fallback and sent back to the MercadoPago step of a complex that is done.
  // We opened the exit and left the road that carries them back in.
  //
  // Missing courts is what incomplete means — a venue with none can take no
  // booking at all — and this hook cannot see them: `useComplexes` returns no
  // court count. So it stops guessing. `useOnboardingStep` already derives the
  // right step from the server once a complex is chosen; the only job left
  // here is choosing one when the caller did not.
  const incompleteComplex = useMemo(() => {
    if (!complexes || complexes.length === 0) return null;
    return (
      [...complexes].sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())[0] ?? null
    );
  }, [complexes]);

  // The complex being onboarded: explicit state > just created > incomplete from API.
  // When creating a new complex, don't pick an existing one.
  const complexId = isNewComplex ? justCreatedId : (stateComplexId ?? justCreatedId ?? incompleteComplex?.id ?? null);

  // The full complex object for the current session.
  const currentComplex = complexes?.find((c) => c.id === complexId) ?? null;

  // Fetch courts for the complex to determine if step 2 is complete.
  const { data: courts, isLoading: courtsLoading } = useCourts(complexId);

  return { complexId, currentComplex, courts, complexesLoading, courtsLoading };
}
