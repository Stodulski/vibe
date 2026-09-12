import { z } from 'zod';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/**
 * The block-slot dialog's own form shape — not the wire shape
 * (`BlockSlotRequest`, built from these fields in `useBlockSlotForm`).
 *
 * `court_id`/`date`/`start_time` all start out required-but-empty (no court,
 * no date, no time picked yet), so this schema is what turns "nothing chosen"
 * into a field error instead of a submit button that does nothing when
 * clicked with blanks still in it.
 */
export const blockSlotFormSchema = z
  .object({
    date: z.string().min(1, t.validation.dateRequired),
    court_id: z.string().min(1, t.validation.selectCourt),
    start_time: z.string().min(1, t.validation.timeRequired),
    end_time: z.string().min(1, t.validation.timeRequired),
    reason: z.string().max(500, t.validation.maxChars500).optional(),
  })
  .refine((data) => data.start_time < data.end_time, {
    message: t.validation.timeRangeNotEmpty,
    path: ['end_time'],
  });

export type BlockSlotFormValues = z.infer<typeof blockSlotFormSchema>;
