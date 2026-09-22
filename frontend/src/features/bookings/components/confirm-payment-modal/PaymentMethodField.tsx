import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { FormField } from '@/shared/components/common/FormField';
import { ES_AR } from '@/shared/i18n/es_AR';
import { COUNTER_PAYMENT_METHODS, type CounterPaymentMethod } from '../../lib/paymentMethods';
import type { UseFormSetValue, FieldErrors } from 'react-hook-form';
import type { ConfirmPaymentDto } from '../../schemas/booking.schema';

const t = ES_AR;

export function PaymentMethodField({
  setValue,
  errors,
}: {
  setValue: UseFormSetValue<ConfirmPaymentDto>;
  errors: FieldErrors<ConfirmPaymentDto>;
}) {
  return (
    <FormField
      className="space-y-2"
      label={t.bookings.paymentMethod}
      htmlFor="confirm-payment-method"
      error={errors.method?.message}
    >
      <Select
        defaultValue="cash"
        onValueChange={(v) => {
          setValue('method', v as CounterPaymentMethod);
        }}
      >
        <SelectTrigger id="confirm-payment-method">
          <SelectValue />
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
