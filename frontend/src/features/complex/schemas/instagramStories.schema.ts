import { z } from 'zod';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR.instagram.stories;

/** Earliest and latest daily_time the server accepts (inclusive). */
const DAILY_TIME_MIN = '08:00';
const DAILY_TIME_MAX = '22:00';

/** The freed-story cap the server accepts, 1..10. */
export const FREED_CAP_OPTIONS = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10] as const;

const HH_MM = /^([01]\d|2[0-3]):[0-5]\d$/;

const instagramStorySettingsSchema = z.object({
  daily_enabled: z.boolean(),
  daily_time: z.string(),
  freed_enabled: z.boolean(),
  freed_daily_cap: z.number().int(),
});

export type InstagramStorySettings = z.infer<typeof instagramStorySettingsSchema>;

const instagramStorySchema = z.object({
  kind: z.enum(['daily', 'freed']),
  status: z.enum(['pending', 'published', 'skipped', 'failed']),
  scheduled_for: z.string(),
  published_at: z.string().nullable(),
  reason: z.string().nullable(),
});

export type InstagramStory = z.infer<typeof instagramStorySchema>;

/** `GET complexes/{id}/instagram/stories`: the settings and the last 10 stories, newest first. */
export const instagramStoriesResponseSchema = z.object({
  settings: instagramStorySettingsSchema,
  recent: z.array(instagramStorySchema),
});

export type InstagramStoriesResponse = z.infer<typeof instagramStoriesResponseSchema>;

/** `PUT complexes/{id}/instagram/stories` answers the settings it stored. */
export const instagramStorySettingsResponseSchema = z.object({
  settings: instagramStorySettingsSchema,
});

/**
 * The form's own check, the same rule the server answers 422 for. "HH:MM"
 * strings of equal width compare in clock order, so a string compare is enough.
 */
export const instagramStorySettingsFormSchema = instagramStorySettingsSchema.extend({
  daily_time: z
    .string()
    .regex(HH_MM, t.timeOutOfRange)
    .refine((v) => v >= DAILY_TIME_MIN && v <= DAILY_TIME_MAX, t.timeOutOfRange),
  freed_daily_cap: z.number().int().min(1, t.capOutOfRange).max(10, t.capOutOfRange),
});
