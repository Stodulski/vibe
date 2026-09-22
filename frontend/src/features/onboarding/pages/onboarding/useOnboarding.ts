import { useCallback, useState } from 'react';
import { useLocation } from 'react-router-dom';
import { useLogout } from '@/features/auth';
import type { Complex } from '@/shared/types/api.types';
import { useOnboardingComplex } from './useOnboardingComplex';
import { useOnboardingStep, type OnboardingStep } from './useOnboardingStep';

export type { OnboardingStep };

export function useOnboarding() {
  const location = useLocation();

  // Accept complexId from location state (for MP connect of existing complex).
  const stateComplexId = (location.state as { complexId?: string } | null)?.complexId ?? null;

  // Track the complexId created during this session (step 1 -> step 2 transition).
  const [justCreatedId, setJustCreatedId] = useState<string | null>(null);

  const {
    complexId,
    currentComplex,
    courts,
    complexesLoading,
    courtsLoading,
    complexesError,
    complexesFetching,
    refetchComplexes,
  } = useOnboardingComplex({
    stateComplexId,
    justCreatedId,
  });

  const { step, animKey, changeStep, completeOnboarding, navigate } = useOnboardingStep({
    complexId,
    currentComplex,
    courts,
    complexesLoading,
    courtsLoading,
  });

  // --- Step 1: complex created ---
  const handleComplexCreated = useCallback(
    (created?: Complex) => {
      if (created) {
        setJustCreatedId(created.id);
        // The new complex also goes into history state.
        //
        // `justCreatedId` is component state and dies on reload; `location.state`
        // is part of the history entry and survives one. Without this, pressing
        // F5 on step 2 came back with `justCreatedId` gone, so
        // `useOnboardingComplex` resolved no complex at all, `deriveStep` read
        // that as "nothing created yet" and offered the create form again —
        // and filling it in created a second complex for a venue that already
        // had one. Replacing rather than pushing also keeps Back from returning
        // to an entry that would repeat the same trick.
        void navigate('/onboarding', { state: { complexId: created.id }, replace: true });
      }
      changeStep(2);
    },
    [changeStep, navigate],
  );

  const hasCourts = courts && courts.length > 0;

  const logout = useLogout();

  return {
    // State
    step,
    animKey,
    complexId,
    currentComplex,
    courts: courts ?? [],
    hasCourts: !!hasCourts,
    complexesError,
    complexesFetching,
    refetchComplexes,

    // Step navigation
    changeStep,
    completeOnboarding,

    // Step 1
    handleComplexCreated,

    // General
    logout,
    navigate,
  };
}
