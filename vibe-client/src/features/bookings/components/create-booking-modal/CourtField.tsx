import { useId } from 'react';
import { FormField } from '@/shared/components/common/FormField';
import { FieldRequirement } from '@/shared/components/common/FieldRequirement';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import { DurationField } from './DurationField';
import type { UseFormSetValue, FieldErrors } from 'react-hook-form';
import type { CourtWithPrices } from '@/shared/types/api.types';
import type { CreateBookingDto } from '../../schemas/booking.schemas';

const t = ES_AR;

export function CourtField({
  activeCourts,
  courtId,
  selectedCourt,
  durationMinutes,
  setValue,
  errors,
}: {
  activeCourts: CourtWithPrices[];
  courtId: string | undefined;
  selectedCourt: CourtWithPrices | undefined;
  durationMinutes: number;
  setValue: UseFormSetValue<CreateBookingDto>;
  errors: FieldErrors<CreateBookingDto>;
}) {
  const labelId = useId();
  return (
    <div className="space-y-2">
      <FormField
        className="space-y-2"
        htmlFor={labelId}
        label={
          <>
            {t.bookings.court}
            <FieldRequirement required />
          </>
        }
        error={errors.court_id?.message}
      >
        <Select
          {...(courtId !== undefined ? { value: courtId } : {})}
          onValueChange={(v) => {
            setValue('court_id', v);
            setValue('start_time', '');
          }}
        >
          <SelectTrigger id={labelId} aria-label={t.bookings.court}>
            <SelectValue placeholder={t.bookings.selectCourt} />
          </SelectTrigger>
          <SelectContent>
            {activeCourts.map((court) => (
              <SelectItem key={court.id} value={court.id}>
                {court.name} · {t.courts.sportTypes[court.sport]}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </FormField>
      {selectedCourt && <DurationField durationMinutes={durationMinutes} setValue={setValue} errors={errors} />}
    </div>
  );
}
