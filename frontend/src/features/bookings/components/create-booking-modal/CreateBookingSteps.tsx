import { useState } from 'react';
import { StepIndicator } from '@/shared/components/common/StepIndicator';
import { ES_AR } from '@/shared/i18n/es_AR';
import { submitHandler } from '@/shared/lib/form';
import { CreateBookingFormBody } from './CreateBookingFormBody';
import { SeriesConflictsPanel } from './SeriesConflictsPanel';
import type { CreateBookingStep } from './CreateBookingFormBody';
import type { useCreateBookingForm } from './useCreateBookingForm';
import type { CreateBookingDto } from '../../schemas/booking.schema';

const t = ES_AR;

const STEP_LABELS = [t.bookings.createBookingStep1, t.bookings.createBookingStep2, t.bookings.createBookingStep3];

/** Which `CreateBookingDto` fields each step validates before advancing. `price` (the manual
 * price input, step 1) has no schema rule of its own — whether it's required depends on
 * `priceRequired` (derived from `courts`/`schedules`, neither a form field), checked by hand
 * in `goNext` below instead. */
const STEP_FIELDS = {
  1: ['date', 'court_id', 'duration_minutes', 'start_time', 'price'],
  2: ['client_first_name', 'client_last_name', 'client_phone', 'client_email'],
} as const satisfies Record<1 | 2, (keyof CreateBookingDto)[]>;

/** The dates a weekly series could not take, with the two ways out (see `SeriesConflictsPanel`). */
function SeriesConflictsView({
  form,
  onClose,
  onRestart,
}: {
  form: ReturnType<typeof useCreateBookingForm>;
  onClose: () => void;
  onRestart: () => void;
}) {
  const { seriesSubmit } = form;
  return (
    <SeriesConflictsPanel
      conflicts={seriesSubmit.conflicts}
      pending={seriesSubmit.createSeries.isPending}
      onSkip={() => {
        seriesSubmit.submitSkippingConflicts(onClose);
      }}
      onBack={() => {
        seriesSubmit.dismissConflicts();
        onRestart();
      }}
    />
  );
}

/**
 * Owns the step cursor and renders the indicator + stepped body. Mounted
 * only inside `DialogContent`, which Radix unmounts on close — so a fresh
 * mount (rather than a reset effect) is what puts every reopen back on
 * step 1, matching how `key`-based remounts are the idiomatic way to reset
 * local state tied to a closed/reopened surface.
 */
export function CreateBookingSteps({
  form,
  onClose,
}: {
  form: ReturnType<typeof useCreateBookingForm>;
  onClose: () => void;
}) {
  const [step, setStep] = useState<CreateBookingStep>(1);

  const goNext = async (fromStep: 1 | 2) => {
    const valid = await form.trigger(STEP_FIELDS[fromStep]);
    if (!valid) return;

    // Not a schema rule (see `STEP_FIELDS`'s comment) — checked by hand so
    // step 1 can't be left without a price when one is required.
    if (fromStep === 1 && form.priceRequired && !form.price) {
      form.setError('price', { message: t.validation.manualPriceRequired });
      return;
    }

    // The end choice of a weekly series is not a schema field either.
    if (fromStep === 1 && !form.series.validate(form.date)) return;

    setStep((fromStep + 1) as CreateBookingStep);
  };

  const goBack = () => {
    setStep((s) => (s - 1) as CreateBookingStep);
  };

  return (
    <>
      <StepIndicator
        currentStep={step}
        totalSteps={3}
        stepLabels={STEP_LABELS}
        ariaLabel={t.bookings.createBookingStepProgress}
      />

      {form.seriesSubmit.conflicts.length > 0 ? (
        <SeriesConflictsView
          form={form}
          onClose={onClose}
          onRestart={() => {
            setStep(1);
          }}
        />
      ) : (
        <form
          onSubmit={submitHandler(form.handleSubmit, (data) => {
            form.onSubmit(data, onClose);
          })}
          className="space-y-5"
        >
          <CreateBookingFormBody
            step={step}
            form={form}
            onClose={onClose}
            onNext={(fromStep) => {
              void goNext(fromStep);
            }}
            onBack={goBack}
          />
        </form>
      )}
    </>
  );
}
