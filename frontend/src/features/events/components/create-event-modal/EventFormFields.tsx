import { type UseFormReturn } from 'react-hook-form';
import { FormField } from '@/shared/components/common/FormField';
import { DatePickerField } from '@/shared/components/common/DatePickerField';
import { MoneyPesosField } from '@/shared/components/common/MoneyPesosField';
import { Input } from '@/shared/components/ui/input';
import { FieldRequirement } from '@/shared/components/common/FieldRequirement';
import { TimeSelectField } from '@/shared/components/common/TimeSelectField';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useEventTimeOptions } from '../../hooks/useEventTimeOptions';
import { eventDurationMinutes, type EventFormValues } from '../../schemas/eventForm.schema';
import { EventCourtsField } from './EventCourtsField';
import type { CourtWithPrices, Sport } from '@/shared/types/api.types';

const t = ES_AR;
const SPORTS = Object.keys(t.courts.sportTypes) as Sport[];

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

function NameAndSportFields({ form }: { form: Form }) {
  const { register, watch, setValue, formState } = form;
  return (
    <>
      <FormField label={required(t.events.form.name)} htmlFor="event-name" error={formState.errors.name?.message}>
        <Input id="event-name" maxLength={100} placeholder={t.events.form.namePlaceholder} {...register('name')} />
      </FormField>
      <FormField label={required(t.events.form.sport)} htmlFor="event-sport">
        <Select
          value={watch('sport')}
          onValueChange={(sport) => {
            setValue('sport', sport);
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
    </>
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
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
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
  return (
    <>
      <NameAndSportFields form={form} />
      <WhenFields form={form} />
      <EventCourtsField form={form} courts={courts} />
      <CapacityAndPriceFields form={form} />
    </>
  );
}
