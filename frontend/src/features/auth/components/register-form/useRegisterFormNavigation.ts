import { useState } from 'react';
import type { UseFormTrigger } from 'react-hook-form';
import type { RegisterDto } from '../../schemas/auth.schema';

export type Step = 1 | 2 | 3;

/** Which RegisterDto fields each step validates before advancing. */
const STEP_FIELDS = {
  1: ['email'],
  2: ['first_name', 'last_name', 'phone'],
} as const;

/** Step state and forward/back navigation for the 3-step register wizard. */
export function useRegisterFormNavigation(trigger: UseFormTrigger<RegisterDto>) {
  const [step, setStep] = useState<Step>(1);

  const goNext = async (fromStep: 1 | 2) => {
    const valid = await trigger(STEP_FIELDS[fromStep]);
    if (valid) {
      setStep((fromStep + 1) as Step);
    }
  };

  const goBack = () => {
    setStep((s) => (s - 1) as Step);
  };

  return { step, goNext, goBack };
}
