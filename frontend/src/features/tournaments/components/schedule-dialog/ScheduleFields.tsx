import { type UseFormReturn } from 'react-hook-form';
import { DatePickerField } from '@/shared/components/common/DatePickerField';
import { FormField } from '@/shared/components/common/FormField';
import { Input } from '@/shared/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import { generateTimeSlots } from '@/shared/lib/time';
import type { ScheduleFormValues } from '../../schemas/scheduleForm.schema';
import type { CourtWithPrices } from '@/shared/types/api.types';

const t = ES_AR.tournaments.schedule;
const TIME_OPTIONS = generateTimeSlots();

interface Props {
  form: UseFormReturn<ScheduleFormValues>;
  courts: CourtWithPrices[];
  /** The category's match duration, said under the duration field. */
  defaultDuration: number;
  /** The tournament's first and last day (`YYYY-MM-DD`): the only days a match can be scheduled. */
  firstDay: string;
  lastDay: string;
}

function CourtAndTime({ form, courts }: Pick<Props, 'form' | 'courts'>) {
  const { watch, setValue, formState } = form;
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      <FormField label={t.court} htmlFor="match-court" error={formState.errors.court_id?.message}>
        <Select
          value={watch('court_id')}
          onValueChange={(courtId) => {
            setValue('court_id', courtId, { shouldValidate: true });
          }}
        >
          <SelectTrigger id="match-court">
            <SelectValue placeholder={t.courtPlaceholder} />
          </SelectTrigger>
          <SelectContent>
            {courts.map((court) => (
              <SelectItem key={court.id} value={court.id}>
                {court.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </FormField>
      <FormField label={t.time} htmlFor="match-time" error={formState.errors.start_time?.message}>
        <Select
          value={watch('start_time')}
          onValueChange={(time) => {
            setValue('start_time', time, { shouldValidate: true });
          }}
        >
          <SelectTrigger id="match-time">
            <SelectValue placeholder="--:--" />
          </SelectTrigger>
          <SelectContent>
            {TIME_OPTIONS.map((time) => (
              <SelectItem key={time} value={time}>
                {time}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </FormField>
    </div>
  );
}

/** Court, day, time and duration of a match. */
export function ScheduleFields({ form, courts, defaultDuration, firstDay, lastDay }: Props) {
  const { watch, setValue, register, formState } = form;
  return (
    <>
      <CourtAndTime form={form} courts={courts} />
      <DatePickerField
        id="match-date"
        label={t.date}
        value={watch('date')}
        placeholder={ES_AR.tournaments.form.selectDate}
        error={formState.errors.date?.message}
        disablePast={false}
        minDate={firstDay}
        maxDate={lastDay}
        onSelect={(day) => {
          setValue('date', day, { shouldValidate: true });
        }}
      />
      <FormField label={t.duration} htmlFor="match-duration" error={formState.errors.duration_minutes?.message}>
        <Input
          id="match-duration"
          type="number"
          inputMode="numeric"
          min={30}
          max={480}
          step={15}
          {...register('duration_minutes', { valueAsNumber: true })}
        />
      </FormField>
      <p className="text-text-tertiary -mt-2 text-xs">{t.durationHint(defaultDuration)}</p>
    </>
  );
}
