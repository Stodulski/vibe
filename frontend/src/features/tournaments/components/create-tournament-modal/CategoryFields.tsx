import { type UseFormReturn } from 'react-hook-form';
import { FormField } from '@/shared/components/common/FormField';
import { MoneyPesosField } from '@/shared/components/common/MoneyPesosField';
import { Button } from '@/shared/components/ui/button';
import { Input } from '@/shared/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import { categoryField, type TournamentFormValues } from '../../schemas/tournamentForm.schema';
import { CategoryGroupFields } from './CategoryGroupFields';
import { CategoryMatchFormatFields } from './CategoryMatchFormatFields';

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
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      <FormField label={t.form.categoryName} htmlFor={`category-${String(index)}-name`} error={errors?.name?.message}>
        <Input
          id={`category-${String(index)}-name`}
          maxLength={30}
          placeholder={t.form.categoryNamePlaceholder}
          {...form.register(categoryField(index, 'name'))}
        />
      </FormField>
      <FormField label={t.form.categoryFormat} htmlFor={`category-${String(index)}-format`}>
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

function CapacityAndPrice({ form, index }: Pick<Props, 'form' | 'index'>) {
  const errors = form.formState.errors.categories?.[index];
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      <FormField
        label={t.form.capacity}
        htmlFor={`category-${String(index)}-capacity`}
        error={errors?.capacity?.message}
      >
        <Input
          id={`category-${String(index)}-capacity`}
          type="number"
          inputMode="numeric"
          min={2}
          max={128}
          {...form.register(categoryField(index, 'capacity'), { valueAsNumber: true })}
        />
      </FormField>
      <MoneyPesosField
        id={`category-${String(index)}-price`}
        label={t.form.price}
        value={form.watch(categoryField(index, 'price'))}
        onChange={(pesos) => {
          form.setValue(categoryField(index, 'price'), pesos ?? Number.NaN, { shouldValidate: true });
        }}
        error={errors?.price?.message}
        placeholder="0"
      />
      <p className="text-text-tertiary text-xs sm:col-span-2">{t.form.priceHint}</p>
      <MoneyPesosField
        id={`category-${String(index)}-court-fee`}
        label={t.form.courtFee}
        value={form.watch(categoryField(index, 'court_fee'))}
        onChange={(pesos) => {
          form.setValue(categoryField(index, 'court_fee'), pesos, { shouldValidate: true });
        }}
        error={errors?.court_fee?.message}
        placeholder="0"
      />
      <p className="text-text-tertiary text-xs sm:col-span-2">{t.form.courtFeeHint}</p>
    </div>
  );
}

/** One category of the create form: name, format, capacity, price per pair, court fee, group settings and match format. */
export function CategoryFields({ form, index, canRemove, onRemove }: Props) {
  const isGroups = form.watch(categoryField(index, 'format')) === 'groups';
  return (
    <section
      aria-label={t.form.category(index + 1)}
      className="border-border-subtle space-y-4 rounded-2xl border p-3.5 sm:p-4"
    >
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-text-primary text-sm font-semibold">{t.form.category(index + 1)}</h3>
        {canRemove && (
          <Button type="button" variant="ghost" size="sm" className="rounded-lg" onClick={onRemove}>
            {t.form.removeCategory}
          </Button>
        )}
      </div>
      <NameAndFormat form={form} index={index} />
      <CapacityAndPrice form={form} index={index} />
      {isGroups && <CategoryGroupFields form={form} index={index} />}
      <CategoryMatchFormatFields form={form} index={index} />
    </section>
  );
}
