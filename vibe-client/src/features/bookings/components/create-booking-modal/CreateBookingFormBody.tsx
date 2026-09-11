import { BookingScheduleFields } from './BookingScheduleFields';
import { ClientInfoFields } from './ClientInfoFields';
import { PaymentAndNotesFields } from './PaymentAndNotesFields';
import { PaymentOptionsSection } from './PaymentOptionsSection';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { useCreateBookingForm } from './useCreateBookingForm';

const t = ES_AR;

export type CreateBookingStep = 1 | 2 | 3;

interface CreateBookingFormBodyProps {
  step: CreateBookingStep;
  form: ReturnType<typeof useCreateBookingForm>;
  onClose: () => void;
  onNext: (fromStep: 1 | 2) => void;
  onBack: () => void;
}

/**
 * MercadoPago is not consulted here, and used to be: the submit was disabled
 * without it and step 1 opened with a warning saying bookings could not be
 * created. Neither was true. The server requires a MercadoPago payment for
 * PUBLIC bookings only (internal/bookings/public.go) — nothing in the owner's
 * path checks it — and this form's payment options are unpaid, deposit and
 * full, all of them recorded by hand for money that moved elsewhere.
 *
 * A club that takes cash could not write down a booking taken over the phone,
 * which is the oldest way a padel court gets reserved.
 *
 * The create-booking form's three steps (p.361 — easiest first), switched
 * on `step`. Each step ends in the shared `SectionFooter`, reused as the
 * step nav: `onCancel`/`cancelLabel` double as "Back", and `onSubmit`
 * (which makes the button `type="button"`) doubles as "Next" so only step 3
 * performs a real form submission.
 */
export function CreateBookingFormBody({ step, form, onClose, onNext, onBack }: CreateBookingFormBodyProps) {
  if (step === 1) {
    return (
      <>
        <BookingScheduleFields form={form} />
        <SectionFooter
          onCancel={onClose}
          submitLabel={t.common.next}
          onSubmit={() => {
            onNext(1);
          }}
        />
      </>
    );
  }

  if (step === 2) {
    return (
      <>
        <ClientInfoFields register={form.register} control={form.control} errors={form.errors} />
        <SectionFooter
          onCancel={onBack}
          cancelLabel={t.common.back}
          submitLabel={t.common.next}
          onSubmit={() => {
            onNext(2);
          }}
        />
      </>
    );
  }

  return (
    <>
      <PaymentAndNotesFields
        register={form.register}
        setValue={form.setValue}
        errors={form.errors}
        paymentOption={form.paymentOption}
      />
      <PaymentOptionsSection
        effectivePrice={form.effectivePrice}
        paymentOption={form.paymentOption}
        depositAmount={form.depositAmount}
        defaultDepositPesos={form.defaultDepositPesos}
        maxDepositPesos={form.maxDepositPesos}
        setValue={form.setValue}
        errors={form.errors}
      />
      <SectionFooter
        onCancel={onBack}
        cancelLabel={t.common.back}
        submitLabel={t.bookings.create}
        pending={form.createBooking.isPending}
      />
    </>
  );
}
