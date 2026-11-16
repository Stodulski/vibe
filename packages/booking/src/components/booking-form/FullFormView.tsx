import type { Control, FieldErrors, UseFormHandleSubmit, UseFormRegister } from 'react-hook-form';
import { BOOKING_MESSAGES } from '../../messages';
import { submitHandler } from '@vibe/ui';
import type { PublicBookingFormData } from '../../schemas/public-booking.schema';
import { SubmitFooter } from './SubmitFooter';
import { NameFields } from './NameFields';
import { PhoneAndEmailFields } from './PhoneAndEmailFields';
import { PrivacyNotice } from './PrivacyNotice';
import { NotesField } from './NotesField';
import type { BookingPricing } from './pricing';

const t = BOOKING_MESSAGES;

interface FullFormViewProps {
  pricing: BookingPricing;
  isLoading: boolean;
  register: UseFormRegister<PublicBookingFormData>;
  handleSubmit: UseFormHandleSubmit<PublicBookingFormData>;
  control: Control<PublicBookingFormData>;
  errors: FieldErrors<PublicBookingFormData>;
  onSubmit: (data: PublicBookingFormData) => void;
}

/** The fields and the pay button. The ticket of what is paid for sits above it, in the checkout's band. */
export function FullFormView({
  pricing,
  isLoading,
  register,
  handleSubmit,
  control,
  errors,
  onSubmit,
}: FullFormViewProps) {
  return (
    <form onSubmit={submitHandler(handleSubmit, onSubmit)} className="space-y-4">
      <NameFields register={register} errors={errors} />
      <PhoneAndEmailFields register={register} control={control} errors={errors} />
      <NotesField register={register} />
      <SubmitFooter isLoading={isLoading} totalOnline={pricing.totalOnline} type="submit" />
      <PrivacyNotice text={t.publicBooking.privacyNotice} />
    </form>
  );
}
