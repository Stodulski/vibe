import { useCallback } from 'react';
import { Trash2 } from 'lucide-react';
import { useWatch, type Control, type UseFormRegister, type UseFormSetValue } from 'react-hook-form';
import { TimeSelect } from '@/shared/components/common/TimeSelect';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';
import { PriceField } from './PriceRow';
import { bandField, endsNextDay } from './bands';
import type { PriceFormValues } from './days';
import type { DayType } from '@/shared/types/api.types';

const t = ES_AR;

interface BandRowProps {
  day: DayType;
  /** The day's full name, for the accessible names on this row's controls. */
  dayLabel: string;
  index: number;
  control: Control<PriceFormValues>;
  register: UseFormRegister<PriceFormValues>;
  setValue: UseFormSetValue<PriceFormValues>;
  onRemove: () => void;
  errors: {
    time_from?: string | undefined;
    time_to?: string | undefined;
    price?: string | undefined;
  };
}

/**
 * One time band of one day: `desde → hasta`, its hourly rate, and a way to
 * delete it.
 *
 * It only appears once a day has a differentiated row — see `DayRow`, which
 * keeps a plain day at a single full-day price field. The owner who does not
 * use differentiated prices should not have to read a row that exists to
 * describe them.
 *
 * Every control names its day AND its position ("Jueves, franja 2: desde"),
 * because a dialog that can hold twenty of these otherwise offers a screen
 * reader twenty controls called "desde". The number is the band's position in
 * the day as displayed, counted from one.
 *
 * One line, always, down to a 320px dialog — the owner's own floor, measured
 * in the browser rather than assumed. `flex-nowrap` is what makes that a
 * requirement instead of a hope: without it the row already wrapped at that
 * width, dropping the price field and the delete button to a second line.
 *
 * Two different shapes below and above `sm`, on purpose, not one fill
 * behavior stretched everywhere:
 *
 *   below `sm`   every field FILLS the line — `min-w-0 flex-1` on each time
 *                select, a touch more (`grow-[1.15]`) on the price field,
 *                because its number reads longer — instead of sitting at
 *                content width with empty space left over before the delete
 *                button. This is the shape a 320px dialog needs to keep the
 *                row on one line at all; a spacer that only pushed the price
 *                field right would leave the row exactly as sparse as before.
 *   `sm:` and up  the two time selects go back to `TimeSelect`'s own plain
 *                `sm:w-[6.75rem] sm:flex-none` default (this component passes
 *                no `sm:` override for them any more), and the PRICE FIELD
 *                is what absorbs the spare width — `sm:ml-auto` on it, not
 *                stretched, just pushed to sit immediately left of the
 *                delete button. `[desde a hasta]` reads on the left, a gap,
 *                then `[precio][🗑]` together on the right.
 *
 * `PriceField`'s and `TimeSelect`'s own `compact` prop still narrows their
 * padding and text only below `sm`, in both shapes.
 */
