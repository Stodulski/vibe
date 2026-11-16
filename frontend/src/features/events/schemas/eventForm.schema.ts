import { z } from 'zod';
import { ES_AR } from '@/shared/i18n/es_AR';
import { timeToMinutes } from '@/shared/lib/time';

const t = ES_AR;

const MIN_DURATION_MIN = 30;
const MAX_DURATION_MIN = 12 * 60;

/**
 * Minutes an event lasts from `start` to `end`. An end not after the start
 * means the event runs past midnight, the same reading the server has.
 */
export function eventDurationMinutes(start: string, end: string): number {
  const startMin = timeToMinutes(start);
  const endMin = timeToMinutes(end);
  return endMin <= startMin ? endMin + 24 * 60 - startMin : endMin - startMin;
}

/**
 * The create-event form's own shape, not the wire shape
 * (`CreateOpenPlayEventRequest`, built from it in `useCreateEventForm`).
 * `courtIds` empty with `allCourts` on means every active court, which the API
 * takes as an absent `court_ids`. The event has no typed name (S10): it is named
 * from its sport, modality and categories. `allLevels` on means no category range;
 * off means both ends of the range are set, `levelMin` the strongest (S14).
 */
export const eventFormSchema = z
  .object({
    sport: z.string().min(1),
    modality: z.enum(['men', 'women', 'mixed']).optional(),
    allLevels: z.boolean(),
    levelMin: z.number().int().min(1).max(9).optional(),
    levelMax: z.number().int().min(1).max(9).optional(),
    date: z.string().min(1, t.validation.dateRequired),
    start_time: z.string().min(1, t.validation.timeRequired),
    end_time: z.string().min(1, t.validation.timeRequired),
    allCourts: z.boolean(),
    courtIds: z.array(z.string()),
    capacity: z
      .number({ error: t.events.validation.capacityRange })
      .int(t.events.validation.capacityRange)
      .min(1, t.events.validation.capacityRange)
      .max(500, t.events.validation.capacityRange),
    price: z.number({ error: t.events.validation.priceRequired }).positive(t.events.validation.pricePositive),
  })
  // `when` keeps these visible while a price or capacity is still blank (see tournamentForm.schema).
  .refine((data) => !!data.modality, {
    message: t.events.validation.modalityRequired,
    path: ['modality'],
    when: () => true,
  })
  .refine((data) => data.allLevels || data.levelMin !== undefined, {
    message: t.events.validation.levelRangeIncomplete,
    path: ['levelMin'],
    when: () => true,
  })
  .refine((data) => data.allLevels || data.levelMin === undefined || data.levelMax !== undefined, {
    message: t.events.validation.levelRangeIncomplete,
    path: ['levelMax'],
    when: () => true,
  })
  .refine(
    (data) =>
      data.allLevels || data.levelMin === undefined || data.levelMax === undefined || data.levelMin <= data.levelMax,
    { message: t.events.validation.levelRangeOrder, path: ['levelMax'], when: () => true },
  )
  .superRefine((data, ctx) => {
    if (!data.allCourts && data.courtIds.length === 0) {
      ctx.addIssue({ code: 'custom', path: ['courtIds'], message: t.events.validation.courtsRequired });
    }
    if (data.start_time && data.end_time) {
      const minutes = eventDurationMinutes(data.start_time, data.end_time);
      if (minutes < MIN_DURATION_MIN || minutes > MAX_DURATION_MIN) {
        ctx.addIssue({ code: 'custom', path: ['end_time'], message: t.events.validation.duration });
      }
    }
  });

export type EventFormValues = z.infer<typeof eventFormSchema>;
