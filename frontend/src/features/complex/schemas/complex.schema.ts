import { z } from 'zod';
import { ES_AR } from '@/shared/i18n/es_AR';
import { optionalEmailField } from '@/shared/lib/validations';

const t = ES_AR;

const slugRegex = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

/**
 * The amenity vocabulary, matching the server's `knownAmenities` map and
 * the complexes_amenities_known CHECK constraint. A value outside it is refused by the
 * database, so the form must not be able to produce one.
 */
export const AMENITY_VALUES = [
  'parking',
  'changing_rooms',
  'showers',
  'bar',
  'racket_rental',
  'pro_shop',
  'wifi',
  'lockers',
  'lessons',
  'tournaments',
  'accessible',
  'match_recording',
] as const;

const complexShape = {
  name: z.string().min(1, t.validation.nameRequired).max(200, t.validation.maxChars200),
  slug: z.string().min(1, t.validation.slugRequired).regex(slugRegex, t.validation.slugFormat),
  formatted_address: z.string().min(1, t.validation.addressRequired),
  address: z.string(),
  city: z.string(),
  province: z.string(),
  latitude: z.number().optional(),
  longitude: z.number().optional(),
  phone: z.string().min(1, t.validation.phoneRequired),
  email: optionalEmailField,
  deposit_percentage: z.number().min(0, t.validation.min0Percent).max(100, t.validation.max100Percent),
  cancellation_hours: z.number().min(1, t.validation.min1Hour).max(168, t.validation.max168HoursWeek),
  amenities: z.array(z.enum(AMENITY_VALUES)),
};

// Creating a complex requires resolved coordinates (the address autocomplete must
// have run at least once). Editing one must not force re-picking the address just
// to change an unrelated field, so coordinates stay optional on updateComplexSchema
// — a complex can have null latitude/longitude (e.g. seeded outside this form) and
// must still be editable.
export const createComplexSchema = z
  .object(complexShape)
  .refine((data) => data.latitude != null && data.longitude != null, {
    message: t.validation.addressRequired,
    path: ['latitude'],
  });

export const updateComplexSchema = z.object(complexShape);

const scheduleItemSchema = z
  .object({
    day: z.enum(['monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday']),
    open_time: z.string(),
    close_time: z.string(),
    is_closed: z.boolean(),
  })
  .refine((data) => data.is_closed || data.open_time !== data.close_time, {
    message: t.validation.closeNotEqualOpen,
    path: ['close_time'],
  });

export const updateSchedulesSchema = z.object({
  schedules: z.array(scheduleItemSchema).length(7, t.validation.exactly7Days),
});

export type CreateComplexDto = z.infer<typeof createComplexSchema>;
export type UpdateSchedulesDto = z.infer<typeof updateSchedulesSchema>;