export function BandRow({ day, dayLabel, index, control, register, setValue, onRemove, errors }: BandRowProps) {
  const timeFrom = useWatch({ control, name: bandField(day, index, 'time_from') });
  const timeTo = useWatch({ control, name: bandField(day, index, 'time_to') });
  const isNextDay = endsNextDay({ time_from: timeFrom, time_to: timeTo });

  // Stable identities so a row's re-render does not churn its two selects.
  // `setValue` is stable across renders, and `day`/`index` are the row.
  const setFrom = useCallback(
    (v: string) => {
      setValue(bandField(day, index, 'time_from'), v, { shouldValidate: true });
    },
    [setValue, day, index],
  );
  const setTo = useCallback(
    (v: string) => {
      setValue(bandField(day, index, 'time_to'), v, { shouldValidate: true });
    },
    [setValue, day, index],
  );

  const bandName = `${dayLabel}, ${t.courts.bandOrdinal} ${String(index + 1)}`;
  // The only place this fact reaches anyone now: there is no visible marker
  // for a band that runs into the next day any more (owner instruction — it
  // used to show beside the `Hasta` select, and as a corner badge below
  // `sm`), so a screen reader has nothing to read it from except this select's
  // own accessible name. Validation is unaffected either way: an end that
  // reads at or before its start was always accepted as "next day" by
  // `bandSpan`/`endsNextDay`, whether or not anything on screen said so.
  const toName = isNextDay
    ? `${bandName}: ${t.courts.bandTo}, ${t.courts.bandEndsNextDay}`
    : `${bandName}: ${t.courts.bandTo}`;

  return (
    // `items-start`, not `items-center`: every control here is the same
    // 40px-tall by default, so the two alignments look identical until one
    // of them grows — a `PriceField` in error is taller than its input
    // alone (see `PriceField`'s own comment), and centering THAT box would
    // center the input+message pair as a unit, floating the input itself
    // above the other three controls' shared centre instead of keeping all
    // four inputs aligned. Top-aligning them is what keeps the inputs
    // themselves lined up regardless of which one has grown a message below
    // it. The plain-text connector span opts back into `self-center` for
    // exactly that reason — it has no input to line up with, only a middle
    // to sit in.
    <div className="flex flex-nowrap items-start gap-1 py-1.5 sm:gap-2">
      <TimeSelect
        value={timeFrom}
        aria-label={`${bandName}: ${t.courts.bandFrom}`}
        onChange={setFrom}
        compact
        // No `sm:` override here on purpose: below `sm` this fills its share
        // of the line (`min-w-0 max-w-28 flex-1`); at `sm:` and up, with
        // nothing here to dedupe against it, `TimeSelect`'s own
        // `sm:w-[6.75rem] sm:flex-none` default takes back over unchanged —
        // the plain fixed desktop width it always had.
        className="max-w-28 min-w-0 flex-1"
      />
      {/* Dropped below `sm`: "a" is one character already, and the two
          selects sitting flush against each other still read as a pair. */}
      <span className="text-micro text-text-tertiary hidden shrink-0 self-center sm:inline">
        {t.complex.timeRangeJoiner}
      </span>
      <TimeSelect value={timeTo} aria-label={toName} onChange={setTo} compact className="max-w-28 min-w-0 flex-1" />

      <PriceField
        placeholder=""
        // The two time errors share this field's error slot; only the
        // earliest (read left to right) shows, so that is the one to fix first.
        error={errors.time_from ?? errors.time_to ?? errors.price}
        aria-label={`${bandName}: ${t.courts.price}`}
        compact
        {...register(bandField(day, index, 'price'), { valueAsNumber: true })}
      />
      {/* `ml-auto` only below `sm` — the safety net if the two time selects
          and the price field ever hit their caps before using the whole
          320px line, which would otherwise strand this button short of the
          true right edge. `sm:ml-0` turns that off at `sm:` and up: with TWO
          auto margins on the same row, `PriceField`'s own `sm:ml-auto`
          shares the spare space with this one instead of using all of it,
          which is what opened a gap between the price field and this button
          instead of sitting them together. */}
      <RemoveBandButton bandName={bandName} onRemove={onRemove} className="ml-auto sm:ml-0" />
    </div>
  );
}

/**
 * `h-10` — the same height `TimeSelect` and the (now `h-10`) `PriceField`
 * render at on this row, so all four controls share one and their vertical
 * centres line up instead of the button looking short or tall beside them.
 * Width stays independent of that, `w-6 sm:w-9` (24px/36px): the owner's own
 * floor for this button is 24px wide below `sm`, not 44px like
 * `ApplyToAllButton`, and widening it to match a 40px height would eat back
 * the room the fill pass just gave the three fields beside it. A 24×40 or
 * 36×40 box is still well over the 24×24 touch-target floor in both
 * dimensions; on an actual coarse pointer, globals.css's own floor
 * (`@media (pointer: coarse) button { min-height: 44px }`) still raises the
 * real height further, same as before.
 */
function RemoveBandButton({
  bandName,
  onRemove,
  className,
}: {
  bandName: string;
  onRemove: () => void;
  className?: string;
}) {
  return (
    <button
      type="button"
      title={t.courts.removeBand}
      aria-label={`${t.courts.removeBand}: ${bandName}`}
      onClick={onRemove}
      className={cn(
        'text-text-secondary hover:bg-bg-highlight hover:text-error-text flex h-10 w-6 shrink-0 items-center justify-center rounded-lg transition-colors sm:w-9',
        className,
      )}
    >
      <Trash2 className="size-3.5" />
    </button>
  );
}
