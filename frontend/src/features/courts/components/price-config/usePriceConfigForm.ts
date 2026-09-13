import { useAppForm } from '@/shared/lib/form';
import { zodResolver } from '@hookform/resolvers/zod';
import { toast } from 'sonner';
import { useUpdatePrices } from '../../hooks/useUpdatePrices';
import { useComplex, useSchedules } from '@/features/complex';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getFieldErrors } from '@/shared/lib/serverErrors';
import { priceFormSchema } from '../../schemas/courts.schema';
import { ALL_DAYS, EMPTY_PRICE_FORM_VALUES, type PriceFormValues } from './days';
import type { CourtWithPrices, DayType, Schedule } from '@/shared/types/api.types';

const t = ES_AR;

// A failed-validation body keys its errors "prices[3].time_to" — the array
// index the server refused, not a day name. This picks that index back out
// so it can be mapped onto the row it came from.
const PRICE_INDEX_RX = /^prices\[(\d+)\]/;

/** The form's `defaultValues`: each day's stored price, in pesos. */
function priceFormValues(court: CourtWithPrices): PriceFormValues {
  const values: PriceFormValues = { ...EMPTY_PRICE_FORM_VALUES };
  for (const p of court.prices) {
    if (p.day_type in values) {
      values[p.day_type] = p.price / 100;
    }
  }
  return values;
}

export function usePriceConfigForm(complexId: string, court: CourtWithPrices, onClose: () => void) {
  const updatePrices = useUpdatePrices(complexId);
  // Both are cached queries the app has already run; reading them here beats
  // threading opening hours through the three screens that mount this form.
  const { data: complex } = useComplex(complexId);
  const { data: schedules = [] } = useSchedules(complexId, complex?.slug);

  // `defaultValues` computed once at mount, not synced back in with a
  // `reset()` effect: `PriceConfig` is keyed by court id (see `CourtGrid`),
  // so a different court gets a fresh instance of this hook instead of the
  // same one reset out from under an in-progress edit.
  const form = useAppForm<PriceFormValues>({
    resolver: zodResolver(priceFormSchema),
    defaultValues: priceFormValues(court),
  });
  const { setError } = form;

  const onSubmit = (data: PriceFormValues) => {
    // Kept alongside `prices` below so a server error naming "prices[i]" can
    // be traced back to the day it came from: includedDays[i] is the day that
    // became prices[i] on the wire, in the same order.
    const includedDays = ALL_DAYS.filter(({ value }) => data[value] > 0);
    const prices = includedDays.map(({ value }) => ({
      price: Math.round(data[value] * 100),
      day_type: value,
      ...bandFor(schedules, value),
    }));

    if (prices.length === 0) {
      toast.error(t.courts.atLeastOnePrice);
      return;
    }

    updatePrices.mutate(
      { courtId: court.id, data: { prices } },
      {
        onSuccess: () => {
          onClose();
        },
        // Server-side field errors land ON their row, not only in a toast —
        // this is the same "overlaps another band" or "invalid time" a stub
        // day's schedule can still produce even though the owner never typed
        // a time themselves (see usePriceConfigForm's bandFor). Anything the
        // index can't be traced back to a day (a malformed body, or a
        // whole-array error like "must contain at least one price") falls
        // back to a toast instead of vanishing silently — see
        // useUpdatePrices, which stays quiet on a mappable field error so the
        // two don't both fire for the same failure.
        onError: (error: unknown) => {
          const fieldErrors = getFieldErrors(error);
          const unmapped: string[] = [];
          for (const [key, message] of Object.entries(fieldErrors)) {
            const match = PRICE_INDEX_RX.exec(key);
            const day = match ? includedDays[Number(match[1])]?.value : undefined;
            if (day) {
              setError(day, { type: 'server', message });
            } else {
              unmapped.push(message);
            }
          }
          if (unmapped.length > 0) {
            toast.error(unmapped.join('. '));
          }
        },
      },
    );
  };

  return { form, onSubmit, isPending: updatePrices.isPending };
}

/**
 * The hours one day's price covers: that day's own opening window.
 *
 * This form asks for one rate per day, so the band it writes has to be the day.
 * It used to write 00:00–23:59 for every day, which stops at the minute before
 * midnight — so a venue trading Thursday 08:00 to 01:30 had its last four hours
 * unpriced no matter what the owner typed, and an unpriced hour is not for sale.
 * Writing the window instead makes the two agree by construction: whatever is
 * open is priced, and nothing else is.
 *
 * A closed day still gets a band, covering its calendar day. Staff book days
 * the venue is shut, and the rate they are charged comes from that day's card.
 *
 * A window that runs past midnight (open_time > close_time, e.g. "08:00" to
 * "01:30") is sent as-is rather than split or clamped: the server reads
 * time_to <= time_from as "this band ends the next day" (the span_min generated column),
 * the same rule complex_schedules has always used for opening hours, so a band
 * built straight from the schedule already lands in the shape
 * the server expects.
 */
function bandFor(schedules: Schedule[], day: DayType): { time_from: string; time_to: string } {
  const row = schedules.find((s) => s.day === day);
  if (!row || row.is_closed || row.open_time === row.close_time) {
    return { time_from: '00:00', time_to: '23:59' };
  }
  return { time_from: row.open_time, time_to: row.close_time };
}
