import { useId, useState } from 'react';
import { ChevronDown, Plus } from 'lucide-react';
import {
  useFieldArray,
  useWatch,
  type Control,
  type FieldErrors,
  type UseFormRegister,
  type UseFormSetValue,
  type UseFormGetValues,
} from 'react-hook-form';
import { toast } from 'sonner';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';
import { ApplyToAllButton } from './ApplyToAllButton';
import { BandRow } from './BandRow';
import { PriceRow, PriceField } from './PriceRow';
import { dayPriceField } from './bands';
import { ALL_DAYS, type PriceFormValues } from './days';
import type { PriceBandValues } from '../../schemas/courts.schema';
import type { DayType } from '@/shared/types/api.types';

const t = ES_AR;

/** The subset of react-hook-form a day's rows need. */
interface DayRowProps {
  day: DayType;
  label: string;
  shortLabel: string;
  control: Control<PriceFormValues>;
  register: UseFormRegister<PriceFormValues>;
  setValue: UseFormSetValue<PriceFormValues>;
  getValues: UseFormGetValues<PriceFormValues>;
  /** Builds the row "Nuevo precio" appends; see `nextDifferentiatedBand`. */
  nextBand: (day: DayType, bands: PriceBandValues[]) => PriceBandValues;
  errors: FieldErrors<PriceFormValues>;
}

/**
 * One day's rate: a full-day price, always on screen, plus whatever
 * differentiated rows the owner has chosen to look at.
 *
 * The day row never changes shape the way it briefly did in an earlier pass
 * at this feature. It is always: day name, one editable price field (the
 * full-day rate — the price that applies to the whole opening window unless
 * a row below says otherwise), "aplicar a todos", and a chevron. That is the
 * exact row this dialog showed before differentiated prices existed, plus
 * the chevron. An owner who charges the same rate all day never needs to
 * touch anything else.
 *
 * The chevron reveals — never replaces — a panel underneath: one `BandRow`
 * per differentiated row, each its own `desde → hasta → precio`, deletable,
 * then a full-width "Nuevo precio" button as the panel's last item, below
 * every row rather than above them. Collapsed by default, always,
 * regardless of how many rows a day already has: unlike the single price
 * field of a plain day, the full-day price is never ambiguous or a lie while
 * collapsed — it is explicitly the base rate, and the rows are explicitly
 * exceptions, so hiding them costs nothing a range or a "which one is this"
 * guess would have to solve. A day that already has rows says so without
 * expanding: a small count sits beside the chevron, and the chevron's own
 * accessible name carries the same count for anyone who cannot see it.
 *
 * Pressing "Nuevo precio" always opens the panel it just added a row to —
 * a row added and left invisible would be a row the owner cannot see they
 * asked for. A collapsed day that carries a validation error also force-opens:
 * the message lives on the `BandRow` it belongs to, and a row that is not
 * mounted cannot show one.
 *
 * Every day looks the same as every other. Weekend rows used to be amber and
 * weekday rows green, which coloured a distinction the form does not make —
 * each day is its own field here, and Saturday is not a warning.
 */
export function DayRow(props: DayRowProps) {
  const { day, control, getValues, nextBand, errors } = props;
  const { fields, append, remove } = useFieldArray({ control, name: `${day}.bands` });
  const panelId = useId();

  const [expanded, setExpanded] = useState(false);
  const toggle = () => {
    setExpanded((v) => !v);
  };

  const addBand = () => {
    append(nextBand(day, getValues(`${day}.bands`)));
    setExpanded(true);
  };

  // Cast, not a bare read: react-hook-form's `Merge<FieldError, ...>` type for
  // a field nested two levels under a keyed record (`day.bands[i].field`)
  // collapses to `any` rather than resolving the array's element shape, so
  // the cast is what keeps every access below type-checked instead of silent.
  const bandErrors = errors[day]?.bands as FieldErrors<PriceBandValues>[] | undefined;
  const hasBandErrors = Array.isArray(bandErrors) && bandErrors.some(Boolean);
  const isExpanded = expanded || hasBandErrors;

  return (
    <div className="border-border-subtle border-b py-1.5 last:border-0">
      <BaseRow {...props} bandCount={fields.length} expanded={isExpanded} onToggle={toggle} panelId={panelId} />
      {isExpanded && (
        <div id={panelId} className="space-y-0.5 px-2 pb-1">
          {fields.map((field, index) => (
            <BandRow
              key={field.id}
              day={day}
              dayLabel={props.label}
              index={index}
              control={control}
              register={props.register}
              setValue={props.setValue}
              onRemove={() => {
                remove(index);
              }}
              errors={{
                time_from: bandErrors?.[index]?.time_from?.message,
                time_to: bandErrors?.[index]?.time_to?.message,
                price: bandErrors?.[index]?.price?.message,
              }}
            />
          ))}
          {/* Last inside the panel and full-width, not hugging its own text:
              every `desde → hasta → precio` row reads first, and the action
              that adds another one reads as one long "add a row here" target
              spanning the same edges they do, not a small button floating
              above them. */}
          <NewPriceButton dayLabel={props.label} onClick={addBand} />
        </div>
      )}
    </div>
  );
}

