import { type UseFormReturn } from 'react-hook-form';
import { FormField } from '@/shared/components/common/FormField';
import { DatePickerField } from '@/shared/components/common/DatePickerField';
import { MoneyPesosField } from '@/shared/components/common/MoneyPesosField';
import { Input } from '@/shared/components/ui/input';
import { FieldRequirement } from '@/shared/components/common/FieldRequirement';
import { TimeSelectField } from '@/shared/components/common/TimeSelectField';
import { Button } from '@/shared/components/ui/button';
import { Label } from '@/shared/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { Switch } from '@/shared/components/ui/switch';
import { ES_AR } from '@/shared/i18n/es_AR';
import { LEVELS, PADEL_SPORT, levelLabel, modalityLabel, type Modality } from '@/shared/lib/traitLabels';
import { cn } from '@/shared/lib/utils';
import { useEventTimeOptions } from '../../hooks/useEventTimeOptions';
import { eventDurationMinutes, type EventFormValues } from '../../schemas/eventForm.schema';
import { EventCourtsField } from './EventCourtsField';
import type { CourtWithPrices, Sport } from '@/shared/types/api.types';

const t = ES_AR;
const SPORTS = Object.keys(t.courts.sportTypes) as Sport[];
const MODALITIES: readonly Modality[] = ['men', 'women', 'mixed'];

type Form = UseFormReturn<EventFormValues>;

/** A label with its "(requerido)": every field of this form is, and saying so beats a legend. */
function required(label: string) {
  return (
    <>
      {label}
      <FieldRequirement required />
    </>
  );
}

