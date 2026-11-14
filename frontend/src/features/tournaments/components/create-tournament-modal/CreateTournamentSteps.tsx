import { useState, type SyntheticEvent } from 'react';
import { type FieldErrors } from 'react-hook-form';
import { StepIndicator } from '@/shared/components/common/StepIndicator';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { CategoriesStep } from './CategoriesStep';
import { ReviewStep } from './ReviewStep';
import { BASICS_FIELDS, stepForErrors, stepForServerError, type CreateStep } from './stepFields';
import { TournamentStep } from './TournamentStep';
import { useCreateTournamentForm } from './useCreateTournamentForm';
import type { TournamentFormValues } from '../../schemas/tournamentForm.schema';

const t = ES_AR.tournaments;
const STEP_LABELS = [t.createSteps.step1, t.createSteps.step2, t.createSteps.step3];

/**
 * Owns the step cursor, the indicator and the single form. Mounted only while the dialog is open,
 * so every opening starts on step 1 with a blank form. A step cannot be left with errors in its
 * own fields; going back keeps every value.
 */
export function CreateTournamentSteps({ complexId, onClose }: { complexId: string; onClose: () => void }) {
  const flow = useCreateTournamentForm(complexId, onClose);
  const { form } = flow;
  const [step, setStep] = useState<CreateStep>(1);
  const message = flow.error ? getHttpErrorMessage(flow.error, t.form.createError) : undefined;

  const goTo = (to: CreateStep) => {
    flow.clearError();
    setStep(to);
  };
  const leaveBasics = async () => {
    if (await form.trigger(BASICS_FIELDS)) goTo(2);
  };
  const onInvalid = (errors: FieldErrors<TournamentFormValues>) => {
    goTo(stepForErrors(Object.keys(errors)));
  };

  const onSubmit =
    step === 3
      ? flow.submit({
          // Not goTo: the step shows the error, so it must not be cleared on the way.
          onError: (error) => {
            setStep(stepForServerError(error));
          },
          onInvalid,
        })
      : (event: SyntheticEvent<HTMLFormElement>) => {
          event.preventDefault();
          if (step === 1) void leaveBasics();
        };

  return (
    <>
      <StepIndicator currentStep={step} totalSteps={3} stepLabels={STEP_LABELS} ariaLabel={t.createSteps.progress} />
      <form onSubmit={onSubmit} className="space-y-4" noValidate>
        {step === 1 && (
          <TournamentStep form={form} error={message} onCancel={onClose} onNext={() => void leaveBasics()} />
        )}
        {step === 2 && (
          <CategoriesStep
            flow={flow}
            onBack={() => {
              goTo(1);
            }}
            onNext={() => {
              goTo(3);
            }}
          />
        )}
        {step === 3 && (
          <ReviewStep
            values={form.getValues()}
            error={message}
            pending={flow.isPending}
            onBack={() => {
              goTo(2);
            }}
          />
        )}
      </form>
    </>
  );
}
