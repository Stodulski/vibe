import { type UseFormReturn } from 'react-hook-form';
import { FormField } from '@/shared/components/common/FormField';
import { Input } from '@/shared/components/ui/input';
import { ES_AR } from '@/shared/i18n/es_AR';
import { FieldLabel } from './FieldLabel';
import { categoryField, type TournamentFormValues } from '../../schemas/tournamentForm.schema';

const t = ES_AR.tournaments.form;

/** Pairs per group and how many of each advance: set by the owner per category (defaults 3 and 2). */
export function CategoryGroupFields({ form, index }: { form: UseFormReturn<TournamentFormValues>; index: number }) {
  const errors = form.formState.errors.categories?.[index];
  const id = `category-${String(index)}`;
  return (
    <>
      <FormField
        label={<FieldLabel text={t.groupSize} required />}
        htmlFor={`${id}-group-size`}
        helpText={t.groupSizeHint}
        error={errors?.group_size?.message}
      >
        <Input
          id={`${id}-group-size`}
          type="number"
          inputMode="numeric"
          min={2}
          max={12}
          {...form.register(categoryField(index, 'group_size'), {
            valueAsNumber: true,
            onChange: () => void form.trigger(categoryField(index, 'qualifiers_per_group')),
          })}
        />
      </FormField>
      <FormField
        label={<FieldLabel text={t.qualifiers} required />}
        htmlFor={`${id}-qualifiers`}
        helpText={t.qualifiersHint}
        error={errors?.qualifiers_per_group?.message}
      >
        <Input
          id={`${id}-qualifiers`}
          type="number"
          inputMode="numeric"
          min={1}
          {...form.register(categoryField(index, 'qualifiers_per_group'), { valueAsNumber: true })}
        />
      </FormField>
    </>
  );
}
