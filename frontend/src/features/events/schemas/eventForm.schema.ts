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
 * takes as an absent `court_ids`.
 */
export const eventFormSchema = z
  .object({
    name: z.string().trim().min(1, t.events.validation.nameRequired).max(100, t.events.validation.nameMax),
    sport: z.string().min(1),
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
