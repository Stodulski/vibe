import { type UseFormReturn } from 'react-hook-form';
import { DatePickerField } from '@/shared/components/common/DatePickerField';
import { FormField } from '@/shared/components/common/FormField';
import { Input } from '@/shared/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { TournamentFormValues } from '../../schemas/tournamentForm.schema';
import type { Sport } from '@/shared/types/api.types';

const t = ES_AR.tournaments.form;
const SPORTS = Object.keys(ES_AR.courts.sportTypes) as Sport[];

type Form = UseFormReturn<TournamentFormValues>;

function NameAndSport({ form }: { form: Form }) {
  const { register, watch, setValue, formState } = form;
  return (
    <>
      <FormField label={t.name} htmlFor="tournament-name" error={formState.errors.name?.message}>
        <Input id="tournament-name" maxLength={60} placeholder={t.namePlaceholder} {...register('name')} />
      </FormField>
      <FormField label={t.sport} htmlFor="tournament-sport">
        <Select
          value={watch('sport')}
          onValueChange={(sport) => {
            setValue('sport', sport);
          }}
        >
          <SelectTrigger id="tournament-sport">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {SPORTS.map((sport) => (
              <SelectItem key={sport} value={sport}>
                {ES_AR.courts.sportTypes[sport]}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </FormField>
    </>
  );
}

/** Name, sport and the first and last day of the tournament. */
export function TournamentBasicsFields({ form }: { form: Form }) {
  const { watch, setValue, formState } = form;
  const [start, end] = watch(['start_date', 'end_date']);
  return (
    <>
      <NameAndSport form={form} />
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <DatePickerField
          id="tournament-start"
          label={t.startDate}
          value={start}
          placeholder={t.selectDate}
          error={formState.errors.start_date?.message}
          onSelect={(day) => {
            setValue('start_date', day, { shouldValidate: true });
            if (end) void form.trigger('end_date');
          }}
        />
        <DatePickerField
          id="tournament-end"
          label={t.endDate}
          value={end}
          placeholder={t.selectDate}
          error={formState.errors.end_date?.message}
          onSelect={(day) => {
            setValue('end_date', day, { shouldValidate: true });
          }}
        />
      </div>
    </>
  );
}
