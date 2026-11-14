import { type UseFormReturn } from 'react-hook-form';
import { useSchedules, useSelectedComplex } from '@/features/complex';
import { TimeSelectField } from '@/shared/components/common/TimeSelectField';
import { DatePickerField } from '@/shared/components/common/DatePickerField';
import { FormField } from '@/shared/components/common/FormField';
import { Input } from '@/shared/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import { timeOptionsForOpeningHours } from '@/shared/lib/timeOptions';
import type { ScheduleFormValues } from '../../schemas/scheduleForm.schema';
import type { CourtWithPrices } from '@/shared/types/api.types';

const t = ES_AR.tournaments.schedule;

interface Props {
  form: UseFormReturn<ScheduleFormValues>;
  courts: CourtWithPrices[];
  /** The category's match duration, said under the duration field. */
  defaultDuration: number;
  /** The tournament's first and last day (`YYYY-MM-DD`): the only days a match can be scheduled. */
  firstDay: string;
  lastDay: string;
}

function TimeField({ form }: Pick<Props, 'form'>) {
  const { watch, setValue, formState } = form;
  const time = watch('start_time');
  const options = useTimeOptions(time);
  return (
    <TimeSelectField
      id="match-time"
      label={t.time}
      value={time}
      options={options}
      error={formState.errors.start_time?.message}
      onChange={(next) => {
        setValue('start_time', next, { shouldValidate: true });
      }}
    />
  );
}

/** The hours the selected complex is open (the one already chosen stays); the whole day until its schedule loads. */
function useTimeOptions(current: string): string[] {
  const { complex } = useSelectedComplex();
  const { data: schedules } = useSchedules(complex?.id ?? '', complex?.slug);
  return timeOptionsForOpeningHours(schedules, [current]);
}

function CourtField({ form, courts }: Pick<Props, 'form' | 'courts'>) {
  const { watch, setValue, formState } = form;
  return (
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
  );
}

/** Day, time, court and duration of a match, in that order, the way the event form asks for its day first. */
export function ScheduleFields({ form, courts, defaultDuration, firstDay, lastDay }: Props) {
  const { watch, setValue, register, formState } = form;
  return (
    <>
      <div className="grid gap-4 sm:grid-cols-2">
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
        <TimeField form={form} />
        <CourtField form={form} courts={courts} />
        <FormField
          label={t.duration}
          htmlFor="match-duration"
          helpText={t.durationHint(defaultDuration)}
          error={formState.errors.duration_minutes?.message}
        >
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
      </div>
    </>
  );
}
