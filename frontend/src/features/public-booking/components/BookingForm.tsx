import { useState } from 'react';
import { useAppForm } from '@/shared/lib/form';
import { useUnsavedWork } from '@/shared/hooks/useUnsavedWork';
import { zodResolver } from '@hookform/resolvers/zod';
import { publicBookingSchema, type PublicBookingFormData } from '../schemas/public-booking.schema';
import { DEFAULT_PHONE_PREFIX } from '@/shared/lib/constants';
import { getSavedFormData, hasCompleteSavedData, clearSavedFormData } from './booking-form/savedFormData';
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

/**
 * What `reset()` hands the (shared) form back to for "No, soy otra persona".
 * `useAppForm`'s own `defaultValues` were captured once, from `saved`, at
 * this form's first mount — switching to `FullFormView` alone would still
 * show the person who just said this isn't them.
 */
const EMPTY_QUICK_BOOK_VALUES: PublicBookingFormData = {
  client_first_name: '',
  client_last_name: '',
  client_phone: '',
  client_email: '',
  client_notes: '',
};

export function BookingForm({ slotInfo, onSubmit, isLoading }: BookingFormProps) {
  const saved = getSavedFormData();
  const [quickBookMode, setQuickBookMode] = useState(() => hasCompleteSavedData(saved));

  const pricing = computeBookingPricing(slotInfo);

  const {
    register,
    handleSubmit,
    watch,
    control,
    reset,
    formState: { errors, isDirty },
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
  // Half-filled contact details are work a reload would throw away, and this
  // page is where the PWA is most likely to want one: a client who starts
  // typing, switches to WhatsApp to check a phone number and comes back must
  // find the form as they left it, not a fresh one. `isDirty` compares against
  // the quick-book defaults, so pre-filled values alone do not count.
  useUnsavedWork(isDirty);

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
        onNotMe={() => {
          clearSavedFormData();
          reset(EMPTY_QUICK_BOOK_VALUES);
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
