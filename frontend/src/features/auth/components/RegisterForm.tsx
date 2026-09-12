import { useRef, useState } from 'react';

import { zodResolver } from '@hookform/resolvers/zod';
import { registerSchema, type RegisterDto } from '../schemas/auth.schema';
import { useRegister } from '../hooks/useRegister';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useAppForm, submitHandler } from '@/shared/lib/form';
import { useTurnstileChallenge } from '@/shared/hooks/useTurnstileChallenge';
import type { TurnstileFieldHandle } from '@/shared/components/common/TurnstileField';
import { RegisterFormSteps } from './register-form/RegisterFormSteps';
import { StepIndicator } from '@/shared/components/common/StepIndicator';
import { RegisterLoginLink } from './register-form/RegisterLoginLink';
import { useAbandonedRegistrationLead } from './register-form/useAbandonedRegistrationLead';
import { useRegisterFormNavigation, type Step } from './register-form/useRegisterFormNavigation';

const t = ES_AR;

const REGISTER_STEP_LABELS = [t.auth.registerStep1, t.auth.registerStep2, t.auth.registerStep3];

/**
 * FORM-09: without these the inputs mount with `value === undefined`, so React
 * treats them as uncontrolled and then switches them to controlled on the first
 * keystroke — a dev warning, and a `reset()` that cannot put a field back to a
 * value it never had.
 */
const EMPTY_REGISTER_VALUES: RegisterDto = {
  first_name: '',
  last_name: '',
  email: '',
  phone: '',
  password: '',
  confirm_password: '',
};

function RegisterFormHeader({ step }: { step: Step }) {
  return (
    <div className="flex flex-col items-center gap-4 px-0 pt-2 pb-1 md:pt-6">
      <h1 className="font-display text-text-primary text-xl font-bold tracking-tight sm:text-2xl">{t.auth.register}</h1>
      <StepIndicator
        currentStep={step}
        totalSteps={3}
        stepLabels={REGISTER_STEP_LABELS}
        ariaLabel={t.auth.registerStepProgress}
      />
    </div>
  );
}

export function RegisterForm() {
  const turnstileRef = useRef<TurnstileFieldHandle>(null);
  const turnstile = useTurnstileChallenge();
  const [showPassword, setShowPassword] = useState(false);

  const {
    register,
    handleSubmit,
    control,
    trigger,
    getValues,
    formState: { errors },
  } = useAppForm<RegisterDto>({
    resolver: zodResolver(registerSchema),
    defaultValues: EMPTY_REGISTER_VALUES,
  });

  const { step, goNext, goBack } = useRegisterFormNavigation(trigger);
  const lead = useAbandonedRegistrationLead(getValues);
  const registerMutation = useRegister({
    resetTurnstile: () => turnstileRef.current?.reset(),
    onRegistered: lead.markRegistered,
  });

  const onSubmit = (formData: RegisterDto) => {
    const { confirm_password, ...data } = formData;
    registerMutation.mutate({ ...data, ...turnstile.payload() });
  };

  return (
    <div className="auth-card w-full">
      <RegisterFormHeader step={step} />

      {/* Form */}
      <div className="px-0 pt-4 pb-6">
        <form onSubmit={submitHandler(handleSubmit, onSubmit)} className="space-y-3 sm:space-y-3.5" noValidate>
          <RegisterFormSteps
            step={step}
            register={register}
            control={control}
            errors={errors}
            showPassword={showPassword}
            onToggleShowPassword={() => {
              setShowPassword(!showPassword);
            }}
            onNext={(fromStep) => {
              void goNext(fromStep);
            }}
            onBack={goBack}
            isPending={registerMutation.isPending}
            submitDisabled={turnstile.isBlocked(registerMutation.isPending)}
            turnstileRef={turnstileRef}
            onTurnstileTokenChange={turnstile.onTokenChange}
          />
        </form>

        <RegisterLoginLink />
      </div>
    </div>
  );
}
