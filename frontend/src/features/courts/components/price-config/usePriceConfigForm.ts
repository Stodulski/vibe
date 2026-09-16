import { useCallback } from 'react';
import { useAppForm } from '@/shared/lib/form';
import { zodResolver } from '@hookform/resolvers/zod';
import { toast } from 'sonner';
import { useUpdatePrices } from '../../hooks/useUpdatePrices';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getFieldErrors } from '@/shared/lib/serverErrors';
import { priceFormSchema, type PriceBandValues } from '../../schemas/courts.schema';
import { ALL_DAYS, type PriceFormValues } from './days';
import { bandField, buildDayBands, dayPriceField, nextDifferentiatedBand, priceFormValues } from './bands';
import type { UseFormSetError } from 'react-hook-form';
import type { CourtWithPrices, DayType, Schedule, UpdatePricesRequest } from '@/shared/types/api.types';

const t = ES_AR;

// A failed-validation body keys its errors "prices[3].time_to" — the array
// index the server refused, not a day name, and not a row within one. This
// picks that index and the field name back out so both can be mapped onto the
// row they came from. The field group is optional because a whole-item error
// ("prices[3]") carries no field at all, and lands on the row's price.
const PRICE_INDEX_RX = /^prices\[(\d+)\](?:\.(\w+))?$/;

// Which of a differentiated row's fields a server error may be steered onto.
// Anything else the server might name (a column this form does not render)
// falls through to the toast rather than being forced onto an unrelated input.
const MAPPABLE_FIELDS = new Set<keyof PriceBandValues>(['time_from', 'time_to', 'price']);

function isMappableField(field: string | undefined): field is keyof PriceBandValues {
  return field !== undefined && MAPPABLE_FIELDS.has(field as keyof PriceBandValues);
}

/**
 * Which form control a wire `prices[i]` entry came from: either a
 * differentiated row (its own `desde`/`hasta`/`precio`), or a gap `buildDayBands`
 * filled in at the full-day rate — which has no row of its own, only the
 * day's price field, so `time_from`/`time_to` have nowhere to land for one of
 * these (see `placeFieldErrors`).
 */
type SentBand = { day: DayType } & ({ kind: 'day' } | { kind: 'band'; index: number });

/**
 * `schedules` is a parameter rather than a query read in here because the
 * opening hours are now `defaultValues` — the times the owner SEES and edits,
 * not something derived at submit time from whatever had loaded by then. A
 * form seeded from an unresolved query shows fabricated hours and saves them,
 * which is the bug the schedule tab already fixed by refusing to mount its
 * form until the real data exists (`ScheduleConfig`). `PriceConfig` gates this
 * one the same way, so by the time this runs the hours are the venue's.
 */
export function usePriceConfigForm(
  complexId: string,
  court: CourtWithPrices,
  schedules: Schedule[],
  onClose: () => void,
) {
  const updatePrices = useUpdatePrices(complexId);

  // `defaultValues` computed once at mount, not synced back in with a
  // `reset()` effect: `PriceConfig` is keyed by court id (see `CourtGrid`),
  // so a different court gets a fresh instance of this hook instead of the
  // same one reset out from under an in-progress edit.
  const form = useAppForm<PriceFormValues>({
    resolver: zodResolver(priceFormSchema),
    defaultValues: priceFormValues(court),
  });
  const { setError } = form;

  // Handed down to every `DayRow` so "Nuevo precio" does not have to know
  // about schedules. Stable so a row's re-render does not churn its button.
  const nextBand = useCallback(
    (day: DayType, bands: PriceBandValues[]) => nextDifferentiatedBand(bands, schedules, day),
    [schedules],
  );

  const onSubmit = (data: PriceFormValues) => {
    const { prices, sent } = buildPrices(data, schedules);

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
        // this is the same "overlaps another band" or "invalid time" the
        // client-side schema already checks, kept because the server is the one
        // that decides and because a stub day's schedule can still produce one
        // even though the owner never typed a time themselves (see
        // `openingBandFor`). Anything the index can't be traced back to a row (a
        // malformed body, or a whole-array error like "must contain at least
        // one price") falls back to a toast instead of vanishing silently — see
        // useUpdatePrices, which stays quiet on a mappable field error so the
        // two don't both fire for the same failure.
        onError: (error: unknown) => {
          const unmapped = placeFieldErrors(getFieldErrors(error), sent, setError);
          if (unmapped.length > 0) {
            toast.error(unmapped.join('. '));
          }
        },
      },
    );
  };

  return { form, onSubmit, nextBand, isPending: updatePrices.isPending };
}

/**
 * The wire payload, and the map back from it to the form controls it was
 * built from.
 *
 * `sent[i]` is where `prices[i]` came from, in the same order, so a server
 * error naming "prices[3]" can be traced to the control the owner is looking
 * at. The wire index counts across days, which is why the map has to carry
 * the day as well as the position within it.
 */
function buildPrices(
  data: PriceFormValues,
  schedules: Schedule[],
): { prices: UpdatePricesRequest['prices']; sent: SentBand[] } {
  const sent: SentBand[] = [];
  const prices: UpdatePricesRequest['prices'] = [];
  for (const { value: day } of ALL_DAYS) {
    const dayValues = data[day];
    // A day with no full-day price and no differentiated rows has no rate at
    // all — the state this dialog has always used for "I do not price this
    // day" — so nothing is sent for it. Once there is a row, the schema has
    // already insisted on a full-day price to fill the rest, so nothing is
    // silently lost from a day the owner did describe.
    if (!(dayValues.price > 0) && dayValues.bands.length === 0) continue;

    for (const built of buildDayBands(dayValues, schedules, day)) {
      sent.push(built.source.kind === 'day' ? { day, kind: 'day' } : { day, kind: 'band', index: built.source.index });
      prices.push({
        price: Math.round(built.price * 100),
        day_type: day,
        time_from: built.time_from,
        time_to: built.time_to,
      });
    }
  }
  return { prices, sent };
}

/**
 * Puts each server field error on the control it came from, and returns the
 * messages that had nowhere to go.
 */
function placeFieldErrors(
  fieldErrors: Record<string, string>,
  sent: SentBand[],
  setError: UseFormSetError<PriceFormValues>,
): string[] {
  const unmapped: string[] = [];
  for (const [key, message] of Object.entries(fieldErrors)) {
    const match = PRICE_INDEX_RX.exec(key);
    if (!match) {
      unmapped.push(message);
      continue;
    }
    const source = sent[Number(match[1])];
    if (!source) {
      unmapped.push(message);
      continue;
    }
    if (source.kind === 'day') {
      // A gap `buildDayBands` filled at the full-day rate has no row of its
      // own — only `price` has anywhere to land; a server complaint about
      // ITS hours would be about hours the owner never typed.
      if (isMappableField(match[2]) && match[2] !== 'price') {
        unmapped.push(message);
      } else {
        setError(dayPriceField(source.day), { type: 'server', message });
      }
      continue;
    }
    const field = isMappableField(match[2]) ? match[2] : 'price';
    setError(bandField(source.day, source.index, field), { type: 'server', message });
  }
  return unmapped;
}
