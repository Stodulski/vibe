import { useEffect } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { useComplexes } from '@/features/complex';
import { useStore } from '@/shared/stores';
import type { Complex } from '@/shared/types/api.types';

export function useComplexSelector() {
  const navigate = useNavigate();
  const location = useLocation();
  const { data: complexes, isLoading, isError, refetch } = useComplexes();
  const { setSelectedComplexId } = useStore();

  // Where to go after selecting a complex
  const fromPage = (location.state as { from?: string } | null)?.from;
  const returnTo = fromPage ?? '/dashboard';

  // Auto-select if user has exactly 1 complex and didn't navigate here intentionally
  useEffect(() => {
    if (fromPage) return; // User explicitly navigated here — let them see the page
    if (complexes?.length !== 1) return;
    const only = complexes[0];
    if (!only) return;
    setSelectedComplexId(only.id);
    void navigate('/dashboard', { replace: true });
  }, [complexes, navigate, setSelectedComplexId, fromPage]);

  const handleSelectComplex = (complex: Complex) => {
    setSelectedComplexId(complex.id);

    // A club with no courts goes to its own onboarding, not to the dashboard —
    // there is nothing to run there yet, and the dashboard is where the
    // problem is NOT. The id travels explicitly: the onboarding's fallback
    // picks a complex on its own when nobody says which, and with two clubs an
    // owner would land on the wrong one.
    if (complex.court_count === 0) {
      void navigate('/onboarding', { state: { complexId: complex.id } });
      return;
    }

    void navigate(returnTo, { replace: true });
  };

  const handleAddComplex = () => {
    void navigate('/onboarding', { state: { newComplex: true } });
  };

  return { complexes, isLoading, isError, refetch, handleSelectComplex, handleAddComplex };
}
