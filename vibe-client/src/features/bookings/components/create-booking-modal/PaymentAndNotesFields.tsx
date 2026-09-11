import { FormField } from '@/shared/components/common/FormField';
import { FieldRequirement } from '@/shared/components/common/FieldRequirement';
import { Textarea } from '@/shared/components/ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { UseFormSetValue, UseFormRegister, FieldErrors } from 'react-hook-form';
import type { CreateBookingDto } from '../../schemas/booking.schemas';

const t = ES_AR;

export function PaymentAndNotesFields({
  register,
  setValue,
  errors,
  paymentOption,
}: {
  register: UseFormRegister<CreateBookingDto>;
  setValue: UseFormSetValue<CreateBookingDto>;
  errors: FieldErrors<CreateBookingDto>;
  paymentOption: string | undefined;
}) {
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      <FormField
        label={
          <>
            {t.bookings.paymentMethod}
            <FieldRequirement required={paymentOption === 'deposit' || paymentOption === 'full'} />
          </>
        }
        htmlFor="booking-payment-method"
        error={errors.payment_method?.message}
      >
        <Select
          onValueChange={(v) => {
            setValue('payment_method', v as 'cash' | 'transfer');
          }}
        >
          <SelectTrigger id="booking-payment-method">
            <SelectValue placeholder={t.bookings.unspecified} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="cash">{t.bookings.paymentMethods.cash}</SelectItem>
            <SelectItem value="transfer">{t.bookings.paymentMethods.transfer}</SelectItem>
          </SelectContent>
        </Select>
      </FormField>
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
