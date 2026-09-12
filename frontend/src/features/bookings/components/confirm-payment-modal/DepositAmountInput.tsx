import { Banknote } from 'lucide-react';
import type { UseFormSetValue, FieldErrors } from 'react-hook-form';
import { Input } from '@/shared/components/ui/input';
import { FormField } from '@/shared/components/common/FormField';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { ConfirmPaymentDto } from '../../schemas/booking.schema';
import type { Booking } from '@/shared/types/api.types';

const t = ES_AR;

/**
 * How much of the total is being taken now, when it is not the whole thing.
 *
 * Its own component because it is the only part of the payment fields that
 * does any work: everything around it is a select with two options, while this
 * clamps what was typed and converts pesos to the cents the API stores.
 *
 * The clamp is not decoration. `max` on the input is a hint a browser may
 * enforce and a keyboard may not, so the same ceiling is applied again here —
 * a deposit larger than the booking is refused by the server anyway, and being
 * told so after submitting is worse than not being able to type it.
 */
export function DepositAmountInput({
  booking,
  setValue,
  errors,
}: {
  booking: Booking | null;
  setValue: UseFormSetValue<ConfirmPaymentDto>;
  errors: FieldErrors<ConfirmPaymentDto>;
}) {
  const maxPesos = booking ? booking.price / 100 : Infinity;

  return (
    <FormField
      className="space-y-2"
      label={t.bookings.depositAmountInput}
      htmlFor="deposit-input"
      error={errors.amount?.message}
    >
      <div className="relative">
        <Banknote className="text-text-tertiary absolute top-1/2 left-3 size-4 -translate-y-1/2" />
        <Input
          id="deposit-input"
          type="number"
          className="pl-9"
          min={1}
          max={booking ? maxPesos : undefined}
          placeholder={t.placeholders.depositAmount}
          onChange={(e) => {
            const pesos = e.target.valueAsNumber;
            if (Number.isNaN(pesos) || pesos <= 0) {
              setValue('amount', 0);
            } else {
              setValue('amount', Math.min(pesos, maxPesos) * 100);
            }
          }}
        />
      </div>
    </FormField>
  );
}
