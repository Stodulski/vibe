import { ES_AR } from '@/shared/i18n/es_AR';
import type { PriceFormSchema } from '../../schemas/courts.schema';
import type { DayType } from '@/shared/types/api.types';

const t = ES_AR;

export const ALL_DAYS: { value: DayType; label: string; short: string }[] = [
  { value: 'monday', label: t.complex.days.monday, short: t.courts.daysShort.monday },
  { value: 'tuesday', label: t.complex.days.tuesday, short: t.courts.daysShort.tuesday },
  { value: 'wednesday', label: t.complex.days.wednesday, short: t.courts.daysShort.wednesday },
  { value: 'thursday', label: t.complex.days.thursday, short: t.courts.daysShort.thursday },
  { value: 'friday', label: t.complex.days.friday, short: t.courts.daysShort.friday },
  { value: 'saturday', label: t.complex.days.saturday, short: t.courts.daysShort.saturday },
  { value: 'sunday', label: t.complex.days.sunday, short: t.courts.daysShort.sunday },
];

// Derived from the zod schema (`courts.schemas.ts`) instead of hand-written,
// so the two can't drift the way a parallel `Record<DayType, number>` could.
export type PriceFormValues = PriceFormSchema;

export const EMPTY_PRICE_FORM_VALUES: PriceFormValues = {
  monday: 0,
  tuesday: 0,
  wednesday: 0,
  thursday: 0,
  friday: 0,
  saturday: 0,
  sunday: 0,
};
