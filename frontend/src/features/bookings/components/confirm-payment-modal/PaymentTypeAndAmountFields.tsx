import { FormField } from '@/shared/components/common/FormField';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import { DepositAmountInput } from './DepositAmountInput';
import type { UseFormSetValue, FieldErrors } from 'react-hook-form';
import type { ConfirmPaymentDto } from '../../schemas/booking.schema';
import type { Booking } from '@/shared/types/api.types';

const t = ES_AR;

export function PaymentTypeAndAmountFields({
  isDepositPaid,
  paymentType,
  onPaymentTypeChange,
  booking,
  setValue,
  errors,
}: {
  isDepositPaid: boolean;
  paymentType: 'full' | 'deposit';
  onPaymentTypeChange: (type: 'full' | 'deposit') => void;
  booking: Booking | null;
  setValue: UseFormSetValue<ConfirmPaymentDto>;
  errors: FieldErrors<ConfirmPaymentDto>;
}) {
  return (
    <>
      {!isDepositPaid && (
        <FormField className="space-y-2" label={t.bookings.chargeType} htmlFor="confirm-payment-type">
          <Select
            value={paymentType}
            onValueChange={(v) => {
              onPaymentTypeChange(v as 'full' | 'deposit');
            }}
          >
            <SelectTrigger id="confirm-payment-type">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="full">{t.bookings.fullPayment}</SelectItem>
              <SelectItem value="deposit">{t.bookings.depositPayment}</SelectItem>
            </SelectContent>
          </Select>
        </FormField>
      )}

      {paymentType === 'deposit' && !isDepositPaid && (
        <DepositAmountInput booking={booking} setValue={setValue} errors={errors} />
      )}
    </>
  );
}
