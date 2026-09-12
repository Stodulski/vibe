import { PaymentOptionFields } from './PaymentOptionFields';
import type { UseFormSetValue, FieldErrors } from 'react-hook-form';
import type { CreateBookingDto } from '../../schemas/booking.schema';

/**
 * Renders nothing until a price is known — either the server-computed
 * estimate or a manually-entered one (see `effectivePrice` in
 * `useBookingPricing`) — since the payment option/deposit controls need a
 * price to validate against.
 */
export function PaymentOptionsSection({
  effectivePrice,
  paymentOption,
  depositAmount,
  defaultDepositPesos,
  maxDepositPesos,
  setValue,
  errors,
}: {
  effectivePrice: number | null;
  paymentOption: string | undefined;
  depositAmount: number | undefined;
  defaultDepositPesos: number | null;
  maxDepositPesos: number | null;
  setValue: UseFormSetValue<CreateBookingDto>;
  errors: FieldErrors<CreateBookingDto>;
}) {
  if (effectivePrice === null) return null;

  return (
    <PaymentOptionFields
      estimatedPrice={effectivePrice}
      paymentOption={paymentOption}
      depositAmount={depositAmount}
      defaultDepositPesos={defaultDepositPesos}
      maxDepositPesos={maxDepositPesos}
      setValue={setValue}
      errors={errors}
    />
  );
}
