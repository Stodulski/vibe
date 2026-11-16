import { type UseFormReturn } from 'react-hook-form';
import { FormField } from '@/shared/components/common/FormField';
import { MoneyInput } from '@/shared/components/common/MoneyInput';
import { Input } from '@/shared/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import { categoryField, type TournamentFormValues } from '../../schemas/tournamentForm.schema';
import { CategoryGroupFields } from './CategoryGroupFields';
import { CategoryMatchFormatFields } from './CategoryMatchFormatFields';
import { FieldLabel } from './FieldLabel';

const t = ES_AR.tournaments;

interface Props {
  form: UseFormReturn<TournamentFormValues>;
  index: number;
}

function NameAndFormat({ form, index }: Props) {
  const errors = form.formState.errors.categories?.[index];
  return (
    <div className="grid gap-4 sm:grid-cols-2">
      <FormField
        label={<FieldLabel text={t.form.categoryName} required />}
        htmlFor={`category-${String(index)}-name`}
        error={errors?.name?.message}
      >
        <Input
          id={`category-${String(index)}-name`}
          maxLength={30}
          placeholder={t.form.categoryNamePlaceholder}
          {...form.register(categoryField(index, 'name'))}
        />
      </FormField>
      <FormField
        label={<FieldLabel text={t.form.categoryFormat} required />}
        htmlFor={`category-${String(index)}-format`}
      >
        <Select
          value={form.watch(categoryField(index, 'format'))}
          onValueChange={(value) => {
            form.setValue(categoryField(index, 'format'), value === 'groups' ? 'groups' : 'elimination', {
              shouldValidate: true,
            });
          }}
        >
          <SelectTrigger id={`category-${String(index)}-format`}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="groups">{t.format.groups}</SelectItem>
            <SelectItem value="elimination">{t.format.elimination}</SelectItem>
          </SelectContent>
        </Select>
      </FormField>
    </div>
  );
}

function CapacityAndPrice({ form, index }: Props) {
  const errors = form.formState.errors.categories?.[index];
  const id = `category-${String(index)}`;
  return (
    <>
      <div className="grid gap-4 sm:grid-cols-2">
        <FormField
          label={<FieldLabel text={t.form.capacity} required />}
          htmlFor={`${id}-capacity`}
          error={errors?.capacity?.message}
        >
          <Input
            id={`${id}-capacity`}
            type="number"
            inputMode="numeric"
            min={2}
            max={128}
            {...form.register(categoryField(index, 'capacity'), { valueAsNumber: true })}
          />
        </FormField>
        <FormField
          label={<FieldLabel text={t.form.price} required />}
          htmlFor={`${id}-price`}
          helpText={t.form.priceHint}
          error={errors?.price?.message}
        >
          <MoneyInput
            id={`${id}-price`}
            value={form.watch(categoryField(index, 'price'))}
            onChange={(pesos) => {
              form.setValue(categoryField(index, 'price'), pesos ?? Number.NaN, { shouldValidate: true });
            }}
            placeholder="0"
          />
        </FormField>
      </div>
      <FormField
        label={<FieldLabel text={t.form.courtFee} required={false} />}
        htmlFor={`${id}-court-fee`}
        helpText={t.form.courtFeeHint}
        error={errors?.court_fee?.message}
      >
        <MoneyInput
          id={`${id}-court-fee`}
          value={form.watch(categoryField(index, 'court_fee'))}
          onChange={(pesos) => {
            form.setValue(categoryField(index, 'court_fee'), pesos, { shouldValidate: true });
          }}
          placeholder="0"
        />
      </FormField>
    </>
  );
}

/** First view of a category: its name, format, capacity, price per pair and court fee. */
export function CategoryBasicsFields({ form, index }: Props) {
  return (
    <>
      <NameAndFormat form={form} index={index} />
      <CapacityAndPrice form={form} index={index} />
    </>
  );
}

/** Second view of a category: how it plays. Zone settings only exist for the zones format. */
export function CategoryPlayFields({ form, index }: Props) {
  const isGroups = form.watch(categoryField(index, 'format')) === 'groups';
  return (
    <>
      {isGroups && <CategoryGroupFields form={form} index={index} />}
      <CategoryMatchFormatFields form={form} index={index} />
    </>
  );
}
