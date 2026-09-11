import { Banknote } from 'lucide-react';
import { Input } from '@/shared/components/ui/input';
import { FormField } from '@/shared/components/common/FormField';
import { FieldRequirement } from '@/shared/components/common/FieldRequirement';
import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { UseFormSetValue, FieldErrors } from 'react-hook-form';
import type { CreateBookingDto } from '../../schemas/booking.schemas';

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
        <Banknote className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-text-tertiary" />
        <Input
          id="deposit-amount"
          type="number"
          className="pl-9"
          min={1}
          value={depositAmount ?? ''}
          onChange={(e) => {
            const raw = e.target.valueAsNumber;
            // `shouldDirty` marks this as a manual edit — `useCreateBookingForm`'s
            // submit handler only re-applies the price-derived default deposit
            // to a field the person never touched (see 02-bookings-clients.md M3).
            if (Number.isNaN(raw)) {
              setValue('deposit_amount', undefined, { shouldDirty: true });
              return;
            }
            const max = maxDepositPesos ?? Infinity;
            setValue('deposit_amount', Math.min(raw, max), { shouldDirty: true });
          }}
          placeholder={t.placeholders.amount}
        />
      </div>
    </FormField>
  );
}
