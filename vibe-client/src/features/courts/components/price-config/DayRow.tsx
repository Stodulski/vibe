import {
  useWatch,
  type Control,
  type FieldErrors,
  type UseFormRegister,
  type UseFormSetValue,
  type UseFormGetValues,
} from 'react-hook-form';
import { toast } from 'sonner';
import { ES_AR } from '@/shared/i18n/es_AR';
import { ApplyToAllButton } from './ApplyToAllButton';
import { PriceRow, PriceField } from './PriceRow';
import { ALL_DAYS, type PriceFormValues } from './days';
import type { DayType } from '@/shared/types/api.types';

const t = ES_AR;

interface DayRowProps {
  day: DayType;
  label: string;
  shortLabel: string;
  control: Control<PriceFormValues>;
  register: UseFormRegister<PriceFormValues>;
  setValue: UseFormSetValue<PriceFormValues>;
  getValues: UseFormGetValues<PriceFormValues>;
  errors: FieldErrors<PriceFormValues>;
}

/**
 * One day's rate.
 *
 * Every day looks the same. Weekend rows used to be amber and weekday rows
 * green, which coloured a distinction the form does not make — each day is its
 * own field here, and Saturday is not a warning.
 */
export function DayRow({ day, label, shortLabel, control, register, setValue, getValues, errors }: DayRowProps) {
  const price = useWatch({ control, name: day });

  const applyToAll = () => {
    const val = getValues(day);
    if (!val || val <= 0) return;
    for (const { value } of ALL_DAYS) {
      setValue(value, val);
    }
    toast.success(t.courts.pricesAppliedToAll);
  };

  return (
    <PriceRow label={label} shortLabel={shortLabel}>
      <div className="flex shrink-0 items-center gap-2">
        <PriceField
          placeholder=""
          error={errors[day]?.message}
          aria-label={`${t.courts.price} ${label}`}
          {...register(day, { valueAsNumber: true })}
        />
        {price > 0 && <ApplyToAllButton onClick={applyToAll} />}
      </div>
    </PriceRow>
  );
}
