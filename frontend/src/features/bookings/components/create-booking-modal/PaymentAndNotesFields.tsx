import { FormField } from '@/shared/components/common/FormField';
import { FieldRequirement } from '@/shared/components/common/FieldRequirement';
import { Textarea } from '@/shared/components/ui/textarea';
import { cn } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { PaymentMethodSelect } from './PaymentMethodSelect';
import type { UseFormSetValue, UseFormRegister, FieldErrors } from 'react-hook-form';
import type { CreateBookingDto } from '../../schemas/booking.schema';

const t = ES_AR;

export function PaymentAndNotesFields({
  register,
  setValue,
  errors,
  paymentOption,
  hidePayment = false,
}: {
  register: UseFormRegister<CreateBookingDto>;
  setValue: UseFormSetValue<CreateBookingDto>;
  errors: FieldErrors<CreateBookingDto>;
  paymentOption: string | undefined;
  /** A weekly series is charged per occurrence at the court: no payment method to pick. */
  hidePayment?: boolean;
}) {
  return (
    <div className={cn('grid grid-cols-1 gap-3', !hidePayment && 'sm:grid-cols-2')}>
      {!hidePayment && <PaymentMethodSelect setValue={setValue} errors={errors} paymentOption={paymentOption} />}
      <FormField
        label={
          <>
            {t.bookings.notes}
            <FieldRequirement required={false} />
          </>
        }
        htmlFor="booking-notes"
      >
        <Textarea
          id="booking-notes"
          {...register('notes')}
          placeholder={t.bookings.internalNotes}
          maxLength={500}
          className="min-h-[40px] resize-none"
        />
      </FormField>
    </div>
  );
}