function SportField({ form }: { form: Form }) {
  const { watch, setValue, formState } = form;
  return (
    <FormField label={required(t.events.form.sport)} htmlFor="event-sport" error={formState.errors.sport?.message}>
      <Select
        value={watch('sport')}
        onValueChange={(sport) => {
          setValue('sport', sport);
          // A level range is padel's only: switching to another sport clears it back to every level.
          if (sport !== PADEL_SPORT) {
            setValue('allLevels', true);
            setValue('levelMin', undefined);
            setValue('levelMax', undefined);
          }
        }}
      >
        <SelectTrigger id="event-sport">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {SPORTS.map((sport) => (
            <SelectItem key={sport} value={sport}>
              {t.courts.sportTypes[sport]}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </FormField>
  );
}

/** Who may sign up, required on every sport (S14, S15), in the words its sport uses; the title is composed from it. */
function ModalityField({ form, sport }: { form: Form; sport: string }) {
  const modality = form.watch('modality');
  const error = form.formState.errors.modality?.message;
  return (
    <div className="space-y-2">
      <p className="text-text-primary text-sm font-medium">{required(t.events.form.modality)}</p>
      <div role="group" aria-label={t.events.form.modality} className="flex flex-wrap gap-2">
        {MODALITIES.map((value) => {
          const selected = modality === value;
          return (
            <Button
              key={value}
              type="button"
              variant={selected ? 'default' : 'outline'}
              aria-pressed={selected}
              className={cn(
                'h-10 flex-1 rounded-xl border px-3 sm:flex-none sm:px-5',
                selected && 'border-transparent',
              )}
              onClick={() => {
                form.setValue('modality', value, { shouldValidate: true });
              }}
            >
              {modalityLabel(value, sport)}
            </Button>
          );
        })}
      </div>
      {error && (
        <p role="alert" className="text-error-text text-xs">
          {error}
        </p>
      )}
    </div>
  );
}

function LevelSelect({
  id,
  label,
  value,
  error,
  onChange,
}: {
  id: string;
  label: string;
  value: number | undefined;
  error: string | undefined;
  onChange: (level: number) => void;
}) {
  return (
    <FormField label={label} htmlFor={id} error={error}>
      <Select
        value={value === undefined ? '' : String(value)}
        onValueChange={(next) => {
          onChange(Number(next));
        }}
      >
        <SelectTrigger id={id}>
          <SelectValue placeholder={t.events.form.levelPlaceholder} />
        </SelectTrigger>
        <SelectContent>
          {LEVELS.map((level) => (
            <SelectItem key={level} value={String(level)}>
              {levelLabel(level)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </FormField>
  );
}

/**
 * Every category, or a range of them: "Desde" is the strongest, "Hasta" the weakest. Both ends or
 * neither; the toggle is on for neither (the default).
 */
function LevelsField({ form }: { form: Form }) {
  const { watch, setValue, formState } = form;
  const { errors } = formState;
  const allLevels = watch('allLevels');
  return (
    <fieldset className="space-y-3">
      <legend className="text-text-primary text-sm font-medium">{t.events.form.levels}</legend>
      <div className="flex items-center gap-2.5">
        <Switch
          id="event-all-levels"
          checked={allLevels}
          onCheckedChange={(checked) => {
            setValue('allLevels', checked, { shouldValidate: true });
          }}
        />
        <Label htmlFor="event-all-levels" className="min-w-0 text-sm">
          {t.events.form.allLevels}
        </Label>
      </div>
      {!allLevels && (
        <div className="grid grid-cols-2 gap-3">
          <LevelSelect
            id="event-level-min"
            label={t.events.form.levelFrom}
            value={watch('levelMin')}
            error={errors.levelMin?.message}
            onChange={(level) => {
              setValue('levelMin', level, { shouldValidate: true });
            }}
          />
          <LevelSelect
            id="event-level-max"
            label={t.events.form.levelTo}
            value={watch('levelMax')}
            error={errors.levelMax?.message}
            onChange={(level) => {
              setValue('levelMax', level, { shouldValidate: true });
            }}
          />
        </div>
      )}
      {!allLevels && <p className="text-text-tertiary text-xs">{t.events.form.levelsHint}</p>}
    </fieldset>
  );
}

function WhenFields({ form }: { form: Form }) {
  const { watch, setValue, formState } = form;
  const { errors } = formState;
  const [date, start, end] = watch(['date', 'start_time', 'end_time']);
  // An end at or before the start is read as the next day, so say it.
  const pastMidnight = !!start && !!end && eventDurationMinutes(start, end) > 0 && end <= start;
  const timeOptions = useEventTimeOptions([start, end]);

  return (
    <>
      <DatePickerField
        id="event-date"
        label={required(t.events.form.date)}
        side="top"
        value={date}
        placeholder={t.events.form.selectDate}
        error={errors.date?.message}
        onSelect={(day) => {
          setValue('date', day, { shouldValidate: true });
        }}
      />
      <div className="grid grid-cols-2 gap-3">
        <TimeSelectField
          id="event-start"
          label={required(t.events.form.startTime)}
          options={timeOptions}
          value={start}
          error={errors.start_time?.message}
          onChange={(time) => {
            setValue('start_time', time, { shouldValidate: true });
            if (end) void form.trigger('end_time');
          }}
        />
        <TimeSelectField
          id="event-end"
          label={required(t.events.form.endTime)}
          options={timeOptions}
          value={end}
          error={errors.end_time?.message}
          onChange={(time) => {
            setValue('end_time', time, { shouldValidate: true });
          }}
        />
      </div>
      {pastMidnight && <p className="text-text-tertiary -mt-2 text-xs">{t.events.endsNextDay}</p>}
    </>
  );
}

function CapacityAndPriceFields({ form }: { form: Form }) {
  const { register, watch, setValue, formState } = form;
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      <FormField
        label={required(t.events.form.capacity)}
        htmlFor="event-capacity"
        helpText={t.events.form.capacityHint}
        error={formState.errors.capacity?.message}
      >
        <Input
          id="event-capacity"
          type="number"
          inputMode="numeric"
          min={1}
          max={500}
          {...register('capacity', { valueAsNumber: true })}
        />
      </FormField>
      <MoneyPesosField
        id="event-price"
        label={required(t.events.form.price)}
        helpText={t.events.form.priceHint}
        value={watch('price')}
        onChange={(pesos) => {
          setValue('price', pesos ?? Number.NaN, { shouldValidate: true });
        }}
        error={formState.errors.price?.message}
        placeholder="0"
      />
    </div>
  );
}

export function EventFormFields({ form, courts }: { form: Form; courts: CourtWithPrices[] }) {
  const sport = form.watch('sport');
  return (
    <>
      <SportField form={form} />
      <ModalityField form={form} sport={sport} />
      {sport === PADEL_SPORT && <LevelsField form={form} />}
      <WhenFields form={form} />
      <EventCourtsField form={form} courts={courts} />
      <CapacityAndPriceFields form={form} />
    </>
  );
}
