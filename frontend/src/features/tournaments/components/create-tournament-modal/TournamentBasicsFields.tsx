import { type UseFormReturn } from 'react-hook-form';
import { DatePickerField } from '@/shared/components/common/DatePickerField';
import { FormField } from '@/shared/components/common/FormField';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import { FieldLabel } from './FieldLabel';
import { PADEL_SPORT, categoryField, type TournamentFormValues } from '../../schemas/tournamentForm.schema';
import type { Sport } from '@/shared/types/api.types';

const t = ES_AR.tournaments.form;
const SPORTS = Object.keys(ES_AR.courts.sportTypes) as Sport[];

type Form = UseFormReturn<TournamentFormValues>;

/**
 * A category's level and suma belong to padel alone (S15): changing the sport clears them. Padel
 * categories go back to a level; any other sport's take no level or suma (kind `none`).
 */
function clearCategoryTraits(form: Form, sport: string) {
  const kind = sport === PADEL_SPORT ? 'level' : 'none';
  form.getValues('categories').forEach((_, index) => {
    form.setValue(categoryField(index, 'kind'), kind);
    form.setValue(categoryField(index, 'level'), undefined);
    form.setValue(categoryField(index, 'suma'), undefined);
  });
}

/** The sport is the tournament's own characteristic: it words every category and sets which traits they take (S10, S15). */
function SportField({ form }: { form: Form }) {
  const { watch, setValue } = form;
  return (
    <>
      <FormField label={<FieldLabel text={t.sport} required />} htmlFor="tournament-sport">
        <Select
          value={watch('sport')}
          onValueChange={(sport) => {
            setValue('sport', sport);
            clearCategoryTraits(form, sport);
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

/** The sport and the first and last day of the tournament. */
export function TournamentBasicsFields({ form }: { form: Form }) {
  const { watch, setValue, formState } = form;
  const [start, end] = watch(['start_date', 'end_date']);
  return (
    <>
      <SportField form={form} />
      <DatePickerField
        id="tournament-start"
        label={<FieldLabel text={t.startDate} required />}
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
        label={<FieldLabel text={t.endDate} required />}
        value={end}
        placeholder={t.selectDate}
        error={formState.errors.end_date?.message}
        onSelect={(day) => {
          setValue('end_date', day, { shouldValidate: true });
        }}
      />
    </>
  );
}
