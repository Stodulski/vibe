import { PricePreview } from './PricePreview';
import { ManualPriceField } from './ManualPriceField';
import type { UseFormSetValue, FieldErrors } from 'react-hook-form';
import type { CreateBookingDto } from '../../schemas/booking.schemas';

/**
 * The estimate when there is one, the manual-price input when the chosen hour
 * has no rate, and nothing at all until a court and time have been picked —
 * `estimatedPrice` is null in that last case too, and asking for a price
 * before anything is chosen puts an error on a field nobody reached yet.
 */
export function PriceOrManualPriceField({
  estimatedPrice,
  priceRequired,
  price,
  setValue,
  errors,
}: {
  estimatedPrice: number | null;
  priceRequired: boolean;
  price: number | undefined;
  setValue: UseFormSetValue<CreateBookingDto>;
  errors: FieldErrors<CreateBookingDto>;
}) {
  if (estimatedPrice !== null) return <PricePreview estimatedPrice={estimatedPrice} />;
  if (!priceRequired) return null;
  return <ManualPriceField price={price} setValue={setValue} errors={errors} />;
}
