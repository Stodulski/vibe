import { z } from 'zod';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR.tournaments.schedule;

/**
 * The schedule form for one tournament: its day must be one of the
 * tournament's (first and last included), the same rule the API enforces with a 422.
 */
export function makeScheduleFormSchema(startDate: string, endDate: string) {
  return z
    .object({
      court_id: z.string().min(1, t.courtRequired),
      date: z.string().min(1, ES_AR.validation.dateRequired),
      start_time: z.string().min(1, ES_AR.validation.timeRequired),
      duration_minutes: z
        .number({ error: t.durationRange })
        .int(t.durationRange)
        .min(30, t.durationRange)
        .max(480, t.durationRange),
    })
    .superRefine((data, ctx) => {
      // `YYYY-MM-DD` strings order the same as the days they name.
      if (data.date && (data.date < startDate || data.date > endDate)) {
        ctx.addIssue({ code: 'custom', path: ['date'], message: t.outsideDays });
      }
    });
}

export type ScheduleFormValues = z.infer<ReturnType<typeof makeScheduleFormSchema>>;
