import { type UseFormReturn } from 'react-hook-form';
import { FormField } from '@/shared/components/common/FormField';
import { MoneyInput } from '@/shared/components/common/MoneyInput';
import { Button } from '@/shared/components/ui/button';
import { AccordionContent, AccordionItem, AccordionTrigger } from '@/shared/components/ui/accordion';
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
  canRemove: boolean;
  onRemove: () => void;
}

function NameAndFormat({ form, index }: Pick<Props, 'form' | 'index'>) {
  const errors = form.formState.errors.categories?.[index];
  return (
    <>
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
    </>
  );
}

function CapacityAndPrice({ form, index }: Pick<Props, 'form' | 'index'>) {
  const errors = form.formState.errors.categories?.[index];
  const id = `category-${String(index)}`;
  return (
    <>
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

/**
 * One category of the create form, as an accordion item: its header names it
 * ("Categoría 1: 6ta") and its body holds the name, format, capacity, price per
 * pair, court fee, group settings and match format in one column.
 */
export function CategoryFields({ form, index, canRemove, onRemove }: Props) {
  const isGroups = form.watch(categoryField(index, 'format')) === 'groups';
  const name = form.watch(categoryField(index, 'name'));
  return (
    <AccordionItem value={String(index)}>
      <AccordionTrigger className="text-text-primary text-sm font-semibold tracking-normal normal-case">
        {t.form.categoryTitle(index + 1, name)}
      </AccordionTrigger>
      <AccordionContent className="space-y-4">
        <NameAndFormat form={form} index={index} />
        <CapacityAndPrice form={form} index={index} />
        {isGroups && <CategoryGroupFields form={form} index={index} />}
        <CategoryMatchFormatFields form={form} index={index} />
        {canRemove && (
          <Button type="button" variant="ghost" size="sm" className="rounded-lg" onClick={onRemove}>
            {t.form.removeCategory}
          </Button>
        )}
      </AccordionContent>
    </AccordionItem>
  );
}
