import { type UseFormReturn } from 'react-hook-form';
import { DatePickerField } from '@/shared/components/common/DatePickerField';
import { FormField } from '@/shared/components/common/FormField';
import { MoneyInput } from '@/shared/components/common/MoneyInput';
import { TimeSelectField } from '@/shared/components/common/TimeSelectField';
import { Input } from '@/shared/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { Textarea } from '@/shared/components/ui/textarea';
import { ES_AR } from '@/shared/i18n/es_AR';
import { LEVELS, levelLabel, modalityLabel, type Modality } from '@/shared/lib/traitLabels';
import { SegmentedTabs } from '../SegmentedTabs';
import { PADEL_SPORT, categoryField, type TournamentFormValues } from '../../schemas/tournamentForm.schema';
import { CategoryGroupFields } from './CategoryGroupFields';
import { CategoryMatchFormatFields } from './CategoryMatchFormatFields';
import { FieldLabel } from './FieldLabel';

const t = ES_AR.tournaments;
const MODALITIES: readonly Modality[] = ['men', 'women', 'mixed'];
const PRIZES_MAX = 200;

interface Props {
  form: UseFormReturn<TournamentFormValues>;
  index: number;
}

/** The category's modality, required on every sport (S2, S15), in the words its sport uses. */
function ModalityField({ form, index, sport }: Props & { sport: string }) {
  const modality = form.watch(categoryField(index, 'modality'));
  const error = form.formState.errors.categories?.[index]?.modality?.message;
  const items = MODALITIES.map((value) => ({ value, label: modalityLabel(value, sport) }));
  return (
    <div className="space-y-2">
      <p className="text-text-primary text-sm font-medium">
        <FieldLabel text={t.form.modality} required />
      </p>
      <SegmentedTabs<Modality | ''>
        items={items}
        active={modality ?? ''}
        onChange={(value) => {
          if (value) form.setValue(categoryField(index, 'modality'), value, { shouldValidate: true });
        }}
        ariaLabel={t.form.modality}
      />
      {error && (
        <p role="alert" className="text-error-text text-xs">
          {error}
        </p>
      )}
    </div>
  );
}

/** "Tipo": a level (1ra to 9na) or a suma. Switching clears the other trait, so only one is ever sent. */
function KindField({ form, index }: Props) {
  const { setValue, watch } = form;
  const kind = watch(categoryField(index, 'kind'));
  return (
    <FormField label={<FieldLabel text={t.form.kind} required />} htmlFor={`category-${String(index)}-kind`}>
      <Select
        value={kind}
        onValueChange={(value) => {
          if (value === 'suma') {
            setValue(categoryField(index, 'kind'), 'suma');
            setValue(categoryField(index, 'level'), undefined);
          } else {
            setValue(categoryField(index, 'kind'), 'level');
            setValue(categoryField(index, 'suma'), undefined);
          }
        }}
      >
        <SelectTrigger id={`category-${String(index)}-kind`}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="level">{t.form.kindCategory}</SelectItem>
          <SelectItem value="suma">{t.form.kindSuma}</SelectItem>
        </SelectContent>
      </Select>
    </FormField>
  );
}

/** A select of 1ra to 9na for a level category, or a number from 2 to 18 for a suma. */
function LevelOrSumaField({ form, index }: Props) {
  const { register, setValue, watch, formState } = form;
  const errors = formState.errors.categories?.[index];
  const kind = watch(categoryField(index, 'kind'));
  const level = watch(categoryField(index, 'level'));
  const id = `category-${String(index)}`;

  if (kind === 'suma') {
    return (
      <FormField
        label={<FieldLabel text={t.form.suma} required />}
        htmlFor={`${id}-suma`}
        helpText={t.form.sumaHint}
        error={errors?.suma?.message}
      >
        <Input
          id={`${id}-suma`}
          type="number"
          inputMode="numeric"
          min={2}
          max={18}
          {...register(categoryField(index, 'suma'), {
            setValueAs: (value: string) => (value === '' ? undefined : Number(value)),
          })}
        />
      </FormField>
    );
  }

  return (
    <FormField
      label={<FieldLabel text={t.form.level} required />}
      htmlFor={`${id}-level`}
      error={errors?.level?.message}
    >
      <Select
        value={level === undefined ? '' : String(level)}
        onValueChange={(value) => {
          setValue(categoryField(index, 'level'), Number(value), { shouldValidate: true });
        }}
      >
        <SelectTrigger id={`${id}-level`}>
          <SelectValue placeholder={t.form.levelPlaceholder} />
        </SelectTrigger>
        <SelectContent>
          {LEVELS.map((value) => (
            <SelectItem key={value} value={String(value)}>
              {levelLabel(value)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </FormField>
  );
}

/** When the category starts: a day inside the tournament's days, and the time it starts (S3). */
function StartsFields({ form, index }: Props) {
  const { watch, setValue, formState } = form;
  const [tournamentStart, tournamentEnd] = watch(['start_date', 'end_date']);
  const startsOn = watch(categoryField(index, 'starts_on'));
  const startsAt = watch(categoryField(index, 'starts_at'));
  const errors = formState.errors.categories?.[index];
  const id = `category-${String(index)}`;
  return (
    <div className="grid gap-4 sm:grid-cols-2">
      <DatePickerField
        id={`${id}-starts-on`}
        label={<FieldLabel text={t.form.startsOn} required />}
        value={startsOn}
        placeholder={t.form.selectDate}
        error={errors?.starts_on?.message}
        {...(tournamentStart ? { minDate: tournamentStart } : {})}
        {...(tournamentEnd ? { maxDate: tournamentEnd } : {})}
        onSelect={(day) => {
          setValue(categoryField(index, 'starts_on'), day, { shouldValidate: true });
        }}
      />
      <TimeSelectField
        id={`${id}-starts-at`}
        label={<FieldLabel text={t.form.startsAt} required />}
        value={startsAt}
        error={errors?.starts_at?.message}
        onChange={(time) => {
          setValue(categoryField(index, 'starts_at'), time, { shouldValidate: true });
        }}
      />
    </div>
  );
}

function FormatField({ form, index }: Props) {
  return (
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

/** Optional prizes, free text up to 200 characters, with a counter (S1). */
function PrizesField({ form, index }: Props) {
  const errors = form.formState.errors.categories?.[index];
  const prizes = form.watch(categoryField(index, 'prizes'));
  const id = `category-${String(index)}-prizes`;
  return (
    <FormField
      label={<FieldLabel text={t.form.prizes} required={false} />}
      htmlFor={id}
      labelSuffix={t.form.prizesCounter(prizes.length)}
      error={errors?.prizes?.message}
    >
      <Textarea
        id={id}
        rows={3}
        maxLength={PRIZES_MAX}
        placeholder={t.form.prizesPlaceholder}
        {...form.register(categoryField(index, 'prizes'))}
      />
    </FormField>
  );
}

/**
 * First view of a category: its modality, when it starts, format, capacity, price, court fee and prizes.
 * Only padel also takes a level or a suma (S15).
 */
export function CategoryBasicsFields({ form, index }: Props) {
  const sport = form.watch('sport');
  return (
    <>
      <ModalityField form={form} index={index} sport={sport} />
      {sport === PADEL_SPORT && (
        <div className="grid gap-4 sm:grid-cols-2">
          <KindField form={form} index={index} />
          <LevelOrSumaField form={form} index={index} />
        </div>
      )}
      <StartsFields form={form} index={index} />
      <FormatField form={form} index={index} />
      <CapacityAndPrice form={form} index={index} />
      <PrizesField form={form} index={index} />
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
