import { useId } from 'react';
import { FormField } from '@/shared/components/common/FormField';
import { FieldRequirement } from '@/shared/components/common/FieldRequirement';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { UseFormSetValue, FieldErrors } from 'react-hook-form';
import type { DurationMinutes } from '@/shared/types/api.types';
import type { CreateBookingDto } from '../../schemas/booking.schema';

const t = ES_AR;

const DURATION_OPTIONS: DurationMinutes[] = [60, 90, 120];

/** Duration select, shown once a court is picked — extracted from `CourtField` to keep it short. */
export function DurationField({
  durationMinutes,
  setValue,
  errors,
}: {
  durationMinutes: number;
  setValue: UseFormSetValue<CreateBookingDto>;
  errors: FieldErrors<CreateBookingDto>;
}) {
  const labelId = useId();
  return (
    <FormField
      className="space-y-2"
      htmlFor={labelId}
      label={
        <>
          {t.bookings.turnDuration}
          <FieldRequirement required />
        </>
      }
      error={errors.duration_minutes?.message}
    >
      <Select
        value={String(durationMinutes)}
        onValueChange={(v) => {
          setValue('duration_minutes', Number(v) as DurationMinutes);
        }}
      >
        <SelectTrigger id={labelId} aria-label={t.bookings.turnDuration} className="h-9 w-full text-xs">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {DURATION_OPTIONS.map((d) => (
            <SelectItem key={d} value={String(d)}>
              {t.courts.durations[d]}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </FormField>
  );
}
