import { Banknote } from 'lucide-react';
import { Input } from '@/shared/components/ui/input';
import { FormField } from '@/shared/components/common/FormField';
import { FieldRequirement } from '@/shared/components/common/FieldRequirement';
import { useMoneyInput } from '@/shared/hooks/useMoneyInput';
import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { UseFormSetValue, FieldErrors } from 'react-hook-form';
import type { CreateBookingDto } from '../../schemas/booking.schema';

const t = ES_AR;

export function DepositAmountField({
  estimatedPrice,
  depositAmount,
  maxDepositPesos,
  setValue,
  errors,
}: {
  estimatedPrice: number;
  depositAmount: number | undefined;
  maxDepositPesos: number | null;
  setValue: UseFormSetValue<CreateBookingDto>;
  errors: FieldErrors<CreateBookingDto>;
}) {
  const { displayValue, inputRef, handleChange, handleBlur } = useMoneyInput({
    value: depositAmount,
    onChange: (raw) => {
      // `shouldDirty` marks this as a manual edit — `useCreateBookingForm`'s
      // submit handler only re-applies the price-derived default deposit
      // to a field the person never touched (see 02-bookings-clients.md M3).
      if (raw === undefined) {
        setValue('deposit_amount', undefined, { shouldDirty: true });
        return;
      }
      const max = maxDepositPesos ?? Infinity;
      setValue('deposit_amount', Math.min(raw, max), { shouldDirty: true });
    },
  });

  return (
    <FormField
      label={
        <>
          {t.bookings.depositAmountLabel}
          <FieldRequirement required />
        </>
      }
      htmlFor="deposit-amount"
      labelSuffix={
        <span className="text-micro text-text-tertiary">
          {t.bookings.depositMax} {formatPrice(estimatedPrice)}
        </span>
      }
      error={errors.deposit_amount?.message}
    >
      <div className="relative">
        <Banknote className="text-text-tertiary absolute top-1/2 left-3 size-4 -translate-y-1/2" />
        <Input
          id="deposit-amount"
          type="text"
          inputMode="numeric"
          autoComplete="off"
          className="pl-9"
          ref={inputRef}
          value={displayValue}
          onChange={handleChange}
          onBlur={handleBlur}
          placeholder={t.placeholders.amount}
        />
      </div>
    </FormField>
  );
}
