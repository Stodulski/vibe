import { FormField } from '@/shared/components/common/FormField';
import { FieldRequirement } from '@/shared/components/common/FieldRequirement';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import { DepositAmountField } from './DepositAmountField';
import { FullPaymentSummary } from './FullPaymentSummary';
import type { UseFormSetValue, FieldErrors } from 'react-hook-form';
import type { CreateBookingDto } from '../../schemas/booking.schema';

const t = ES_AR;

interface PaymentOptionFieldsProps {
  estimatedPrice: number;
  paymentOption: string | undefined;
  depositAmount: number | undefined;
  defaultDepositPesos: number | null;
  maxDepositPesos: number | null;
  setValue: UseFormSetValue<CreateBookingDto>;
  errors: FieldErrors<CreateBookingDto>;
}

export function PaymentOptionFields({
  estimatedPrice,
  paymentOption,
  depositAmount,
  defaultDepositPesos,
  maxDepositPesos,
  setValue,
  errors,
}: PaymentOptionFieldsProps) {
  return (
    <div className="space-y-3">
      <FormField
        label={
          <>
            {t.bookings.paymentOptionLabel}
            <FieldRequirement required={false} />
          </>
        }
        htmlFor="booking-payment-option"
      >
        <Select
          value={paymentOption ?? 'unpaid'}
          onValueChange={(v) => {
            setValue('payment_option', v as 'unpaid' | 'deposit' | 'full');
            if (v === 'deposit' && defaultDepositPesos) {
              setValue('deposit_amount', defaultDepositPesos);
            } else {
              setValue('deposit_amount', undefined);
            }
          }}
        >
          <SelectTrigger id="booking-payment-option">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="unpaid">{t.bookings.paymentOptions.unpaid}</SelectItem>
            <SelectItem value="deposit">{t.bookings.paymentOptions.deposit}</SelectItem>
            <SelectItem value="full">{t.bookings.paymentOptions.full}</SelectItem>
          </SelectContent>
        </Select>
      </FormField>
      {paymentOption === 'deposit' && (
        <DepositAmountField
          estimatedPrice={estimatedPrice}
          depositAmount={depositAmount}
          maxDepositPesos={maxDepositPesos}
          setValue={setValue}
          errors={errors}
        />
      )}
      {paymentOption === 'full' && <FullPaymentSummary estimatedPrice={estimatedPrice} />}
    </div>
  );
}
