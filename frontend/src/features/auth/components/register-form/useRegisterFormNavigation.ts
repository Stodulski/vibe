import { useState } from 'react';
import type { UseFormTrigger } from 'react-hook-form';
import type { RegisterDto } from '../../schemas/auth.schemas';

export type Step = 1 | 2 | 3;

/** Which RegisterDto fields each step validates before advancing. */
const STEP_FIELDS = {
  1: ['email'],
  2: ['first_name', 'last_name', 'phone'],
} as const;

/**
 * Step state and forward/back navigation for the 3-step register wizard.
 *
 * `goNext` takes the "advanced past step 1" callback per call, not up
 * front: that callback comes from `useAbandonedRegistrationLead(step, ...)`,
 * which itself needs this hook's `step` — passing it at call time avoids
 * that circular dependency.
 */
export function useRegisterFormNavigation(trigger: UseFormTrigger<RegisterDto>) {
  const [step, setStep] = useState<Step>(1);

  const goNext = async (fromStep: 1 | 2, onAdvanceFromStep1?: () => void) => {
    const valid = await trigger(STEP_FIELDS[fromStep]);
    if (valid) {
      if (fromStep === 1) onAdvanceFromStep1?.();
      setStep((fromStep + 1) as Step);
    }
  };

  const goBack = () => {
    setStep((s) => (s - 1) as Step);
  };

  return { step, goNext, goBack };
}
