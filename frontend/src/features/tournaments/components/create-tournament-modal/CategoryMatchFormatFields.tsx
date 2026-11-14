import { type UseFormReturn } from 'react-hook-form';
import { FormField } from '@/shared/components/common/FormField';
import { Checkbox } from '@/shared/components/ui/checkbox';
import { Input } from '@/shared/components/ui/input';
import { Label } from '@/shared/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import { FieldLabel } from './FieldLabel';
import { categoryField, type TournamentFormValues } from '../../schemas/tournamentForm.schema';

const t = ES_AR.tournaments.form;

interface Props {
  form: UseFormReturn<TournamentFormValues>;
  index: number;
}

/**
 * How the category plays its matches: one set or best of three, the games a set
 * goes to, whether the third set is a match tiebreak (only for best of three)
 * and how long a match holds its court.
 */
export function CategoryMatchFormatFields({ form, index }: Props) {
  const { watch, setValue } = form;
  const setsToWin = watch(categoryField(index, 'sets_to_win'));
  const superOn = watch(categoryField(index, 'super_tiebreak_decider'));
  const id = `category-${String(index)}`;

  return (
    <fieldset className="space-y-3">
      <legend className="text-text-primary text-sm font-medium">{t.matchFormat}</legend>
      <Select
        value={String(setsToWin)}
        onValueChange={(value) => {
          setValue(categoryField(index, 'sets_to_win'), value === '1' ? 1 : 2);
        }}
      >
        <SelectTrigger aria-label={t.matchFormat}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="2">{t.setsToWin[2]}</SelectItem>
          <SelectItem value="1">{t.setsToWin[1]}</SelectItem>
        </SelectContent>
      </Select>
      <GamesAndDuration form={form} index={index} />
      {setsToWin === 2 && (
        <div className="flex items-center gap-2.5">
          <Checkbox
            id={`${id}-super`}
            checked={superOn}
            onCheckedChange={(checked) => {
              setValue(categoryField(index, 'super_tiebreak_decider'), checked === true);
            }}
          />
          <Label htmlFor={`${id}-super`} className="min-w-0 text-sm">
            {t.superTiebreak}
          </Label>
        </div>
      )}
    </fieldset>
  );
}

function GamesAndDuration({ form, index }: Props) {
  const { register, formState } = form;
  const errors = formState.errors.categories?.[index];
  const id = `category-${String(index)}`;
  return (
    <>
      <FormField
        label={<FieldLabel text={t.gamesPerSet} required />}
        htmlFor={`${id}-games`}
        error={errors?.games_per_set?.message}
      >
        <Input
          id={`${id}-games`}
          type="number"
          inputMode="numeric"
          min={4}
          max={9}
          {...register(categoryField(index, 'games_per_set'), { valueAsNumber: true })}
        />
      </FormField>
      <FormField
        label={<FieldLabel text={t.matchDuration} required />}
        htmlFor={`${id}-duration`}
        error={errors?.match_duration_minutes?.message}
      >
        <Input
          id={`${id}-duration`}
          type="number"
          inputMode="numeric"
          min={30}
          max={480}
          step={15}
          {...register(categoryField(index, 'match_duration_minutes'), { valueAsNumber: true })}
        />
      </FormField>
    </>
  );
}
