import { useCreateBooking } from '../../hooks/useCreateBooking';
import { useBookingFormState } from './useBookingFormState';
import { useBookingPricing } from './useBookingPricing';
import { useBookingTimeSlots } from './useBookingTimeSlots';
import { useBookingReset } from './useBookingReset';
import type { CreateBookingPrefill } from './useBookingReset';
import { cleanBookingPayload } from './cleanBookingPayload';
import { useSeriesOptions } from './useSeriesOptions';
import { useSeriesSubmit } from './useSeriesSubmit';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { UseFormSetError, UseFormSetValue } from 'react-hook-form';
import type { CreateBookingDto } from '../../schemas/booking.schema';
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

/**
 * A series is charged like a normal booking, at the court: drop any deposit
 * chosen before the switch, whose payment method would otherwise fail
 * validation from a field this mode no longer shows.
 */
function switchRepeat(
  series: ReturnType<typeof useSeriesOptions>,
  setValue: UseFormSetValue<CreateBookingDto>,
  repeat: boolean,
) {
  series.setRepeat(repeat);
  if (repeat) setValue('payment_option', 'unpaid');
}

/**
 * The form values to send as far as the manual price goes, or `null` after
 * flagging a missing one.
 *
 * `price` has no cross-field rule in the schema (it depends on
 * `courts`/`schedules`, neither of which is a form field), so it is checked
 * here; `CreateBookingSteps.goNext` runs the same check before letting step 1
 * advance, so this is the last-resort backstop, not the only gate.
 *
 * A manual price is only for a span no rate covers. One typed for an earlier,
 * unpriced span stays in the form once the owner moves to a priced one (the
 * field is hidden, not cleared), and the server takes a sent price over its
 * own; so it goes only when the span has no estimate.
 */
function withManualPriceRule(
  formData: CreateBookingDto,
  priceRequired: boolean,
  setError: UseFormSetError<CreateBookingDto>,
): CreateBookingDto | null {
  if (priceRequired && !formData.price) {
    setError('price', { message: t.validation.manualPriceRequired });
    return null;
  }
  return priceRequired ? formData : { ...formData, price: undefined };
}

/**
 * The deposit to send, in pesos. The deposit field is only ever filled by hand:
 * once by selecting "deposit" as the payment option, and after that (if the
 * price changed while it stayed selected) directly by the person. The fresh
 * default replaces a stale one here — no effect chased this in the background
 * (see 02-bookings-clients.md M3). A deposit is cash the owner says was taken,
 * so the caller refuses one that comes out empty: the server does too.
 */
function depositToSend(data: CreateBookingDto, typedByHand: boolean, defaultPesos: number | null) {
  return typedByHand ? data.deposit_amount : (defaultPesos ?? data.deposit_amount);
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
  const series = useSeriesOptions(open);
  const formState = useBookingFormState();
  const seriesSubmit = useSeriesSubmit(complexId, series);
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

  const onSubmit = (formData: CreateBookingDto, onClose: () => void) => {
    const data = withManualPriceRule(formData, pricing.priceRequired, setError);
    if (!data) return;

    // A weekly series carries no payment: each occurrence is charged like a
    // normal booking, so the deposit logic below does not apply.
    if (series.repeat) {
      seriesSubmit.submit(data, onClose);
      return;
    }

    const depositAmount = depositToSend(data, !!dirtyFields.deposit_amount, pricing.defaultDepositPesos);
    if (data.payment_option === 'deposit' && !depositAmount) {
      setError('deposit_amount', { message: t.validation.amountRequired });
      return;
    }

    createBooking.mutate(cleanBookingPayload({ ...data, deposit_amount: depositAmount }), {
      onSuccess: () => {
        onClose();
      },
    });
  };

  return {
    ...formState,
    ...pricing,
    timeSlots,
    courtName: courts.find((c) => c.id === courtId)?.name ?? '',
    onSubmit,
    createBooking,
    series: {
      ...series,
      setRepeat: (repeat: boolean) => {
        switchRepeat(series, formState.setValue, repeat);
      },
    },
    seriesSubmit,
    isSubmitting: createBooking.isPending || seriesSubmit.createSeries.isPending,
  };
}
