import { useState } from 'react';
import { useAppForm } from '@/shared/lib/form';
import { zodResolver } from '@hookform/resolvers/zod';
import { publicBookingSchema, type PublicBookingFormData } from '../schemas/public-booking.schema';
import { DEFAULT_PHONE_PREFIX } from '@/shared/lib/constants';
import { getSavedFormData, hasCompleteSavedData } from './booking-form/savedFormData';
import { useSaveFormData } from './booking-form/useSaveFormData';
import { computeBookingPricing } from './booking-form/pricing';
import { QuickBookView } from './booking-form/QuickBookView';
import { FullFormView } from './booking-form/FullFormView';
import type { BookingSlotInfo } from './booking-form/types';

export type { BookingSlotInfo } from './booking-form/types';

interface BookingFormProps {
  slotInfo: BookingSlotInfo;
  onSubmit: (data: PublicBookingFormData) => void;
  isLoading: boolean;
}

export function BookingForm({ slotInfo, onSubmit, isLoading }: BookingFormProps) {
  const saved = getSavedFormData();
  const [quickBookMode, setQuickBookMode] = useState(() => hasCompleteSavedData(saved));

  const pricing = computeBookingPricing(slotInfo);

  const {
    register,
    handleSubmit,
    watch,
    control,
    formState: { errors },
  } = useAppForm<PublicBookingFormData>({
    resolver: zodResolver(publicBookingSchema),
    defaultValues: {
      client_first_name: saved.client_first_name ?? '',
      client_last_name: saved.client_last_name ?? '',
      // Argentina is the only prefix ever emitted now — a stale non-AR
      // `phone_prefix` from before the country selector was removed must not
      // be trusted (see savedFormData.ts).
      client_phone: saved.client_phone ? DEFAULT_PHONE_PREFIX + saved.client_phone : '',
      client_email: saved.client_email ?? '',
      client_notes: saved.client_notes ?? '',
    },
  });

  useSaveFormData(watch);

  if (quickBookMode) {
    return (
      <QuickBookView
        slotInfo={slotInfo}
        pricing={pricing}
        saved={saved}
        isLoading={isLoading}
        onSubmit={onSubmit}
        onEdit={() => {
          setQuickBookMode(false);
        }}
      />
    );
  }

  return (
    <FullFormView
      slotInfo={slotInfo}
      pricing={pricing}
      isLoading={isLoading}
      register={register}
      handleSubmit={handleSubmit}
      control={control}
      errors={errors}
      onSubmit={onSubmit}
    />
  );
}
