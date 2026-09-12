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
  // When coming from "Add complex", start fresh.
  const isNewComplex = (location.state as { newComplex?: boolean } | null)?.newComplex ?? false;

  // Track the complexId created during this session (step 1 -> step 2 transition).
  const [justCreatedId, setJustCreatedId] = useState<string | null>(null);

  const { complexId, currentComplex, courts, complexesLoading, courtsLoading } = useOnboardingComplex({
    isNewComplex,
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
      }
      changeStep(2);
    },
    [changeStep],
  );

  const hasCourts = courts && courts.length > 0;

  const logout = useLogout();

  return {
    // State
    step,
    animKey,
    isNewComplex,
    complexId,
    currentComplex,
    courts: courts ?? [],
    hasCourts: !!hasCourts,

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
