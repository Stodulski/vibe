import { FormField } from '@/shared/components/common/FormField';
import { FieldRequirement } from '@/shared/components/common/FieldRequirement';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import { COUNTER_PAYMENT_METHODS, type CounterPaymentMethod } from '../../lib/paymentMethods';
import type { UseFormSetValue, FieldErrors } from 'react-hook-form';
import type { CreateBookingDto } from '../../schemas/booking.schema';

const t = ES_AR;

export function PaymentMethodSelect({
  setValue,
  errors,
  paymentOption,
}: {
  setValue: UseFormSetValue<CreateBookingDto>;
  errors: FieldErrors<CreateBookingDto>;
  paymentOption: string | undefined;
}) {
  return (
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
          setValue('payment_method', v as CounterPaymentMethod);
        }}
      >
        <SelectTrigger id="booking-payment-method">
          <SelectValue placeholder={t.bookings.unspecified} />
        </SelectTrigger>
        <SelectContent>
          {COUNTER_PAYMENT_METHODS.map((method) => (
            <SelectItem key={method} value={method}>
              {t.bookings.paymentMethods[method]}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </FormField>
  );
}
