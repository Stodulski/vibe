import type { Ref } from 'react';
import type { Control, FieldErrors, UseFormRegister } from 'react-hook-form';
import type { RegisterDto } from '../../schemas/auth.schemas';
import { TurnstileField, type TurnstileFieldHandle } from '@/shared/components/common/TurnstileField';
import { RegisterEmailField } from './RegisterEmailField';
import { RegisterContactFields } from './RegisterContactFields';
import { RegisterPasswordFields } from './RegisterPasswordFields';
import { RegisterFormFooter } from './RegisterFormFooter';
import { RegisterStepNav } from './RegisterStepNav';
import { GoogleSignInSection } from '../GoogleSignInSection';

interface RegisterFormStepsProps {
  step: 1 | 2 | 3;
  register: UseFormRegister<RegisterDto>;
  control: Control<RegisterDto>;
  errors: FieldErrors<RegisterDto>;
  showPassword: boolean;
  onToggleShowPassword: () => void;
  onNext: (fromStep: 1 | 2) => void;
  onBack: () => void;
  isPending: boolean;
  /** `isPending`, plus a Turnstile challenge configured but not yet solved (step 3 only). */
  submitDisabled: boolean;
  turnstileRef: Ref<TurnstileFieldHandle>;
  onTurnstileTokenChange: (token: string | undefined) => void;
}

/** The three register-form steps, switched on `step`. */
export function RegisterFormSteps({
  step,
  register,
  control,
  errors,
  showPassword,
  onToggleShowPassword,
  onNext,
  onBack,
  isPending,
  submitDisabled,
  turnstileRef,
  onTurnstileTokenChange,
}: RegisterFormStepsProps) {
  if (step === 1) {
    return (
      <>
        <RegisterEmailField register={register} errors={errors} />
        <RegisterStepNav
          onNext={() => {
            onNext(1);
          }}
        />
        <GoogleSignInSection />
      </>
    );
  }

  if (step === 2) {
    return (
      <>
        <RegisterContactFields register={register} control={control} errors={errors} />
        <RegisterStepNav
          onBack={onBack}
          onNext={() => {
            onNext(2);
          }}
        />
      </>
    );
  }

  return (
    <>
      <RegisterPasswordFields
        register={register}
        errors={errors}
        showPassword={showPassword}
        onToggleShowPassword={onToggleShowPassword}
      />
      <TurnstileField ref={turnstileRef} onTokenChange={onTurnstileTokenChange} />
      <RegisterFormFooter isPending={isPending} submitDisabled={submitDisabled} onBack={onBack} />
    </>
  );
}
