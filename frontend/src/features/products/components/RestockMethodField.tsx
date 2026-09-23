import { FormField } from '@/shared/components/common/FormField';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import { COUNTER_PAYMENT_METHODS, type CounterPaymentMethod } from '@/shared/lib/paymentMethods';

const t = ES_AR;

/** Same shape as `features/cash`'s `MovementMethodField` — a restock is paid out of the till, so it shares the same closed method list. */
export function RestockMethodField({
  value,
  onChange,
  error,
}: {
  value: CounterPaymentMethod;
  onChange: (method: CounterPaymentMethod) => void;
  error?: string | undefined;
}) {
  return (
    <FormField label={t.products.restockMethodField} htmlFor="restock-method" error={error}>
      <Select
        value={value}
        onValueChange={(v) => {
          onChange(v as CounterPaymentMethod);
        }}
      >
        <SelectTrigger id="restock-method">
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