/**
 * The row that is always on screen: day name, the full-day price field,
 * "aplicar a todos" when it is priced, a count of any differentiated rows,
 * and the chevron that reveals them.
 */
function BaseRow({
  day,
  label,
  shortLabel,
  control,
  register,
  setValue,
  getValues,
  errors,
  bandCount,
  expanded,
  onToggle,
  panelId,
}: DayRowProps & { bandCount: number; expanded: boolean; onToggle: () => void; panelId: string }) {
  const price = useWatch({ control, name: dayPriceField(day) });
  const dayError = errors[day]?.price;

  return (
    <PriceRow label={label} shortLabel={shortLabel}>
      <div className="flex shrink-0 items-center gap-2">
        <PriceField
          placeholder=""
          error={dayError?.message}
          aria-label={`${t.courts.price} ${label}`}
          {...register(dayPriceField(day), { valueAsNumber: true })}
        />
        {price > 0 && <ApplyToAllButton onClick={applyToAll(day, getValues, setValue)} />}
        <DisclosureButton dayLabel={label} expanded={expanded} count={bandCount} panelId={panelId} onClick={onToggle} />
      </div>
    </PriceRow>
  );
}

/**
 * Copies this day's full-day price onto every day's full-day price.
 *
 * Only the full-day price, never a day's differentiated rows: a row is an
 * exception the owner set on purpose for THAT day (an evening surcharge on
 * Friday means nothing about Monday), so "aplicar a todos" now leaves every
 * day's own exceptions exactly as they were, and only lines up the base rate
 * they all share.
 */
function applyToAll(
  day: DayType,
  getValues: UseFormGetValues<PriceFormValues>,
  setValue: UseFormSetValue<PriceFormValues>,
) {
  return () => {
    const val = getValues(dayPriceField(day));
    if (!val || val <= 0) return;
    for (const { value: target } of ALL_DAYS) {
      setValue(dayPriceField(target), val, { shouldValidate: true });
    }
    toast.success(t.courts.pricesAppliedToAll);
  };
}

/**
 * Adds a day's differentiated row — "Nuevo precio", not "Agregar franja":
 * this button is offering to price an exception on top of the full-day rate,
 * not growing a plain list the way the rows below it, once added, are still
 * called ("franja").
 *
 * Full-width and last inside the panel, dashed-bordered like an "add a row"
 * affordance rather than a small button that happens to float above the rows
 * it adds to — see the panel it renders in for why the order matters. `w-full`
 * is what makes its box share the rows' own left and right edges instead of
 * only hugging its own text width.
 *
 * Visible text stays exactly "Nuevo precio", matching the owner's own
 * wording; the day is folded into the accessible name as a leading,
 * visually-hidden prefix rather than into the visible label, so seven of
 * these across the dialog stay distinguishable to a screen reader without
 * seven different labels on screen.
 */
function NewPriceButton({ dayLabel, onClick }: { dayLabel: string; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="border-border-subtle text-text-secondary hover:bg-bg-highlight hover:text-text-primary hover:border-border-interactive mt-1 flex min-h-11 w-full items-center justify-center gap-1.5 rounded-lg border border-dashed px-2 text-xs font-medium transition-colors sm:min-h-9"
    >
      <Plus className="size-3.5" />
      <span className="sr-only">{dayLabel}: </span>
      {t.courts.newPrice}
    </button>
  );
}

/**
 * Opens or closes a day's differentiated rows.
 *
 * Named per day AND per direction, same reason `NewPriceButton` folds the day
 * into its own name: seven of these sit in the dialog, and a screen reader
 * offered seven identical "Mostrar franjas" targets could not tell which day
 * any of them opens, or whether it is already open. The row count, when
 * there is one, rides along in the same accessible name and is also drawn as
 * a small badge, so a day with existing exceptions says so before the owner
 * ever expands it.
 */
function DisclosureButton({
  dayLabel,
  expanded,
  count,
  panelId,
  onClick,
}: {
  dayLabel: string;
  expanded: boolean;
  count: number;
  panelId: string;
  onClick: () => void;
}) {
  const direction = expanded ? t.courts.hideBandsFor : t.courts.showBandsFor;
  const name = count > 0 ? `${direction} ${dayLabel} (${String(count)})` : `${direction} ${dayLabel}`;
  return (
    <button
      type="button"
      title={name}
      aria-label={name}
      aria-expanded={expanded}
      aria-controls={panelId}
      onClick={onClick}
      className="text-text-secondary hover:bg-bg-highlight hover:text-text-primary flex size-11 shrink-0 items-center justify-center gap-0.5 rounded-lg transition-colors sm:size-9"
    >
      {count > 0 && (
        <span aria-hidden="true" className="text-micro text-text-tertiary tabular-nums">
          {count}
        </span>
      )}
      <ChevronDown className={cn('size-3.5 transition-transform', expanded && 'rotate-180')} />
    </button>
  );
}
