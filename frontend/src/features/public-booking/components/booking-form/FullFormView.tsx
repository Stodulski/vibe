import type { Control, FieldErrors, UseFormHandleSubmit, UseFormRegister } from 'react-hook-form';
import { submitHandler } from '@/shared/lib/form';
import type { PublicBookingFormData } from '../../schemas/public-booking.schema';
import { BookingSummaryCard } from './BookingSummaryCard';
import { SubmitFooter } from './SubmitFooter';
import { NameFields } from './NameFields';
import { PhoneAndEmailFields } from './PhoneAndEmailFields';
import { NotesField } from './NotesField';
import type { BookingPricing } from './pricing';
import type { BookingSlotInfo } from './types';

interface FullFormViewProps {
  slotInfo: BookingSlotInfo;
  pricing: BookingPricing;
  isLoading: boolean;
  register: UseFormRegister<PublicBookingFormData>;
  handleSubmit: UseFormHandleSubmit<PublicBookingFormData>;
  control: Control<PublicBookingFormData>;
  errors: FieldErrors<PublicBookingFormData>;
  onSubmit: (data: PublicBookingFormData) => void;
}

export function FullFormView({
  slotInfo,
  pricing,
  isLoading,
  register,
  handleSubmit,
  control,
  errors,
  onSubmit,
}: FullFormViewProps) {
  return (
    <div className="space-y-5">
      <BookingSummaryCard slotInfo={slotInfo} pricing={pricing} />

      <form onSubmit={submitHandler(handleSubmit, onSubmit)} className="space-y-4">
        <NameFields register={register} errors={errors} />
        <PhoneAndEmailFields register={register} control={control} errors={errors} />
        <NotesField register={register} />
        <SubmitFooter isLoading={isLoading} totalOnline={pricing.totalOnline} type="submit" />
      </form>
    </div>
  );
}
