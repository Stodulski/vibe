import { FormField } from '@/shared/components/common/FormField';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import { COUNTER_PAYMENT_METHODS, type CounterPaymentMethod } from '@/shared/lib/paymentMethods';

const t = ES_AR;

export function MovementMethodField({
  value,
  onChange,
  error,
}: {
  value: CounterPaymentMethod;
  onChange: (method: CounterPaymentMethod) => void;
  error?: string | undefined;
}) {
  return (
    <FormField label={t.cash.movementMethod} htmlFor="movement-method" error={error}>
      <Select
        value={value}
        onValueChange={(v) => {
          onChange(v as CounterPaymentMethod);
        }}
      >
        <SelectTrigger id="movement-method">
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
