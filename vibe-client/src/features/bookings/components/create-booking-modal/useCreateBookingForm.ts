import { useCreateBooking } from '../../hooks/useCreateBooking';
import { useBookingFormState } from './useBookingFormState';
import { useBookingPricing } from './useBookingPricing';
import { useBookingTimeSlots } from './useBookingTimeSlots';
import { useBookingReset } from './useBookingReset';
import type { CreateBookingPrefill } from './useBookingReset';
import { cleanBookingPayload } from './cleanBookingPayload';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CreateBookingDto } from '../../schemas/booking.schemas';
import type { CourtWithPrices, Schedule } from '@/shared/types/api.types';

const t = ES_AR;

interface UseCreateBookingFormArgs {
  open: boolean;
  complexId: string;
  courts: CourtWithPrices[];
  depositPercentage: number;
  /** Opening hours, so the price preview charges from the window the hour came out of. */
  schedules: Schedule[];
  prefill?: CreateBookingPrefill | undefined;
}

export function useCreateBookingForm({
  open,
  complexId,
  courts,
  depositPercentage,
  prefill,
  schedules,
}: UseCreateBookingFormArgs) {
  const createBooking = useCreateBooking(complexId);
  const formState = useBookingFormState();
  const { courtId, date, startTime, durationMinutes, price, setError, reset, dirtyFields } = formState;

  const pricing = useBookingPricing({
    schedules,
    courts,
    courtId,
    date,
    startTime,
    durationMinutes,
    depositPercentage,
    price,
  });

  const { timeSlots } = useBookingTimeSlots({ open, complexId, courtId, date, durationMinutes });

  useBookingReset({ open, prefill, reset });

  const onSubmit = (data: CreateBookingDto, onClose: () => void) => {
    // `price` has no cross-field rule in the schema (it depends on
    // `courts`/`schedules`, neither of which is a form field) — checked here
    // instead. `CreateBookingSteps.goNext` runs the same check before letting
    // step 1 advance, so this is the last-resort backstop, not the only gate.
    if (pricing.priceRequired && !data.price) {
      setError('price', { message: t.validation.manualPriceRequired });
      return;
    }

    // The deposit field is only ever filled by hand: once by selecting
    // "deposit" as the payment option, and after that (if the price changed
    // while it stayed selected) directly by the person. Apply the fresh
    // default in place of a stale one here — no effect chased this in the
    // background (see 02-bookings-clients.md M3).
    const depositAmount = dirtyFields.deposit_amount
      ? data.deposit_amount
      : (pricing.defaultDepositPesos ?? data.deposit_amount);

    createBooking.mutate(cleanBookingPayload({ ...data, deposit_amount: depositAmount }), {
      onSuccess: () => {
        onClose();
      },
    });
  };

  return { ...formState, ...pricing, timeSlots, onSubmit, createBooking };
}
