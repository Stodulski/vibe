import { Banknote } from 'lucide-react';
import { useState } from 'react';
import type { UseFormSetValue, FieldErrors } from 'react-hook-form';
import { Input } from '@/shared/components/ui/input';
import { FormField } from '@/shared/components/common/FormField';
import { useMoneyInput } from '@/shared/hooks/useMoneyInput';
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
 * The clamp is not decoration. `max` on a native number input was a hint a
 * browser may enforce and a keyboard may not; now that this is a
 * `type="text"` money input (see `useMoneyInput`) there is no `max` for the
 * browser to enforce at all, so the same ceiling is applied in JS here
 * regardless — a deposit larger than the booking is refused by the server
 * anyway, and being told so after submitting is worse than not being able to
 * type it.
 *
 * Its own `pesos` state, not driven by the form's `amount` field: `amount`
 * is stored in cents (what the API takes), while the field displays and
 * formats pesos — the same split `RestockDialog`'s cost field and
 * `usePriceConfigForm`'s pesos-vs-cents fields keep, just local here because
 * nothing outside this component ever needs to read the pesos figure back.
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
  const [pesos, setPesos] = useState<number | undefined>(undefined);
  const { displayValue, inputRef, handleChange } = useMoneyInput({
    value: pesos,
    onChange: (v) => {
      setPesos(v);
      if (v === undefined || v <= 0) {
        setValue('amount', 0);
      } else {
        setValue('amount', Math.min(v, maxPesos) * 100);
      }
    },
  });

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
          type="text"
          inputMode="numeric"
          autoComplete="off"
          className="pl-9"
          ref={inputRef}
          value={displayValue}
          onChange={handleChange}
          placeholder={t.placeholders.depositAmount}
        />
      </div>
    </FormField>
  );
}
