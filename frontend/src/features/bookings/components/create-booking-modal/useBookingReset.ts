import { useEffect } from 'react';
import type { UseFormReset } from 'react-hook-form';
import type { CreateBookingDto } from '../../schemas/booking.schema';
import type { DurationMinutes } from '@/shared/types/api.types';
import { todayInArgentina } from '../../lib/today';

export interface CreateBookingPrefill {
  court_id?: string;
  date?: string;
  start_time?: string;
  /**
   * The already-chosen duration, e.g. from the booking calendar's
   * duration popover (which only offers durations that fit before the next
   * booking). When present it takes precedence over the generic default —
   * falling back to the generic default would risk seeding a duration that
   * overlaps the next booking.
   */
  duration_minutes?: DurationMinutes;
}

export function useBookingReset({
  open,
  prefill,
  reset,
}: {
  open: boolean;
  prefill: CreateBookingPrefill | undefined;
  reset: UseFormReset<CreateBookingDto>;
}) {
  useEffect(() => {
    if (open) {
      // Precedence: prefill.duration_minutes (already validated by the
      // calendar) > a generic 90. There used to be a middle step here that
      // fell back to the prefilled court's own default duration, but every
      // court sells the same three durations (60/90/120) — there is nothing
      // court-specific left to prefer, so the court is never consulted for
      // this.
      const defaultDuration: DurationMinutes = prefill?.duration_minutes ?? 90;

      reset({
        court_id: prefill?.court_id ?? '',
        date: prefill?.date ?? todayInArgentina(),
        start_time: prefill?.start_time ?? '',
        duration_minutes: defaultDuration,
        client_phone: '',
        client_email: '',
        client_first_name: '',
        client_last_name: '',
        payment_option: 'unpaid',
        deposit_amount: undefined,
        payment_method: undefined,
        notes: '',
        price: undefined,
      });
    }
  }, [open, prefill, reset]);
}
