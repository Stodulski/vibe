import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useStore } from '@/shared/stores';
import type { Complex } from '@/shared/types/api.types';

export type OnboardingStep = 1 | 2 | 3;

interface UseOnboardingStepArgs {
  complexId: string | null;
  currentComplex: Complex | null;
  courts: unknown[] | undefined;
  complexesLoading: boolean;
  courtsLoading: boolean;
}

/** The step the server's own data implies, or `null` while still loading / once fully onboarded. */
function deriveStep({
  complexId,
  currentComplex,
  courts,
  complexesLoading,
  courtsLoading,
}: UseOnboardingStepArgs): OnboardingStep | null {
  if (complexesLoading) return null;
  if (!complexId) return 1;
  if (courtsLoading) return null;
  if (!courts || courts.length === 0) return 2;
  if (!currentComplex?.mp_user_id) return 3;
  return null; // Fully onboarded — no step to show, `completeOnboarding` takes over.
}

export function useOnboardingStep(args: UseOnboardingStepArgs) {
  const { complexId, currentComplex, courts, complexesLoading, courtsLoading } = args;
  const navigate = useNavigate();
  const setSelectedComplexId = useStore((s) => s.setSelectedComplexId);

  // Animation key for step transitions
  const [animKey, setAnimKey] = useState(0);
  // A manual override only — e.g. the "back" button. `null` means "derive
  // the step from server data" (see `deriveStep`), which is the default and
  // covers both the initial load and resuming an in-progress complex.
  const [manualStep, setManualStep] = useState<OnboardingStep | null>(null);

  const changeStep = useCallback((newStep: OnboardingStep) => {
    setAnimKey((k) => k + 1);
    setManualStep(newStep);
  }, []);

  // Consolidated onboarding completion -- single source of truth.
  const completeOnboarding = useCallback(
    (id: string) => {
      setSelectedComplexId(id);
      void navigate('/dashboard', { replace: true });
    },
    [navigate, setSelectedComplexId],
  );

  // A manual override from a previous complex (e.g. "Agregar otro complejo"
  // right after finishing one) shouldn't carry over to a new one — the new
  // complex's own server data should decide its step again. Adjusted here,
  // during render, rather than in an effect (react.dev/learn/you-might-not-need-an-effect#adjusting-some-state-when-a-prop-changes):
  // React discards this render and immediately re-renders with the reset
  // state, instead of committing one frame with the stale override first.
  const [prevComplexId, setPrevComplexId] = useState(complexId);
  if (complexId !== prevComplexId) {
    setPrevComplexId(complexId);
    setManualStep(null);
  }

  const derivedStep = deriveStep({
    complexId,
    currentComplex,
    courts,
    complexesLoading,
    courtsLoading,
  });
  const isFullyOnboarded =
    !complexesLoading && !courtsLoading && !!complexId && !!courts && courts.length > 0 && !!currentComplex?.mp_user_id;

  // Navigating away is a side effect and belongs in its own effect, not in
  // the render that derives `step` above.
  useEffect(() => {
    if (isFullyOnboarded && complexId) completeOnboarding(complexId);
  }, [isFullyOnboarded, complexId, completeOnboarding]);

  const step = manualStep ?? derivedStep;

  return { step, animKey, changeStep, completeOnboarding, navigate };
}
