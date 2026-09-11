import { DollarSign } from 'lucide-react';
import { Input } from '@/shared/components/ui/input';
import { FormField } from '@/shared/components/common/FormField';
import { FieldRequirement } from '@/shared/components/common/FieldRequirement';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { UseFormSetValue, FieldErrors } from 'react-hook-form';
import type { CreateBookingDto } from '../../schemas/booking.schemas';

const t = ES_AR;

/**
 * Shown instead of `PricePreview` when `findTotalPrice` returns `null` — no
 * price rule covers this court/date/time span, so the person has to type
 * one in. Required by `CreateBookingSteps`'s manual `priceRequired` check
 * (not the schema — see 02-bookings-clients.md M3); sent as `price`
 * (converted pesos -> cents in `cleanBookingPayload`).
 */
export function ManualPriceField({
  price,
  setValue,
  errors,
}: {
  price: number | undefined;
  setValue: UseFormSetValue<CreateBookingDto>;
  errors: FieldErrors<CreateBookingDto>;
}) {
  return (
    <FormField
      label={
        <>
          {t.bookings.manualPriceLabel}
          <FieldRequirement required />
        </>
      }
      htmlFor="manual-price"
      labelSuffix={<span className="text-micro text-text-tertiary">{t.bookings.manualPriceHint}</span>}
      error={errors.price?.message}
    >
      <div className="relative">
        <DollarSign className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-text-tertiary" />
        <Input
          id="manual-price"
          type="number"
          className="pl-9"
          min={1}
          value={price ?? ''}
          onChange={(e) => {
            const raw = e.target.valueAsNumber;
            setValue('price', Number.isNaN(raw) ? undefined : raw);
          }}
          placeholder={t.placeholders.amount}
        />
      </div>
    </FormField>
  );
}
